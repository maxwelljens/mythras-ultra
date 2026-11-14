// SPDX-License-Identifier: EUPL-1.2

// Package session orchestrates one Mythras Ultra play session: it owns the
// game state, the model client and the event stream the TUI renders, and
// implements [tui.Driver] — the one place where the LLM, the harness tools,
// the engine and the UI meet (docs/design/session-loop.md is the contract).
//
// Concurrency model: the single runner goroutine started by [New] and
// [Resume] is the only writer of the game state and the conversation
// history; [Session.Metadata] reads the state under the read side of a
// mutex, and events are emitted from that same goroutine, which is what
// guarantees the ordering the TUI relies on (the player's echo first, one
// EventThinking before each model call, one tool annotation before its
// dice, EventTurnEnd last). A resumed session's replayed play history is
// emitted by that goroutine before it processes any input. [Session.Send]
// only enqueues, so it never blocks on the model.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mythras-ultra/myul/internal/engine/resolve"
	"github.com/mythras-ultra/myul/internal/harness/decide"
	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/harness/prompts"
	"github.com/mythras-ultra/myul/internal/harness/tools"
	"github.com/mythras-ultra/myul/internal/state"
	"github.com/mythras-ultra/myul/internal/tui"
)

const (
	// defaultHistoryWindow is how many conversation messages compaction
	// keeps verbatim when Config.HistoryWindow is not set (session-loop.md,
	// "Context compaction").
	defaultHistoryWindow = 32
	// inputQueue bounds how many player messages may wait for the runner
	// before Send blocks (the TUI reads its events continuously, so the
	// queue only fills while a turn runs long).
	inputQueue = 8
	// eventBuffer is the capacity of the channel the TUI reads. It absorbs
	// bursts (a tool round emits annotation + dice together) so the runner
	// rarely waits on the app.
	eventBuffer = 64
)

// input is one queued player message: the channel it was typed on and the
// text exactly as typed (echoed verbatim before the turn runs).
type input struct {
	ch   tui.Channel
	text string
}

// Config wires a session's collaborators. The command layer builds one; the
// package keeps no global state (AGENTS.md golden rules).
type Config struct {
	// Name is the fresh session's name; used by [New] and ignored by
	// [Resume], which takes the persisted name.
	Name string
	// Seed seeds the fresh session's RNG. Zero draws one from the OS
	// entropy source and records it in State.Seed ([resolve.NewState]);
	// ignored by [Resume], whose stream continues from the persisted
	// RNGState.
	Seed int64
	// State, when non-nil, is a pre-built game state for [New] instead
	// of one minted from Name/Seed. The command layer mints it before
	// the model client so the client can carry the state's id as its
	// provider session id ([llm.Config.SessionID]); nil keeps [New]'s
	// own minting. [Resume] ignores it (it loads the persisted state).
	State *resolve.State
	// ModelName labels the language model in metadata and the TUI header,
	// usually "provider/model".
	ModelName string
	// Store persists state.json, log.jsonl and chat.json. Required.
	Store state.Store
	// Client converses with the LLM. Optional: a session created for
	// `myul new --no-tui` without a configured provider has none, and its
	// turns report [ErrNoClient] instead of calling a model.
	Client llm.Client
	// Registry advertises and dispatches the GM tools. Required.
	Registry *tools.Registry
	// Decide is the optional decision model behind the advisory decide
	// tool; nil when disabled (the registry holds the same pointer).
	Decide *decide.Client
	// Overrides selects the system prompt template.
	Overrides prompts.Overrides
	// SessionsDir holds one directory per session (<SessionsDir>/<id>/);
	// [Session.Close] writes transcript.md there. Required.
	SessionsDir string
	// Autosave saves state.json after each accepted move.
	Autosave bool
	// HistoryWindow is how many conversation messages compaction keeps
	// verbatim; zero or negative means defaultHistoryWindow.
	HistoryWindow int
}

// validate rejects a Config that could not produce a working session.
func (cfg Config) validate() error {
	switch {
	case cfg.Store == nil:
		return errors.New("session: config: Store is required")
	case cfg.Registry == nil:
		return errors.New("session: config: Registry is required")
	case cfg.SessionsDir == "":
		return errors.New("session: config: SessionsDir is required (transcript.md is written there on Close)")
	}
	return nil
}

// build assembles the session shell around an already-loaded state; the
// caller starts the runner once the state is persisted.
func (cfg Config) build(st *resolve.State) *Session {
	window := cfg.HistoryWindow
	if window <= 0 {
		window = defaultHistoryWindow
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		state:       st,
		store:       cfg.Store,
		client:      cfg.Client,
		registry:    cfg.Registry,
		decision:    cfg.Decide,
		overrides:   cfg.Overrides,
		sessionsDir: cfg.SessionsDir,
		autosave:    cfg.Autosave,
		window:      window,
		model:       cfg.ModelName,
		events:      make(chan tui.Event, eventBuffer),
		inputs:      make(chan input, inputQueue),
		ctx:         ctx,
		cancel:      cancel,
		quit:        make(chan struct{}),
		done:        make(chan struct{}),
	}
}

// Session is one play session and its implementation of [tui.Driver]. The
// zero value is not usable; create one with [New] or [Resume].
type Session struct {
	// state is the live game state. It is written only by the runner
	// goroutine (through tool dispatch, saves and local moves) and read by
	// Metadata under mu.
	state *resolve.State
	// store persists state.json, log.jsonl and chat.json.
	store state.Store
	// client is the model behind turns; nil means turns report ErrNoClient.
	client llm.Client
	// registry dispatches the GM tools against state.
	registry *tools.Registry
	// decision is the optional decision model, kept for /meta; the
	// registry was built with the same pointer.
	decision *decide.Client
	// overrides selects the system prompt template.
	overrides prompts.Overrides
	// history is the conversation (system prompt excluded): user,
	// assistant and tool messages, oldest first. Written only by the
	// runner goroutine, seeded from chat.json by [Resume] before the
	// runner starts.
	history []llm.Message
	// replay holds what Resume queued before the runner started: a
	// resume notice when chat.json is unusable, then the prior play
	// rebuilt from log.jsonl. The runner delivers it before it processes
	// input; none of it is recorded again (the play came from the log —
	// recording happens only for freshly produced events).
	replay []tui.Event
	// events is the single channel the TUI reads; closed by Close.
	events chan tui.Event
	// inputs queues player messages for the runner; Send writes, the
	// runner reads.
	inputs chan input
	// sessionsDir + state.ID locate transcript.md on disk.
	sessionsDir string
	// autosave saves state.json after every accepted move.
	autosave bool
	// window is the compaction window in messages.
	window int
	// model labels the language model in metadata (/meta).
	model string

	// mu guards state reads from the TUI against dispatches from the
	// runner; it is never held across a model call or an event send.
	mu sync.RWMutex
	// ctx is cancelled by Close so an in-flight model call aborts.
	ctx    context.Context
	cancel context.CancelFunc
	// quit unblocks emits and input waits during shutdown; done closes
	// when the runner has exited.
	quit, done chan struct{}
	// closed records the first Close so later calls are no-ops.
	closed atomic.Bool
}

// New starts a fresh session: it adopts [Config.State] when the command
// layer pre-built one, otherwise builds state via [resolve.NewState],
// persists it (state.json plus the opening log.jsonl entry), and starts the
// runner goroutine. The caller owns the returned session and must Close it.
func New(cfg Config) (*Session, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	st := cfg.State
	if st == nil {
		st = resolve.NewState(cfg.Name, cfg.Seed)
	}
	s := cfg.build(st)
	if err := s.store.Save(st); err != nil {
		s.cancel()
		return nil, fmt.Errorf("session: save initial state: %w", err)
	}
	if err := s.recordStart(); err != nil {
		s.cancel()
		return nil, err
	}
	s.start()
	return s, nil
}

// Resume continues a saved session: Store.Load runs Normalize (re-binding
// encounter pointer identity), and the RNG stream picks up exactly where
// the last process stopped — Resume never reseeds. The conversation
// history resumes from chat.json (written at every autosaved turn end, on
// /save and on Close); when that file is missing or corrupt the turn loop
// starts with an empty history and one system event says so — continuity
// then comes from the system prompt's recap and summaries, rebuilt from
// state on every turn. The play recorded in log.jsonl is queued for
// replay before the runner starts, so the TUI shows the history
// immediately (chat.go, session-loop.md "Chat persistence & replay").
func Resume(cfg Config, id string) (*Session, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	st, err := cfg.Store.Load(id)
	if err != nil {
		return nil, fmt.Errorf("session: resume %q: %w", id, err)
	}
	// Load already normalizes; running it again is idempotent and keeps
	// this constructor honest if the Store implementation ever changes.
	if err := st.Normalize(); err != nil {
		return nil, fmt.Errorf("session: resume %q: normalize: %w", id, err)
	}
	s := cfg.build(st)
	s.loadChat()
	s.loadReplay()
	s.start()
	return s, nil
}

// ID returns the session identifier state.json is keyed by.
func (s *Session) ID() string { return s.state.ID }

// start launches the runner goroutine.
func (s *Session) start() { go s.run() }

// recordStart writes the opening line of log.jsonl so a brand-new session
// already has an audit trail (and `myul new --no-tui` leaves both
// state.json and log.jsonl behind).
func (s *Session) recordStart() error {
	st := s.state
	move, err := json.Marshal(map[string]int64{"seed": st.Seed})
	if err != nil {
		return fmt.Errorf("session: encode start entry: %w", err)
	}
	res, err := json.Marshal(resolve.Result{
		Summary: fmt.Sprintf("session %q started (seed %d)", st.Name, st.Seed),
	})
	if err != nil {
		return fmt.Errorf("session: encode start result: %w", err)
	}
	if err := s.store.Record(st.ID, state.LogEntry{Kind: "session", Move: move, Result: res}); err != nil {
		return fmt.Errorf("session: record start: %w", err)
	}
	return nil
}

// emit delivers one event to the TUI. It blocks until the app reads the
// event (or the buffer drains), except during shutdown, when quit unblocks
// it so Close can never deadlock behind a full buffer. Emits happen on the
// runner goroutine only — that is the ordering guarantee.
func (s *Session) emit(ev tui.Event) {
	select {
	case <-s.quit:
		return // shutting down: the app has left, drop the event
	default:
	}
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	select {
	case s.events <- ev:
	case <-s.quit:
	}
}

// save persists state.json. Safe without holding mu: the runner is the only
// state writer (Save stamps UpdatedAt/CreatedAt itself) and Metadata never
// reads the timestamps.
func (s *Session) save() error {
	if err := s.store.Save(s.state); err != nil {
		return fmt.Errorf("session: save: %w", err)
	}
	return nil
}

// record appends one dispatch to log.jsonl: the model's call verbatim (id,
// name, arguments) plus the result, or the failure for a refused call.
func (s *Session) record(seq uint64, call llm.ToolCall, res tools.Result, fault error) error {
	entry := state.LogEntry{Seq: seq, Kind: call.Name}
	var err error
	if entry.Move, err = json.Marshal(call); err != nil {
		return fmt.Errorf("session: encode tool call %q: %w", call.Name, err)
	}
	switch {
	case fault != nil:
		entry.Error = fault.Error()
	default:
		if entry.Result, err = json.Marshal(res); err != nil {
			return fmt.Errorf("session: encode result of %q: %w", call.Name, err)
		}
		if !res.OK {
			entry.Error = res.Summary
		}
	}
	if err := s.store.Record(s.state.ID, entry); err != nil {
		return fmt.Errorf("session: record %q: %w", call.Name, err)
	}
	return nil
}

// writeTranscript renders transcript.md from state + log.jsonl into the
// session directory. It runs from Close, after the runner has exited.
func (s *Session) writeTranscript() error {
	id := s.state.ID
	entries, err := s.store.ReadLog(id)
	if err != nil {
		return fmt.Errorf("session: read log for transcript: %w", err)
	}
	path := filepath.Join(s.sessionsDir, id, "transcript.md")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("session: create %s: %w", path, err)
	}
	werr := state.WriteTranscript(f, s.state, entries)
	if cerr := f.Close(); cerr != nil {
		werr = errors.Join(werr, fmt.Errorf("session: close %s: %w", path, cerr))
	}
	if werr != nil {
		return fmt.Errorf("session: write transcript: %w", werr)
	}
	return nil
}

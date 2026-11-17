// SPDX-License-Identifier: EUPL-1.2

package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/tui"
)

// ErrClosed is returned by [Session.Send] after [Session.Close]: the runner
// has exited and no further input can be processed.
var ErrClosed = errors.New("session: closed")

// ErrNoClient is reported by a turn when the session was built without a
// model client (Config.Client nil — typically `myul new --no-tui` on a host
// with no [llm] provider configured). The local commands still work: they
// never call the model.
var ErrNoClient = errors.New("session: no language model configured (set [llm] provider and model)")

// Session implements [tui.Driver]; see the interface for the contract.
var _ tui.Driver = (*Session)(nil)

// Send queues the player's text on ch and returns without waiting for the
// model (tui.Driver). The runner echoes the text verbatim as an
// EventNarrative on the same channel — the TUI matches echoes by exact
// text, so it is never reformatted — and only then runs the turn. The text
// is never parsed for commands: prose that looks like "/save" reaches the
// model as prose. The error is non-nil when the session is closed, the
// text is empty, or ctx ends while the input queue is full.
func (s *Session) Send(ctx context.Context, ch tui.Channel, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("session: refusing an empty message")
	}
	if s.closed.Load() {
		return ErrClosed
	}
	select {
	case <-s.quit:
		return ErrClosed
	default:
	}
	// Both mode changes converge on SetChannel: the channel this message
	// goes out on is the mode to remember for the next resume. It is a UI
	// hint, so a failed meta.json write must never fail the send.
	_ = s.SetChannel(ch)
	select {
	case s.inputs <- input{ch: ch, text: text}:
		return nil
	case <-s.quit:
		return ErrClosed
	case <-ctx.Done():
		return fmt.Errorf("session: send: %w", ctx.Err())
	}
}

// Command queues one named local command for the runner and returns without
// waiting (tui.Driver). Commands carry no player text: they produce no echo,
// they never reach the model, and they run on the same single-runner
// goroutine as turns, so their events keep their place in the stream. The
// command runs on the session's current channel (the last-used IC/OOC
// mode). The error is non-nil when the session is closed or the name is
// empty; an unknown name is reported back through Events as an error line
// (session-loop.md "Commands"). A pending confirmation's "confirm" answer
// is the one exception to the queue: while a hold or a golden-rule
// regeneration waits, it is handed straight to the waiting runner, and a
// confirm with no confirmation behind it comes back as a "nothing pending"
// error line.
func (s *Session) Command(name string, args ...string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("session: refusing an unnamed command")
	}
	if s.closed.Load() {
		return ErrClosed
	}
	select {
	case <-s.quit:
		return ErrClosed
	default:
	}
	if name == "confirm" && s.held.Load() {
		// The runner is blocked in a confirmation wait (waitConfirm or
		// waitRegen): enqueueing would make it report "nothing pending"
		// instead of answering. A malformed answer is refused rather
		// than read as "n", which would silently settle the pending
		// confirmation the other way.
		if len(args) != 1 || (args[0] != "y" && args[0] != "n") {
			return errors.New(`session: confirm expects "y" or "n"`)
		}
		select {
		case s.confirmCh <- args[0]:
			return nil
		default:
			return errors.New("session: a confirmation answer is already pending")
		}
	}
	select {
	case s.inputs <- input{ch: s.LastChannel(), cmd: name, args: args}:
		return nil
	case <-s.quit:
		return ErrClosed
	}
}

// SetChannel remembers ch as the session's current IC/OOC mode and persists
// it to meta.json immediately (tui.Driver), so a later [Resume] reopens on
// the same channel — meta.json is tiny and written atomically, so writing
// it beside the runner's own saves is safe. It is a UI hint, never game
// state: after [Close] it is a no-op returning nil (the final saves have
// run; nothing more is written). Safe to call from the TUI goroutine
// concurrently with the runner.
func (s *Session) SetChannel(ch tui.Channel) error {
	if s.closed.Load() {
		return nil
	}
	s.chanMu.Lock()
	s.lastChannel = ch
	s.chanMu.Unlock()
	data, err := json.Marshal(sessionMeta{Channel: ch.String()})
	if err != nil {
		return fmt.Errorf("session: encode channel meta: %w", err)
	}
	if err := s.store.SaveMeta(s.state.ID, data); err != nil {
		return fmt.Errorf("session: save channel meta: %w", err)
	}
	return nil
}

// Events returns the single event channel; every call returns the same one,
// which Close closes (tui.Driver).
func (s *Session) Events() <-chan tui.Event { return s.events }

// Metadata returns a fresh display snapshot of the state behind the panel:
// a cheap read-through under the read lock, never a model call
// (tui.Driver). See buildMetadata for how state shapes map to the panel;
// the context readout joins it from the last model call's usage.
func (s *Session) Metadata() tui.Metadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := buildMetadata(s.state)
	m.ContextUsage = tui.ContextUsage{Tokens: s.contextTokens, Window: s.contextWindow}
	return m
}

// Close releases the session: it stops the runner (cancelling any in-flight
// model call), performs the final save of state and conversation
// (state.json + chat.json), writes transcript.md next to state.json, and
// closes the Events channel. It is idempotent — every call after the first
// returns nil — and safe to call while a turn is running: the turn's
// partial state is whatever moves were accepted, each of which
// is atomic (session-loop.md failure policy).
func (s *Session) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(s.quit) // unblocks emits and pending Sends
	s.cancel()    // aborts the in-flight model call or dispatch
	<-s.done      // the runner has exited: no more writers or emitters
	var errs []error
	if err := s.save(); err != nil {
		errs = append(errs, err)
	}
	if err := s.saveChat(); err != nil {
		errs = append(errs, err)
	}
	if err := s.writeTranscript(); err != nil {
		errs = append(errs, err)
	}
	close(s.events)
	return errors.Join(errs...)
}

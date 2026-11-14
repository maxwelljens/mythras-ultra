// SPDX-License-Identifier: EUPL-1.2

package session

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/mythras-ultra/myul/internal/engine/resolve"
	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/harness/prompts"
	"github.com/mythras-ultra/myul/internal/harness/tools"
	"github.com/mythras-ultra/myul/internal/tui"
)

const (
	// maxToolRounds bounds one turn's tool-call loop: after this many
	// dispatch rounds the next model reply is reported as an EventError
	// instead of being followed (session-loop.md: "Bound the loop ... stop
	// with an EventError").
	maxToolRounds = 32
	// maxNarrativeBatch bounds how much streamed text accumulates before it
	// is flushed as one EventNarrative when the model emits no newline.
	maxNarrativeBatch = 4096
	// oocTag prefixes player text sent on the OOC channel so the model sees
	// the channel even though providers carry no such concept; the TUI
	// tags its own transcript line from Event.Channel instead. The system
	// prompt asks the GM to answer out of character and perform no moves
	// unless the player confirms (session-loop.md step 4), so an OOC turn
	// expects no tool calls — but if the model makes them anyway the loop
	// dispatches them normally: state never mutates outside the registry.
	oocTag = "[ooc] "
)

// channelTagRe matches a channel marker a model sometimes writes at the
// very start of its reply ("[IC] The door…", "[ooc]: sure"). It is anchored
// to the start of the string (after leading whitespace) so a marker inside
// the prose, or a markdown link such as [ICest](…), is never touched.
var channelTagRe = regexp.MustCompile(`(?i)^\s*\[(ic|ooc)\]:?`)

// stripChannelTag drops a leading channel marker from model output: the TUI
// derives the channel from Event.Channel, so an inline marker would only
// duplicate the interface's own tagging. It is applied to model output
// only — never to the player's echo.
func stripChannelTag(text string) string {
	loc := channelTagRe.FindStringIndex(text)
	if loc == nil {
		return text
	}
	return strings.TrimLeft(text[loc[1]:], " \t")
}

// emitNarrative sends one piece of model output to the transcript with the
// leading channel marker stripped, and records it as a GM chat line (the
// sanitised text — the log holds what the player saw). A message that is
// only the marker emits nothing — the marker is dropped, whatever follows
// it still flows as the next narrative event.
func (s *Session) emitNarrative(ch tui.Channel, text string) {
	body := stripChannelTag(text)
	if strings.TrimSpace(body) == "" {
		return
	}
	s.emitChat(tui.Event{Kind: tui.EventNarrative, Channel: ch, Markdown: body}, false)
}

// run is the session's single runner goroutine: it first delivers whatever
// Resume queued (the replayed play history, before any input — the TUI
// shows the prior session immediately), then takes player input in arrival
// order and processes it one turn at a time, which is what puts the
// player's echo ahead of every event that turn produces. It exits when
// Close closes quit; the done channel reports that to Close.
func (s *Session) run() {
	defer close(s.done)
	for _, ev := range s.replay {
		s.emit(ev)
	}
	s.replay = nil
	for {
		select {
		case <-s.quit:
			return
		case in := <-s.inputs:
			s.process(in)
		}
	}
}

// process echoes the player's text verbatim (recording it as a player chat
// line), then routes it: slash commands run locally instead of through the
// model, everything else is a model turn on the input's channel
// (session-loop.md "Turn loop" step 1a–1b).
func (s *Session) process(in input) {
	s.emitChat(tui.Event{Kind: tui.EventNarrative, Channel: in.ch, Markdown: in.text, Player: true}, true)

	cmd, err := tui.ParseInput(in.text)
	if err != nil {
		// The TUI parses first and shows its own error; this path
		// covers direct Driver users and unknown commands.
		s.emitChat(tui.Event{Kind: tui.EventError, Channel: in.ch, Markdown: err.Error()}, false)
		return
	}
	switch cmd.Kind {
	case tui.CmdMessage:
		s.runTurn(in.ch, in.text)
	case tui.CmdHelp:
		// /help is answered by the TUI overlay; when the driver is used
		// outside the TUI it passes through to the model like chat.
		s.runTurn(in.ch, in.text)
	case tui.CmdRoll:
		s.cmdRoll(in.ch, rollExpr(cmd.Text))
	case tui.CmdSave:
		s.cmdSave(in.ch)
	case tui.CmdRecap:
		s.cmdRecap(in.ch)
	case tui.CmdMeta:
		s.cmdMeta(in.ch)
	default:
		s.emitChat(tui.Event{Kind: tui.EventError, Channel: in.ch,
			Markdown: "that command is handled by the TUI and never reaches the harness"}, false)
	}
}

// rollExpr returns a /roll line's argument exactly as typed — everything
// after the command name — because dice expressions may contain spaces
// that tui.ParseInput's argument splitting would break apart.
func rollExpr(text string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "/"))
	if i := strings.IndexFunc(rest, unicode.IsSpace); i >= 0 {
		return strings.TrimSpace(rest[i:])
	}
	return ""
}

// cmdRoll runs the player's own roll through the roll_dice tool, so it
// lands in the audit trail and the dice theatre exactly like a GM roll
// (session-loop.md: slash commands are handled locally).
func (s *Session) cmdRoll(ch tui.Channel, expr string) {
	args, err := json.Marshal(map[string]string{"expr": expr, "reason": "player /roll"})
	if err != nil {
		s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch, Markdown: fmt.Sprintf("encode /roll: %v", err)}, false)
		return
	}
	// A bad expression comes back as a refused result, which the
	// annotation strip shows with the ✗ glyph — no dice event, no abort.
	s.runTool(ch, llm.ToolCall{Name: "roll_dice", Args: args})
}

// cmdSave flushes state.json and the conversation now (autosave or not)
// and reports it.
func (s *Session) cmdSave(ch tui.Channel) {
	if err := s.save(); err != nil {
		s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch, Markdown: err.Error()}, false)
		return
	}
	if err := s.saveChat(); err != nil {
		s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch, Markdown: err.Error()}, false)
		return
	}
	s.emit(tui.Event{Kind: tui.EventSystem, Channel: ch,
		Markdown: fmt.Sprintf("session saved (%s)", s.state.ID)})
}

// cmdRecap replays the stored end-of-session recaps without calling the
// model.
func (s *Session) cmdRecap(ch tui.Channel) {
	recaps := resolve.SessionRecap(s.state)
	if len(recaps) == 0 {
		s.emit(tui.Event{Kind: tui.EventSystem, Channel: ch,
			Markdown: "no recaps recorded yet — the GM stores one with `end_session_recap` at a closing."})
		return
	}
	s.emit(tui.Event{Kind: tui.EventSystem, Channel: ch, Markdown: strings.Join(recaps, "\n\n")})
}

// cmdMeta dumps the panel snapshot as one compact system block.
func (s *Session) cmdMeta(ch tui.Channel) {
	s.emit(tui.Event{Kind: tui.EventSystem, Channel: ch, Markdown: s.metaDump()})
}

// reply is one model answer: the full narration text plus any tool calls.
type reply struct {
	text  string
	calls []llm.ToolCall
}

// runTurn is one model turn (session-loop.md "Turn loop"): append the
// player's text, assemble the system prompt, call the model, dispatch every
// tool call it makes (annotation → dice → RoleTool results) and call again,
// until it answers in prose — at most maxToolRounds dispatch rounds. A
// model failure rolls the whole turn out of history so the player can
// retry; a tool failure flows back as an IMPOSSIBLE result and never
// aborts the turn. The turn is closed by endTurn, so exactly one
// EventTurnEnd lands after the last event of the turn on every path —
// success, tool-loop overflow or rollback.
func (s *Session) runTurn(ch tui.Channel, text string) {
	defer s.endTurn(ch)
	rollback := len(s.history)
	content := text
	if ch == tui.ChannelOOC {
		content = oocTag + text
	}
	s.history = append(s.history, llm.Message{Role: llm.RoleUser, Content: content})

	prompt, err := s.systemPrompt()
	if err != nil {
		s.failTurn(ch, rollback, fmt.Errorf("assemble the system prompt: %w", err))
		return
	}
	rounds := 0
	for {
		res, err := s.callModel(ch, llm.ChatRequest{
			Messages: s.requestMessages(prompt),
			Tools:    s.registry.Tools(),
		})
		if err != nil {
			s.failTurn(ch, rollback, err)
			return
		}
		s.history = append(s.history, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   res.text,
			ToolCalls: res.calls,
		})
		if len(res.calls) == 0 {
			break // prose answer: the turn is complete
		}
		if rounds >= maxToolRounds {
			// Providers reject a reply whose tool calls have no
			// results, and this turn will not produce them: keep the
			// narration, drop the unanswered calls, stop the loop.
			s.dropDanglingCalls()
			s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch, Markdown: fmt.Sprintf(
				"the model kept calling tools for %d rounds in one turn — stopping here; nudge it to answer in prose",
				maxToolRounds)}, false)
			break
		}
		rounds++
		for _, call := range res.calls {
			toolRes := s.runTool(ch, call)
			payload, merr := json.Marshal(toolRes)
			body := toolRes.Summary
			if merr != nil {
				s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch,
					Markdown: fmt.Sprintf("encode result of %q: %v", call.Name, merr)}, false)
			} else {
				body = string(payload)
			}
			s.history = append(s.history, llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: call.ID,
				Name:       call.Name,
				Content:    body,
			})
		}
	}
	s.compact()
}

// endTurn closes one model turn: it persists the conversation to chat.json
// when autosave is on — the turn just changed it, and narrative-only turns
// (no moves, nothing for state autosave to catch) would otherwise never
// reach disk — then emits the EventTurnEnd the TUI brackets its header on.
// Deferred to runTurn, it runs on every path: a failed turn saves the
// rolled-back history, so chat.json always holds the last good state.
func (s *Session) endTurn(ch tui.Channel) {
	if s.autosave {
		if err := s.saveChat(); err != nil {
			s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch, Markdown: err.Error()}, false)
		}
	}
	s.emit(tui.Event{Kind: tui.EventTurnEnd, Channel: ch})
}

// dropDanglingCalls removes unanswered tool calls from the last assistant
// message: an assistant reply that asks for tools with no RoleTool answers
// following it would be rejected by the provider on the next call.
func (s *Session) dropDanglingCalls() {
	last := len(s.history) - 1
	if last < 0 {
		return
	}
	if s.history[last].Content == "" {
		s.history = s.history[:last] // nothing left worth keeping
		return
	}
	s.history[last].ToolCalls = nil
}

// requestMessages builds one ChatRequest: the freshly assembled system
// prompt first, then the conversation history.
func (s *Session) requestMessages(prompt string) []llm.Message {
	msgs := make([]llm.Message, 0, len(s.history)+1)
	msgs = append(msgs, llm.Message{Role: llm.RoleSystem, Content: prompt})
	return append(msgs, s.history...)
}

// systemPrompt renders prompts.Assemble's Vars from the live state: the
// stored recaps, a summary of the player character, the world summary and
// the in-game date (session-loop.md step 1c).
func (s *Session) systemPrompt() (string, error) {
	st := s.state
	return prompts.Assemble(s.overrides, prompts.Vars{
		SessionName:      st.Name,
		Recap:            recapText(st),
		CharacterSummary: characterSummary(st),
		WorldSummary:     worldSummary(st),
		Date:             inGameDate(st.World),
	})
}

// callModel asks the model for the next message, streaming narration into
// the transcript as it arrives: each paragraph-terminated batch (or, for a
// long single-paragraph passage, each maxNarrativeBatch-sized chunk) becomes
// one EventNarrative, and the last fragment is flushed when the reply ends. A
// stream that fails before producing anything falls back to a single Chat
// call — a client may be chat-only — while a stream that fails midway is a
// model failure the caller rolls back (session-loop.md failure policy).
func (s *Session) callModel(ch tui.Channel, req llm.ChatRequest) (reply, error) {
	if s.client == nil {
		return reply{}, ErrNoClient
	}
	// A model call is in flight — the header throbber shows "thinking"
	// until the first narrative delta or the turn end. Emitted here rather
	// than in runTurn so it covers every call, re-invocations included,
	// and never fires for a session without a client.
	s.emit(tui.Event{Kind: tui.EventThinking, Channel: ch})
	var (
		r       reply
		got     bool
		full    strings.Builder
		pending string
	)
	flush := func() {
		if pending != "" {
			s.emitNarrative(ch, pending)
			pending = ""
		}
	}
	err := s.client.Stream(s.ctx, req, func(ev llm.StreamEvent) error {
		got = true
		switch ev.Kind {
		case llm.StreamTextDelta:
			full.WriteString(ev.Text)
			pending += ev.Text
			if i := strings.Index(pending, "\n\n"); i >= 0 {
				// Flush at paragraph boundaries, not every newline: an
				// entry is a complete Markdown paragraph, so the
				// transcript's "\n\n" join lands exactly one blank line
				// between paragraphs (line-level flushes rendered as
				// extra blank lines). Whitespace-only batches (e.g. a
				// stream opening with "\n\n") are dropped — the join
				// already supplies the separation.
				if batch := pending[:i+2]; strings.TrimSpace(batch) != "" {
					s.emitNarrative(ch, batch)
				}
				pending = pending[i+2:]
			} else if len(pending) >= maxNarrativeBatch {
				flush()
			}
		case llm.StreamToolCall:
			r.calls = append(r.calls, ev.ToolCall)
		case llm.StreamDone:
			// Usage is deliberately unused: compaction windows by
			// message count (HistoryWindow), not by tokens.
		}
		return nil
	})
	switch {
	case err == nil:
		flush()
		r.text = full.String()
		return r, nil
	case !got:
		// The stream never started — fall back to one Chat round trip.
		resp, cerr := s.client.Chat(s.ctx, req)
		flush()
		if cerr != nil {
			return reply{}, fmt.Errorf("chat: %w", cerr)
		}
		s.emitNarrative(ch, resp.Message.Content)
		return reply{text: resp.Message.Content, calls: resp.Message.ToolCalls}, nil
	default:
		flush()
		return reply{}, fmt.Errorf("stream: %w", err)
	}
}

// runTool dispatches one tool call against the live state and reports it:
// the result is recorded in log.jsonl (with the call's id), announced as an
// EventToolAnnotation, and expanded into one EventDice per audit roll so
// the dice theatre can play it. Autosave follows each accepted move. A
// harness fault (a Go error from Dispatch) is surfaced to the player and
// still flows back as an IMPOSSIBLE result, so the turn continues
// (session-loop.md failure policy).
func (s *Session) runTool(ch tui.Channel, call llm.ToolCall) tools.Result {
	s.mu.Lock()
	res, err := s.registry.Dispatch(s.ctx, s.state, call.Name, call.Args)
	seq := s.state.EventSeq
	s.mu.Unlock()
	if err != nil {
		res = tools.Result{OK: false, Summary: "IMPOSSIBLE: harness fault: " + err.Error()}
		s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch,
			Markdown: fmt.Sprintf("tool %s: harness fault: %v", call.Name, err)}, false)
	}
	if rerr := s.record(seq, call, res, err); rerr != nil {
		s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch, Markdown: rerr.Error()}, false)
	}
	s.emit(tui.Event{
		Kind:    tui.EventToolAnnotation,
		Channel: ch,
		Annotation: &tui.Annotation{
			ToolName: call.Name,
			Summary:  res.Summary,
			OK:       res.OK,
		},
	})
	for _, de := range diceEvents(res) {
		ev := de
		s.emit(tui.Event{Kind: tui.EventDice, Channel: ch, Dice: &ev})
	}
	if res.OK && s.autosave {
		if serr := s.save(); serr != nil {
			s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch, Markdown: serr.Error()}, false)
		}
	}
	return res
}

// failTurn rolls the partial turn out of conversation history — the player
// can retry, and no partial tool state exists because moves are atomic —
// and reports the failure on the transcript (session-loop.md failure
// policy).
func (s *Session) failTurn(ch tui.Channel, rollback int, err error) {
	if rollback <= len(s.history) {
		s.history = s.history[:rollback]
	}
	s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch,
		Markdown: fmt.Sprintf("turn failed: %v (the turn was rolled back — try again)", err)}, false)
}

// diceEvents projects a tool result's audit rolls onto the theatre: dice
// values, modifier and total come from the roll itself, the outcome grade
// from the result's "outcome: <grade>" fact when it reports one (the
// engine's own vocabulary — critical, success, failure, fumble).
func diceEvents(res tools.Result) []tui.DiceEvent {
	if res.Audit == nil {
		return nil
	}
	outcomes := gradeOutcomes(res.Facts)
	events := make([]tui.DiceEvent, 0, len(res.Audit.Rolls))
	for i, roll := range res.Audit.Rolls {
		de := tui.DiceEvent{
			Expr:     roll.Expr,
			Dice:     make([]int, len(roll.Dice)),
			Modifier: roll.Modifier,
			Total:    roll.Total,
		}
		for j, die := range roll.Dice {
			de.Dice[j] = die.Value
		}
		if i < len(outcomes) {
			de.Outcome = outcomes[i]
		}
		events = append(events, de)
	}
	return events
}

// gradeOutcomes extracts the qualitative grades a result reports, in roll
// order. Only grades the engine names as outcomes are accepted; results
// without them (a free-form roll) yield no entries and the banner renders
// no outcome.
func gradeOutcomes(facts []string) []string {
	var outcomes []string
	for _, fact := range facts {
		value, ok := strings.CutPrefix(fact, "outcome: ")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "critical", "success", "failure", "fumble":
			outcomes = append(outcomes, strings.ToLower(strings.TrimSpace(value)))
		}
	}
	return outcomes
}

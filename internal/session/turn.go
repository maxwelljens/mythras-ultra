// SPDX-License-Identifier: EUPL-1.2

package session

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mythras-ultra/myul/internal/engine/resolve"
	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/harness/prompts"
	"github.com/mythras-ultra/myul/internal/harness/tools"
	"github.com/mythras-ultra/myul/internal/tui"
)

const (
	// partialNarrativeMin is how many bytes of one paragraph may wait on
	// display before the stream shows them: paragraph boundaries still
	// close the entry, but an unbroken run now appears in batches instead
	// of sitting invisible until the next "\n\n".
	partialNarrativeMin = 64
	// oocTag prefixes player text sent on the OOC channel so the model sees
	// the channel even though providers carry no such concept; the TUI
	// tags its own transcript line from Event.Channel instead. The system
	// prompt asks the GM to answer out of character and perform no moves
	// unless the player confirms (session-loop.md step 4), so an OOC turn
	// expects no tool calls — but if the model makes them anyway the loop
	// dispatches them normally: state never mutates outside the registry.
	oocTag = "[ooc] "
)

// sessionCommands lists the commands [Session.Command] understands, in
// help-output order; Command reports the same list on an unknown name
// (the TUI's palette keeps its own catalogue — tui.PaletteCommands).
var sessionCommands = []string{"roll", "save", "recap", "meta"}

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
// it still flows as the next narrative event. It is the single-batch path
// (the Chat fallback); streamed paragraphs go through the display/record
// split below.
func (s *Session) emitNarrative(ch tui.Channel, text string) {
	body := stripChannelTag(text)
	if strings.TrimSpace(body) == "" {
		return
	}
	s.emitChat(tui.Event{Kind: tui.EventNarrative, Channel: ch, Markdown: body}, false)
}

// emitNarrativeBatch sends one display batch of model output to the
// transcript WITHOUT recording it: batches only feed the screen, and the
// chat line is written once when the entry closes. Sanitation matches the
// recording path — the leading channel marker is stripped and a
// whitespace-only batch emits nothing.
func (s *Session) emitNarrativeBatch(ch tui.Channel, text string, continuation bool) {
	body := stripChannelTag(text)
	if strings.TrimSpace(body) == "" {
		return
	}
	s.emit(tui.Event{Kind: tui.EventNarrative, Channel: ch, Markdown: body,
		Continuation: continuation})
}

// recordNarrative records one closed entry as a GM chat line — the
// sanitised text, what the player saw across every batch of that entry.
// The whitespace-only guard mirrors the display path, so a bare paragraph
// break records nothing.
func (s *Session) recordNarrative(ch tui.Channel, text string) {
	body := stripChannelTag(text)
	if strings.TrimSpace(body) == "" {
		return
	}
	s.recordChat(tui.Event{Kind: tui.EventNarrative, Channel: ch, Markdown: body}, false)
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

// process routes one queued input: a local command runs immediately instead
// of through the model, everything else is a player message — echoed
// verbatim (recording it as a player chat line) and then run as a model
// turn on the input's channel (session-loop.md "Turn loop" step 1a–1b).
// Player text is never parsed for commands: prose that looks like "/save"
// is prose, and reaches the model as typed.
func (s *Session) process(in input) {
	if in.cmd != "" {
		s.runCommand(in.ch, in.cmd, in.args)
		return
	}
	s.emitChat(tui.Event{Kind: tui.EventNarrative, Channel: in.ch, Markdown: in.text, Player: true}, true)
	s.runTurn(in.ch, in.text)
}

// runCommand executes one command enqueued by [Session.Command] on the
// runner goroutine (session-loop.md "Commands"): the same local effects
// the old slash commands had — roll through the roll_dice tool, save the
// session, replay the recaps, dump the panel snapshot — each reported with
// its own system or error event. An unknown command is an error line
// listing the known ones; no model call ever happens here.
func (s *Session) runCommand(ch tui.Channel, name string, args []string) {
	switch name {
	case "roll":
		expr := strings.TrimSpace(strings.Join(args, " "))
		if expr == "" {
			s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch, Markdown: "roll needs an expression, e.g. roll 2d6+3 — commands: " +
				strings.Join(sessionCommands, " ")}, false)
			return
		}
		s.cmdRoll(ch, expr)
	case "save":
		s.cmdSave(ch)
	case "recap":
		s.cmdRecap(ch)
	case "meta":
		s.cmdMeta(ch)
	default:
		s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch, Markdown: fmt.Sprintf(
			"unknown command %q — commands: %s", name,
			strings.Join(sessionCommands, " "))}, false)
	}
}

// cmdRoll runs the player's own roll through the roll_dice tool, so it
// lands in the audit trail and the dice theatre exactly like a GM roll
// (session-loop.md: commands are handled locally).
func (s *Session) cmdRoll(ch tui.Channel, expr string) {
	args, err := json.Marshal(map[string]string{"expr": expr, "reason": "player roll"})
	if err != nil {
		s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch, Markdown: fmt.Sprintf("encode roll: %v", err)}, false)
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
// until it answers in prose — at most s.maxRounds dispatch rounds
// (Config.MaxToolRounds; defaultMaxToolRounds when unset). A model failure
// rolls the whole turn out of history so the player can retry; a tool
// failure flows back as an IMPOSSIBLE result and never aborts the turn.
// The turn is closed by endTurn, so exactly one EventTurnEnd lands after
// the last event of the turn on every path — success, tool-loop overflow
// or rollback.
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
		if rounds >= s.maxRounds {
			// Providers reject a reply whose tool calls have no
			// results, and this turn will not produce them: keep the
			// narration, drop the unanswered calls, stop the loop.
			s.dropDanglingCalls()
			s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch, Markdown: fmt.Sprintf(
				"the model kept calling tools for %d rounds in one turn — stopping here; nudge it to answer in prose",
				s.maxRounds)}, false)
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

// streamEntry is the paragraph one model reply currently has open.
//
// State, owned by the single runner goroutine (no locking): text is every
// delta of the reply so far, start is where the open entry begins in text
// and sent is how far the display has advanced past it. Closing the entry
// at a "\n\n" boundary flushes the tail [sent:end], records the whole
// entry [start:end] exactly once and jumps start and sent past the
// boundary, so the next paragraph opens fresh; the end of the reply closes
// it the same way at len(text). Display batches (rest) never record —
// only a close does.
type streamEntry struct {
	text  string
	start int
	sent  int
}

// rest returns the entry's undisplayed tail and whether showing it
// extends an entry already on screen, then marks the tail shown.
func (e *streamEntry) rest() (tail string, continuation bool) {
	tail, continuation = e.text[e.sent:], e.sent > e.start
	e.sent = len(e.text)
	return tail, continuation
}

// closeAt closes the entry at end (just past a "\n\n", or len(text) at the
// end of the reply): the still-undisplayed tail first (see rest), then the
// whole entry text for the caller to record — empty when the entry is
// whitespace-only, the blank-entry guard that keeps a paragraph break from
// becoming a transcript entry.
func (e *streamEntry) closeAt(end int) (tail string, continuation bool, record string) {
	tail, continuation = e.text[e.sent:end], e.sent > e.start
	record = e.text[e.start:end]
	e.start, e.sent = end, end
	return tail, continuation, record
}

// callModel asks the model for the next message, streaming narration into
// the transcript as it arrives. Display and recording are separate paths:
// every batch — the tail at a paragraph boundary, an eager flush once an
// unbroken run reaches partialNarrativeMin bytes, and the remainder when
// the reply ends — is emitted for display (Continuation when it extends
// the entry on screen), while the paragraph itself is recorded as ONE chat
// line when its entry closes. A stream that fails before producing
// anything falls back to a single Chat call — a client may be chat-only —
// while a stream that fails midway is a model failure the caller rolls
// back (session-loop.md failure policy).
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
		r   reply
		got bool
		ent streamEntry
	)
	err := s.client.Stream(s.ctx, req, func(ev llm.StreamEvent) error {
		got = true
		switch ev.Kind {
		case llm.StreamTextDelta:
			ent.text += ev.Text
			// Every "\n\n" in the open entry closes it: flush the
			// tail, record the paragraph once, open the next one.
			for {
				rel := strings.Index(ent.text[ent.start:], "\n\n")
				if rel < 0 {
					break
				}
				s.closeEntry(ch, &ent, ent.start+rel+2)
			}
			// No boundary: show what has piled up so mid-paragraph
			// text is not invisible — display only, the entry stays
			// open until its paragraph ends.
			if len(ent.text)-ent.sent >= partialNarrativeMin {
				tail, cont := ent.rest()
				s.emitNarrativeBatch(ch, tail, cont)
			}
		case llm.StreamToolCall:
			r.calls = append(r.calls, ev.ToolCall)
		case llm.StreamDone:
			// Usage is advisory display data (the footer's context
			// readout), not a compaction input: the window still
			// counts messages (HistoryWindow).
			s.noteUsage(ev.Usage.InputTokens)
		}
		return nil
	})
	switch {
	case err == nil:
		s.closeEntry(ch, &ent, len(ent.text)) // flush the remainder, record once
		r.text = ent.text
		return r, nil
	case !got:
		// The stream never started — fall back to one Chat round trip.
		s.closeEntry(ch, &ent, len(ent.text)) // no delta ever arrived: nothing to close
		resp, cerr := s.client.Chat(s.ctx, req)
		if cerr != nil {
			return reply{}, fmt.Errorf("chat: %w", cerr)
		}
		s.noteUsage(resp.Usage.InputTokens)
		s.emitNarrative(ch, resp.Message.Content)
		return reply{text: resp.Message.Content, calls: resp.Message.ToolCalls}, nil
	default:
		// A failed stream still closes its entry: the text the player
		// already saw is displayed and recorded once, then the caller
		// rolls the turn back.
		s.closeEntry(ch, &ent, len(ent.text))
		return reply{}, fmt.Errorf("stream: %w", err)
	}
}

// closeEntry closes the entry at end: the undisplayed tail is emitted as
// one batch (continuing the entry on screen when one is already open) and
// the whole entry is recorded as exactly one chat line. Every path through
// callModel ends here, so a paragraph can never be recorded twice — or
// displayed and never recorded.
func (s *Session) closeEntry(ch tui.Channel, ent *streamEntry, end int) {
	batch, cont, record := ent.closeAt(end)
	s.emitNarrativeBatch(ch, batch, cont)
	s.recordNarrative(ch, record)
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
	for _, de := range diceEvents(res.Audit, res.Facts) {
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

// diceEvents projects an audit's rolls onto the theatre: dice values,
// modifier and total come from the roll itself, the outcome grade from the
// result's "outcome: <grade>" fact when it reports one (the engine's own
// vocabulary — critical, success, failure, fumble). It is the one mapping
// from log audit to display, shared by a live dispatch (runTool) and by
// the replay of recorded results (replayEvents) so the two cannot drift.
func diceEvents(audit *resolve.Audit, facts []string) []tui.DiceEvent {
	if audit == nil {
		return nil
	}
	outcomes := gradeOutcomes(facts)
	events := make([]tui.DiceEvent, 0, len(audit.Rolls))
	for i, roll := range audit.Rolls {
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

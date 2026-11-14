// SPDX-License-Identifier: EUPL-1.2

package session

// Offline session tests: llm.ScriptedClient replays canned turns, so every
// assertion here runs without a network. They pin the driver contract
// (session-loop.md): verbatim echo first, annotation before dice, slash
// commands handled locally, OOC tagging, IMPOSSIBLE results flowing back to
// the model, failure rollback, compaction, close semantics, RNG continuity
// across Resume and the metadata snapshot.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mythras-ultra/myul/internal/engine/resolve"
	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/harness/tools"
	"github.com/mythras-ultra/myul/internal/state"
	"github.com/mythras-ultra/myul/internal/tui"
)

// eventTimeout bounds every event wait; scripted turns are instant, so a
// timeout means a broken loop, not a slow model.
const eventTimeout = 5 * time.Second

// newTestSession builds a session over a fresh temp store. Sessions live
// under <root>/sessions, mirroring config.SessionsDir, so Close can write
// transcript.md where the tests look for it.
func newTestSession(t *testing.T, client llm.Client, tweak ...func(*Config)) *Session {
	t.Helper()
	root := t.TempDir()
	cfg := Config{
		Name:        "Test Session",
		Seed:        42,
		Store:       state.NewJSONStore(root, nil),
		Client:      client,
		Registry:    tools.NewRegistry(nil),
		SessionsDir: filepath.Join(root, "sessions"),
		Autosave:    true,
	}
	for _, fn := range tweak {
		fn(&cfg)
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("cleanup Close: %v", err)
		}
	})
	return s
}

// nextEvent waits for the next event, failing the test on timeout or a
// closed channel.
func nextEvent(t *testing.T, s *Session) tui.Event {
	t.Helper()
	select {
	case ev, ok := <-s.Events():
		if !ok {
			t.Fatal("event channel closed while waiting for an event")
		}
		return ev
	case <-time.After(eventTimeout):
		t.Fatal("timed out waiting for an event")
	}
	return tui.Event{}
}

// nextEvents reads exactly n events in arrival order.
func nextEvents(t *testing.T, s *Session, n int) []tui.Event {
	t.Helper()
	events := make([]tui.Event, 0, n)
	for range n {
		events = append(events, nextEvent(t, s))
	}
	return events
}

// kindsOf renders the event kinds of a sequence for failure messages.
func kindsOf(events []tui.Event) []tui.EventKind {
	kinds := make([]tui.EventKind, len(events))
	for i, ev := range events {
		kinds[i] = ev.Kind
	}
	return kinds
}

// drainReplay reads exactly the events Resume replayed — one per chat line
// log.jsonl held when the session was constructed (plus any resume note) —
// so a test can start from the fresh stream the runner produces afterwards.
// The count comes from the log itself: replay is read-only, so nothing
// records while it drains.
func drainReplay(t *testing.T, s *Session) {
	t.Helper()
	entries, err := s.store.ReadLog(s.ID())
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	var n int
	for _, e := range entries {
		if e.Chat != nil {
			n++
		}
	}
	for range n {
		nextEvent(t, s)
	}
}

// mustApply runs one engine move against the session's live state (the
// runner is idle — no input queued — so the test goroutine is the only
// writer).
func mustApply(t *testing.T, st *resolve.State, m resolve.Move) {
	t.Helper()
	if _, err := resolve.Apply(st, m); err != nil {
		t.Fatalf("apply %T: %v", m, err)
	}
}

// scriptedText is one canned prose reply (no tool calls).
func scriptedText(text string) llm.ScriptedTurn {
	return llm.ScriptedTurn{Response: llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: text},
	}}
}

// toolTurn is one canned reply that asks for a single tool call and says
// nothing, so it produces no narrative event.
func toolTurn(call llm.ToolCall) llm.ScriptedTurn {
	return llm.ScriptedTurn{Response: llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{call}},
	}}
}

// rollTotal sends /roll and returns the dice banner's total.
func rollTotal(t *testing.T, s *Session) int {
	t.Helper()
	if err := s.Send(t.Context(), tui.ChannelIC, "/roll 1d100"); err != nil {
		t.Fatalf("Send /roll: %v", err)
	}
	for _, ev := range nextEvents(t, s, 3) {
		if ev.Kind == tui.EventDice && ev.Dice != nil {
			return ev.Dice.Total
		}
	}
	t.Fatal("/roll produced no dice event")
	return 0
}

// TestTurnEventOrder pins the transcript ordering the TUI depends on: the
// verbatim echo first, EventThinking bracketing every model call, the
// scripted narration, the tool annotation, its dice (with the engine's
// grade as the outcome) and the closing narration — then exactly one
// EventTurnEnd — plus the model seeing the system prompt and the tool
// result.
func TestTurnEventOrder(t *testing.T) {
	client := llm.NewScripted(
		llm.ScriptedTurn{Response: llm.ChatResponse{Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "You steady your nerve.\n",
			ToolCalls: []llm.ToolCall{{
				ID:   "t1",
				Name: "skill_test",
				Args: json.RawMessage(`{"actor":"pc-1","skill":"Perception"}`),
			}},
		}}},
		scriptedText("The relic glimmers in the dark."),
	)
	s := newTestSession(t, client)
	mustApply(t, s.state, resolve.CreateCharacter{
		Name: "Wren Ashdown", PC: true, Culture: "Civilised", Career: "Hunter",
	})

	const sent = "I peer into the dark."
	if err := s.Send(t.Context(), tui.ChannelIC, sent); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 8)

	// The exact sequence the throbber and the tag grouping rely on.
	wantKinds := []tui.EventKind{
		tui.EventNarrative, // the player's echo
		tui.EventThinking,  // first model call
		tui.EventNarrative, // streamed narration
		tui.EventToolAnnotation,
		tui.EventDice,
		tui.EventThinking,  // re-invocation after the tool round
		tui.EventNarrative, // closing narration
		tui.EventTurnEnd,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	if ev := events[0]; ev.Markdown != sent || ev.Channel != tui.ChannelIC {
		t.Errorf("first event = %+v, want a verbatim echo on the IC channel", ev)
	}
	if ev := events[2]; ev.Markdown != "You steady your nerve.\n" {
		t.Errorf("third event = %+v, want the scripted narration", ev)
	}
	if ev := events[3]; ev.Annotation == nil ||
		ev.Annotation.ToolName != "skill_test" || !ev.Annotation.OK {
		t.Errorf("fourth event = %+v, want an OK skill_test annotation", ev)
	}
	if ev := events[4]; ev.Dice == nil {
		t.Errorf("fifth event = %+v, want the skill_test roll", ev)
	} else {
		if len(ev.Dice.Dice) != 1 || ev.Dice.Total != ev.Dice.Dice[0] {
			t.Errorf("dice = %+v, want one 1d100 die and Total == that die", ev.Dice)
		}
		switch ev.Dice.Outcome {
		case "critical", "success", "failure", "fumble":
		default:
			t.Errorf("outcome = %q, want the engine's grade", ev.Dice.Outcome)
		}
	}
	if ev := events[6]; ev.Markdown != "The relic glimmers in the dark." {
		t.Errorf("seventh event = %+v, want the closing narration", ev)
	}

	reqs := client.Requests()
	if len(reqs) != 2 {
		t.Fatalf("model calls = %d, want 2 (before and after the tool round)", len(reqs))
	}
	if msgs := reqs[0].Messages; len(msgs) == 0 || msgs[0].Role != llm.RoleSystem ||
		!strings.Contains(msgs[0].Content, "Test Session") {
		t.Errorf("first request has no system prompt carrying the session name")
	}
	if len(reqs[0].Tools) == 0 {
		t.Error("first request advertises no tools")
	}
	if msgs := reqs[1].Messages; len(msgs) == 0 {
		t.Fatal("second request has no messages")
	} else {
		toolMsg := msgs[len(msgs)-1]
		if toolMsg.Role != llm.RoleTool || toolMsg.ToolCallID != "t1" ||
			!strings.Contains(toolMsg.Content, `"ok":true`) {
			t.Errorf("last message = %+v, want the t1 tool result", toolMsg)
		}
	}

	// The audit log carries the play itself interleaved with the move
	// that produced it, in true order: echo, streamed narration, the
	// skill_test dispatch, closing narration.
	entries, err := s.store.ReadLog(s.ID())
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("log entries = %d, want 5 (session start, echo, narration, skill_test, closing narration)", len(entries))
	}
	if e := entries[1]; e.Kind != "chat" || e.Chat == nil ||
		!e.Chat.Player || e.Chat.Channel != "IC" || e.Chat.Kind != "narrative" || e.Chat.Text != sent {
		t.Errorf("entry 1 = %+v, want the player echo as a chat line", e)
	}
	if e := entries[2]; e.Kind != "chat" || e.Chat == nil || e.Chat.Player ||
		e.Chat.Text != "You steady your nerve.\n" {
		t.Errorf("entry 2 = %+v, want the GM narration as a chat line", e)
	}
	if entries[3].Kind != "skill_test" || entries[3].Error != "" ||
		!strings.Contains(string(entries[3].Move), `"id":"t1"`) {
		t.Errorf("log entry = %+v, want an OK skill_test recording call t1", entries[3])
	}
	if e := entries[4]; e.Kind != "chat" || e.Chat == nil || e.Chat.Player ||
		e.Chat.Text != "The relic glimmers in the dark." {
		t.Errorf("entry 4 = %+v, want the closing narration as a chat line", e)
	}
}

// TestSlashRoll checks /roll locally: it runs the roll_dice move (no model
// call), echoes first and lands as one dice event, and is audited.
func TestSlashRoll(t *testing.T) {
	s := newTestSession(t, llm.NewScripted()) // no scripted turns: the model must never be called
	if err := s.Send(t.Context(), tui.ChannelIC, "/roll 2d6+1"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 3) // echo, annotation, dice
	if ev := events[0]; ev.Kind != tui.EventNarrative || ev.Markdown != "/roll 2d6+1" {
		t.Errorf("first event = %+v, want the verbatim /roll echo", ev)
	}
	if ev := events[1]; ev.Kind != tui.EventToolAnnotation || ev.Annotation == nil ||
		ev.Annotation.ToolName != "roll_dice" || !ev.Annotation.OK {
		t.Errorf("second event = %+v, want an OK roll_dice annotation", ev)
	}
	if ev := events[2]; ev.Kind != tui.EventDice || ev.Dice == nil {
		t.Fatalf("third event = %+v, want a dice event", ev)
	} else {
		sum := 0
		for _, d := range ev.Dice.Dice {
			sum += d
		}
		if ev.Dice.Expr != "2d6+1" || len(ev.Dice.Dice) != 2 ||
			ev.Dice.Modifier != 1 || ev.Dice.Total != sum+1 {
			t.Errorf("dice = %+v, want 2d6+1 with two dice, +1 modifier, total %d", ev.Dice, sum+1)
		}
	}
	if seq := s.state.EventSeq; seq != 1 {
		t.Errorf("EventSeq = %d, want 1 — /roll must run as a move", seq)
	}
	entries, err := s.store.ReadLog(s.ID())
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(entries) != 3 || entries[1].Kind != "chat" ||
		entries[1].Chat == nil || !entries[1].Chat.Player ||
		entries[2].Kind != "roll_dice" {
		t.Fatalf("log entries = %v, want the player's /roll echo then a roll_dice entry", entries)
	}
}

// TestSlashSavePersists proves /save flushes a state change autosave is
// configured not to touch: the loaded state must carry the move.
func TestSlashSavePersists(t *testing.T) {
	s := newTestSession(t, llm.NewScripted(), func(c *Config) { c.Autosave = false })
	mustApply(t, s.state, resolve.RollDice{Expr: "1d6", Reason: "unsaved omen"})

	if err := s.Send(t.Context(), tui.ChannelIC, "/save"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	echo, saved := nextEvent(t, s), nextEvent(t, s)
	if echo.Kind != tui.EventNarrative || echo.Markdown != "/save" {
		t.Errorf("first event = %+v, want the verbatim /save echo", echo)
	}
	if saved.Kind != tui.EventSystem || !strings.Contains(saved.Markdown, "session saved") {
		t.Fatalf("second event = %+v, want a saved system line", saved)
	}
	loaded, err := s.store.Load(s.ID())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.EventSeq != 1 {
		t.Errorf("persisted EventSeq = %d, want 1 — /save must flush the pending move", loaded.EventSeq)
	}
}

// TestSlashRecap returns the stored end-of-session recaps as a system line.
func TestSlashRecap(t *testing.T) {
	s := newTestSession(t, llm.NewScripted())
	mustApply(t, s.state, resolve.EndRecap{Summary: "The ferryman lied about the crossing."})

	if err := s.Send(t.Context(), tui.ChannelIC, "/recap"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	echo, recap := nextEvent(t, s), nextEvent(t, s)
	if echo.Markdown != "/recap" {
		t.Errorf("first event = %+v, want the verbatim /recap echo", echo)
	}
	if recap.Kind != tui.EventSystem || !strings.Contains(recap.Markdown, "ferryman lied") {
		t.Errorf("second event = %+v, want the stored recap", recap)
	}
}

// TestSlashMeta dumps the panel snapshot as a system line.
func TestSlashMeta(t *testing.T) {
	s := newTestSession(t, llm.NewScripted())
	if err := s.Send(t.Context(), tui.ChannelIC, "/meta"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	echo, dump := nextEvent(t, s), nextEvent(t, s)
	if echo.Markdown != "/meta" {
		t.Errorf("first event = %+v, want the verbatim /meta echo", echo)
	}
	if dump.Kind != tui.EventSystem ||
		!strings.Contains(dump.Markdown, "session: Test Session") ||
		!strings.Contains(dump.Markdown, "decision model: disabled") {
		t.Errorf("second event = %+v, want the metadata dump", dump)
	}
}

// TestUnknownSlashCommand rejects commands the harness does not handle
// with an error line listing the usage.
func TestUnknownSlashCommand(t *testing.T) {
	s := newTestSession(t, llm.NewScripted())
	if err := s.Send(t.Context(), tui.ChannelIC, "/frobnicate"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	echo, problem := nextEvent(t, s), nextEvent(t, s)
	if echo.Markdown != "/frobnicate" {
		t.Errorf("first event = %+v, want the verbatim echo", echo)
	}
	if problem.Kind != tui.EventError || !strings.Contains(problem.Markdown, "commands") {
		t.Errorf("second event = %+v, want an error listing the commands", problem)
	}
}

// TestOOCTurnTagged sends on the OOC channel: the echo carries the channel
// (the TUI tags "[ooc]"), the model sees the message tagged in its text,
// the reply stays on the channel, and the turn is bracketed by
// thinking/turn-end events.
func TestOOCTurnTagged(t *testing.T) {
	client := llm.NewScripted(scriptedText("Out of character: the abbey lies a day east."))
	s := newTestSession(t, client)

	const sent = "How far is the abbey?"
	if err := s.Send(t.Context(), tui.ChannelOOC, sent); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 4)
	wantKinds := []tui.EventKind{
		tui.EventNarrative, // the verbatim echo
		tui.EventThinking,
		tui.EventNarrative, // the reply
		tui.EventTurnEnd,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	if ev := events[0]; ev.Markdown != sent || ev.Channel != tui.ChannelOOC {
		t.Errorf("first event = %+v, want the verbatim echo on the OOC channel", ev)
	}
	if ev := events[2]; ev.Channel != tui.ChannelOOC {
		t.Errorf("reply event = %+v, want the reply on the OOC channel", ev)
	}
	req, ok := client.LastRequest()
	if !ok {
		t.Fatal("the model was never called")
	}
	msgs := req.Messages
	user := msgs[len(msgs)-1]
	if user.Role != llm.RoleUser || user.Content != oocTag+sent {
		t.Errorf("last message = %+v, want the user text tagged %q", user, oocTag)
	}
	// The echo is recorded as an OOC player chat line — the label the
	// replay on the next Resume maps back to ChannelOOC.
	entries, err := s.store.ReadLog(s.ID())
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("log entries = %d, want 3 (session, echo, reply)", len(entries))
	}
	chat := entries[1].Chat
	if entries[1].Kind != "chat" || chat == nil || !chat.Player ||
		chat.Channel != "OOC" || chat.Text != sent {
		t.Errorf("entry 1 = %+v, want the OOC player echo", entries[1])
	}
}

// TestImpossibleResultReachesModel drives a refusal through the loop: the
// annotation shows OK=false, the IMPOSSIBLE summary goes back to the model
// as the tool message, and the turn still completes.
func TestImpossibleResultReachesModel(t *testing.T) {
	client := llm.NewScripted(
		toolTurn(llm.ToolCall{
			ID:   "t1",
			Name: "skill_test",
			Args: json.RawMessage(`{"actor":"pc-99","skill":"Perception"}`),
		}),
		scriptedText("The blow passes through the wraith."),
	)
	s := newTestSession(t, client)

	if err := s.Send(t.Context(), tui.ChannelIC, "I strike the wraith."); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 6)
	// echo, thinking, annotation (round one is silent), thinking, closing
	// narration, turn end.
	wantKinds := []tui.EventKind{
		tui.EventNarrative, tui.EventThinking, tui.EventToolAnnotation,
		tui.EventThinking, tui.EventNarrative, tui.EventTurnEnd,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	if ev := events[2]; ev.Annotation == nil || ev.Annotation.OK {
		t.Errorf("third event = %+v, want a failed annotation", ev)
	}
	if ev := events[4]; ev.Markdown != "The blow passes through the wraith." {
		t.Errorf("fifth event = %+v, want the turn to complete with narration", ev)
	}
	reqs := client.Requests()
	if len(reqs) != 2 {
		t.Fatalf("model calls = %d, want 2", len(reqs))
	}
	msgs := reqs[1].Messages
	toolMsg := msgs[len(msgs)-1]
	if toolMsg.Role != llm.RoleTool || !strings.Contains(toolMsg.Content, "IMPOSSIBLE:") {
		t.Errorf("last message = %+v, want the IMPOSSIBLE result", toolMsg)
	}
	entries, err := s.store.ReadLog(s.ID())
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(entries) != 4 || entries[2].Kind != "skill_test" ||
		!strings.Contains(entries[2].Error, "IMPOSSIBLE:") ||
		entries[3].Kind != "chat" {
		t.Errorf("log = %+v, want the refusal between the echo and its narration chat lines", entries)
	}
}

// TestModelFailureRollsBackTurn proves the failure policy: a model error
// becomes an error line, the partial turn leaves history untouched, and
// the aborted turn still ends with exactly one EventTurnEnd.
func TestModelFailureRollsBackTurn(t *testing.T) {
	s := newTestSession(t, llm.NewScripted()) // script exhausted on the first call
	if err := s.Send(t.Context(), tui.ChannelIC, "Hello?"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 4)
	// echo, thinking (the call was attempted), error, turn end.
	wantKinds := []tui.EventKind{
		tui.EventNarrative, tui.EventThinking, tui.EventError, tui.EventTurnEnd,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	if echo := events[0]; echo.Markdown != "Hello?" {
		t.Errorf("first event = %+v, want the verbatim echo", echo)
	}
	if problem := events[2]; !strings.Contains(problem.Markdown, "rolled back") {
		t.Errorf("third event = %+v, want a rollback error line", problem)
	}
	if len(s.history) != 0 {
		t.Errorf("history has %d messages, want the turn rolled back to nothing", len(s.history))
	}
}

// TestCompactionWindowsHistory pins the context policy: after a turn
// overflows the window, older turns are replaced by one summary message
// and the last window stays verbatim.
func TestCompactionWindowsHistory(t *testing.T) {
	client := llm.NewScripted(
		scriptedText("First reply."),
		scriptedText("Second reply."),
		scriptedText("The duke betrayed you at the ford."), // the compaction summary
	)
	s := newTestSession(t, client, func(c *Config) { c.HistoryWindow = 2 })

	for i, text := range []string{"hello one", "hello two"} {
		if err := s.Send(t.Context(), tui.ChannelIC, text); err != nil {
			t.Fatalf("Send %q: %v", text, err)
		}
		// Turn one: echo, thinking, narration, turn end. Turn two adds
		// the compaction's own thinking event before its turn end.
		n := 4
		if i == 1 {
			n = 5
		}
		nextEvents(t, s, n)
	}
	// The summary call runs at the end of turn two; a local command runs
	// strictly after it (one runner goroutine), so its events prove
	// compaction finished before the assertions below.
	if err := s.Send(t.Context(), tui.ChannelIC, "/meta"); err != nil {
		t.Fatalf("Send /meta: %v", err)
	}
	nextEvents(t, s, 2)

	if len(s.history) != 3 {
		t.Fatalf("history = %d messages, want summary + the last window (3)", len(s.history))
	}
	if first := s.history[0]; first.Role != llm.RoleUser ||
		!strings.HasPrefix(first.Content, contextPrefix) ||
		!strings.Contains(first.Content, "duke betrayed you") {
		t.Errorf("first message = %+v, want the compaction summary up front", first)
	}
	if s.history[1].Content != "hello two" {
		t.Errorf("second message = %q, want the windowed turn verbatim", s.history[1].Content)
	}
	if last := s.history[2]; last.Role != llm.RoleAssistant || last.Content != "Second reply." {
		t.Errorf("third message = %+v, want the windowed reply verbatim", last)
	}
	for i, msg := range s.history {
		if strings.Contains(msg.Content, "hello one") {
			t.Errorf("message %d still carries the compacted turn: %q", i, msg.Content)
		}
	}
}

// TestCloseIsIdempotentWritesTranscript closes twice, checks the transcript
// and state landed on disk, that the event channel closes, and that Send
// refuses input afterwards.
func TestCloseIsIdempotentWritesTranscript(t *testing.T) {
	s := newTestSession(t, llm.NewScripted())
	if err := s.Send(t.Context(), tui.ChannelIC, "/roll 1d4"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	nextEvents(t, s, 3)

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("second Close = %v, want nil (idempotent)", err)
	}
	if _, ok := <-s.Events(); ok {
		t.Error("events channel is still open after Close")
	}
	dir := filepath.Join(s.sessionsDir, s.ID())
	for _, name := range []string{"state.json", "transcript.md", "chat.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s missing after Close: %v", name, err)
		}
	}
	doc, err := os.ReadFile(filepath.Join(dir, "transcript.md"))
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if text := string(doc); !strings.Contains(text, "# Test Session — Play Log") ||
		!strings.Contains(text, "roll_dice") {
		t.Errorf("transcript = %q, want the header and the recorded roll", text)
	}
	if err := s.Send(t.Context(), tui.ChannelIC, "one more"); !errors.Is(err, ErrClosed) {
		t.Errorf("Send after Close = %v, want ErrClosed", err)
	}
}

// TestResumeContinuesRNGStream plays two rolls, interrupts between them
// with Close/Resume, and compares against an uninterrupted reference with
// the same seed: the resumed pair must match roll for roll.
func TestResumeContinuesRNGStream(t *testing.T) {
	root := t.TempDir()
	cfg := func() Config {
		return Config{
			Name:        "Seed Run",
			Seed:        42,
			Store:       state.NewJSONStore(root, nil),
			Registry:    tools.NewRegistry(nil),
			SessionsDir: filepath.Join(root, "sessions"),
			Autosave:    true,
		}
	}

	// Reference: one session, two rolls.
	ref, err := New(cfg())
	if err != nil {
		t.Fatalf("New reference: %v", err)
	}
	wantFirst, wantSecond := rollTotal(t, ref), rollTotal(t, ref)
	if err := ref.Close(); err != nil {
		t.Fatalf("Close reference: %v", err)
	}

	// Interrupted: roll, close, resume, roll.
	first, err := New(cfg())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	gotFirst := rollTotal(t, first)
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	continued, err := Resume(cfg(), first.ID())
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	drainReplay(t, continued) // the recorded /roll echo replays first
	gotSecond := rollTotal(t, continued)
	if err := continued.Close(); err != nil {
		t.Fatalf("Close resumed: %v", err)
	}

	if gotFirst != wantFirst || gotSecond != wantSecond {
		t.Errorf("interrupted rolls = %d, %d; uninterrupted reference = %d, %d — "+
			"Resume must continue the persisted RNG stream",
			gotFirst, gotSecond, wantFirst, wantSecond)
	}
}

// TestMetadataSnapshot builds a prepared state (NPC first, so the PC is
// not the first character by ID) and checks the panel snapshot: PC
// selection, vitals, per-location HP, inventory, scene with resolved
// present, clocks, flags and the calendar.
func TestMetadataSnapshot(t *testing.T) {
	s := newTestSession(t, llm.NewScripted())
	st := s.state
	mustApply(t, st, resolve.CreateCharacter{
		Name: "Grim the Fence", Culture: "Barbarian", Career: "Hunter",
	})
	mustApply(t, st, resolve.CreateCharacter{
		Name: "Ynara Vex", PC: true, Culture: "Civilised", Career: "Hunter",
	})
	mustApply(t, st, resolve.InventoryAdd{Actor: "pc-1", Item: "Rope", Qty: 3})
	mustApply(t, st, resolve.SetScene{
		Location:    "The flooded crypt",
		Description: "black water to the knee",
		Present:     []string{"pc-1", "the ferryman"},
	})
	mustApply(t, st, resolve.AddClock{Name: "Tide rising", Segments: 6})
	mustApply(t, st, resolve.SetWorldFlag{Key: "economy", Value: "strained", Reason: "the war"})

	m := s.Metadata()
	if m.SessionName != "Test Session" {
		t.Errorf("SessionName = %q, want the session name", m.SessionName)
	}
	if m.Character.Name != "Ynara Vex" {
		t.Errorf("Character.Name = %q, want the first pc:true character, not the NPC", m.Character.Name)
	}
	pc := st.Characters["pc-1"]
	if pc == nil {
		t.Fatal("pc-1 missing from the prepared state")
	}
	var current int
	for _, hp := range pc.CurrentHP {
		current += hp
	}
	if want := strconv.Itoa(current) + "/" + strconv.Itoa(pc.Derived.HitPoints); m.Character.HP != want {
		t.Errorf("HP = %q, want %q", m.Character.HP, want)
	}
	if m.Character.AP != "" {
		t.Errorf("AP = %q, want empty outside an encounter", m.Character.AP)
	}
	if len(m.Character.Locations) == 0 {
		t.Fatal("no hit locations in the snapshot")
	}
	for _, loc := range m.Character.Locations {
		if want := pc.Derived.LocationHP[loc.Name]; loc.Max != want {
			t.Errorf("location %q max = %d, want %d", loc.Name, loc.Max, want)
		}
	}
	if len(m.Inventory) != 1 || m.Inventory[0] != "Rope ×3" {
		t.Errorf("Inventory = %v, want [Rope ×3]", m.Inventory)
	}
	if !strings.Contains(m.Scene, "The flooded crypt") || !strings.Contains(m.Scene, "black water to the knee") {
		t.Errorf("Scene = %q, want the location with its description", m.Scene)
	}
	if want := []string{"Ynara Vex", "the ferryman"}; !equalStrings(m.Present, want) {
		t.Errorf("Present = %v, want %v (character IDs resolved to names)", m.Present, want)
	}
	if want := []string{"Tide rising: 0/6"}; !equalStrings(m.Clocks, want) {
		t.Errorf("Clocks = %v, want %v", m.Clocks, want)
	}
	if want := []string{"economy = strained"}; !equalStrings(m.Flags, want) {
		t.Errorf("Flags = %v, want %v", m.Flags, want)
	}
	if !strings.HasPrefix(m.InGameDate, "Year ") {
		t.Errorf("InGameDate = %q, want a Year label", m.InGameDate)
	}
}

// TestToolLoopBound stops a model that never stops calling tools: the loop
// yields after maxToolRounds dispatch rounds with an error line, the
// history never ends on unanswered tool calls, and the state kept every
// accepted move.
func TestToolLoopBound(t *testing.T) {
	call := llm.ToolCall{ID: "t", Name: "roll_dice", Args: json.RawMessage(`{"expr":"1d6","reason":"bound"}`)}
	turns := make([]llm.ScriptedTurn, maxToolRounds+1)
	for i := range turns {
		turns[i] = toolTurn(call)
	}
	client := llm.NewScripted(turns...)
	// A generous window keeps this test about the bound: compaction would
	// spend one more scripted turn summarising 60+ messages.
	s := newTestSession(t, client, func(c *Config) { c.HistoryWindow = 1000 })

	if err := s.Send(t.Context(), tui.ChannelIC, "keep rolling"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	var problem tui.Event
	// The run-up to the bound error: the echo, one EventThinking per model
	// call (maxToolRounds+1), the annotation/dice pair per dispatch round,
	// and the error itself — with slack so the bound, not the count, is
	// what this test measures.
	for range 3*maxToolRounds + 8 {
		ev := nextEvent(t, s)
		if ev.Kind == tui.EventError {
			problem = ev
			break
		}
	}
	if problem.Markdown == "" {
		t.Fatal("no bound error arrived for an endless tool loop")
	}
	if !strings.Contains(problem.Markdown, strconv.Itoa(maxToolRounds)) {
		t.Errorf("error = %q, want it to name the %d-round bound", problem.Markdown, maxToolRounds)
	}
	if got := len(client.Requests()); got != maxToolRounds+1 {
		t.Errorf("model calls = %d, want %d (bounded loop)", got, maxToolRounds+1)
	}
	if seq := s.state.EventSeq; seq != maxToolRounds {
		t.Errorf("EventSeq = %d, want %d accepted rolls", seq, maxToolRounds)
	}
	if last := s.history[len(s.history)-1]; len(last.ToolCalls) > 0 {
		t.Errorf("history ends with unanswered tool calls: %+v", last)
	}
}

// blockingClient is an llm.Client whose model call hangs until its context
// is cancelled — a stand-in for a provider that stalls mid-turn, used to
// prove Close reaps the runner goroutine instead of deadlocking on it.
type blockingClient struct {
	entered chan struct{} // closed on the first model call
	once    sync.Once
}

// Chat implements [llm.Client].
func (b *blockingClient) Chat(ctx context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
	b.enter()
	<-ctx.Done()
	return llm.ChatResponse{}, ctx.Err()
}

// Stream implements [llm.Client]; it never emits, so the session falls back
// to Chat (which then reports the cancelled context).
func (b *blockingClient) Stream(ctx context.Context, _ llm.ChatRequest, _ func(llm.StreamEvent) error) error {
	b.enter()
	<-ctx.Done()
	return ctx.Err()
}

// enter signals that a turn reached the model, exactly once.
func (b *blockingClient) enter() { b.once.Do(func() { close(b.entered) }) }

// TestCloseReapsRunnerDuringInFlightTurn closes the session while the model
// call hangs: Close must cancel the turn context, reap the runner goroutine
// (the done channel) and only then close Events — no deadlock, no leak
// under -race.
func TestCloseReapsRunnerDuringInFlightTurn(t *testing.T) {
	client := &blockingClient{entered: make(chan struct{})}
	s := newTestSession(t, client)
	if err := s.Send(t.Context(), tui.ChannelIC, "a slow question"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	select {
	case <-client.entered:
	case <-time.After(eventTimeout):
		t.Fatal("the turn never reached the model")
	}

	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close during a turn: %v", err)
		}
	case <-time.After(eventTimeout):
		t.Fatal("Close deadlocked: the runner did not exit while a turn was in flight")
	}
	select {
	case <-s.done: // closed by the runner's deferred close on exit
	default:
		t.Fatal("runner goroutine still alive after Close returned")
	}
	// Drain buffered events: the receive reports ok only while values
	// remain, so the channel's closure surfaces at the end of the drain.
	timeout := time.After(eventTimeout)
drain:
	for {
		select {
		case _, ok := <-s.Events():
			if !ok {
				break drain // channel closed — the driver contract holds
			}
		case <-timeout:
			t.Fatal("events channel still open after Close")
		}
	}
}

// equalStrings compares string slices element-wise.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestStripChannelTag pins the sanitisation of a leading channel marker in
// model output: only the very start of the string may change, and only the
// marker itself.
func TestStripChannelTag(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"leading IC marker", "[IC] The door swings open.", "The door swings open."},
		{"leading OOC marker with colon", "[ooc]: Sure.", "Sure."},
		{"marker after leading whitespace", "  [ic]  The bell tolls.", "The bell tolls."},
		{"marker without separator", "[ic]The crypt is cold.", "The crypt is cold."},
		{"interior marker untouched", "You see [ic] scratched into the wall.", "You see [ic] scratched into the wall."},
		{"markdown link untouched", "[ICest](https://example.test) stays.", "[ICest](https://example.test) stays."},
		{"marker only", "[OOC]", ""},
		{"plain text untouched", "The bell tolls twice.", "The bell tolls twice."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripChannelTag(tt.in); got != tt.want {
				t.Errorf("stripChannelTag(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestNarrativeChannelMarkerStripped drives model output carrying inline
// channel markers through a whole turn: the leading marker never reaches
// the transcript, interior text and the player's own echo are untouched,
// and the turn still ends with exactly one EventTurnEnd.
func TestNarrativeChannelMarkerStripped(t *testing.T) {
	client := llm.NewScripted(scriptedText("[IC] The door swings open.\nSecond line [ic] stays."))
	s := newTestSession(t, client)

	const sent = "[ic] I knock twice." // the player's echo keeps its text
	if err := s.Send(t.Context(), tui.ChannelIC, sent); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 4)
	wantKinds := []tui.EventKind{
		tui.EventNarrative, tui.EventThinking,
		tui.EventNarrative, tui.EventTurnEnd,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	if events[0].Markdown != sent {
		t.Errorf("echo = %q, want the player's text verbatim — sanitisation never touches it", events[0].Markdown)
	}
	// One paragraph-flush now: the two soft-wrapped lines are one Markdown
	// paragraph, so they arrive as a single narrative entry with only the
	// leading marker stripped.
	if got := events[2].Markdown; got != "The door swings open.\nSecond line [ic] stays." {
		t.Errorf("narrative = %q, want leading [IC] stripped and interior [ic] untouched", got)
	}
}

// TestMarkerOnlyNarrativeEmitsNothing proves the empty-body handling: a
// reply that is only a channel marker produces no narrative event, while
// the turn still ends normally.
func TestMarkerOnlyNarrativeEmitsNothing(t *testing.T) {
	client := llm.NewScripted(scriptedText("[OOC]"))
	s := newTestSession(t, client)

	if err := s.Send(t.Context(), tui.ChannelIC, "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 3)
	wantKinds := []tui.EventKind{tui.EventNarrative, tui.EventThinking, tui.EventTurnEnd}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
}

// TestTurnEndPersistsChat pins the fix for "quitting erases the chat": a
// narrative-only turn — no moves, nothing state autosave would catch —
// lands in chat.json at turn end (before any Close), carrying the exact
// exchange.
func TestTurnEndPersistsChat(t *testing.T) {
	client := llm.NewScripted(scriptedText("The relic sleeps."))
	s := newTestSession(t, client)

	const sent = "I wake the relic."
	if err := s.Send(t.Context(), tui.ChannelIC, sent); err != nil {
		t.Fatalf("Send: %v", err)
	}
	nextEvents(t, s, 4) // echo, thinking, narration, turn end

	data, err := s.store.LoadChat(s.ID())
	if err != nil {
		t.Fatalf("LoadChat after turn end (autosave on): %v", err)
	}
	var history []llm.Message
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatalf("decode chat.json: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("persisted history = %d messages, want the exchange (2)", len(history))
	}
	if history[0].Role != llm.RoleUser || history[0].Content != sent {
		t.Errorf("first message = %+v, want the player's line", history[0])
	}
	if history[1].Role != llm.RoleAssistant || history[1].Content != "The relic sleeps." {
		t.Errorf("second message = %+v, want the GM's reply", history[1])
	}
}

// TestSlashSavePersistsChatWithoutAutosave proves /save carries the
// conversation to disk when autosave is off: the finished turn writes no
// chat.json, /save does.
func TestSlashSavePersistsChatWithoutAutosave(t *testing.T) {
	s := newTestSession(t, llm.NewScripted(scriptedText("A hush falls.")),
		func(c *Config) { c.Autosave = false })

	if err := s.Send(t.Context(), tui.ChannelIC, "I listen."); err != nil {
		t.Fatalf("Send: %v", err)
	}
	nextEvents(t, s, 4)
	if _, err := s.store.LoadChat(s.ID()); !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("LoadChat without autosave = %v, want ErrNotFound (nothing written yet)", err)
	}

	if err := s.Send(t.Context(), tui.ChannelIC, "/save"); err != nil {
		t.Fatalf("Send /save: %v", err)
	}
	echo, saved := nextEvent(t, s), nextEvent(t, s)
	if echo.Markdown != "/save" || saved.Kind != tui.EventSystem ||
		!strings.Contains(saved.Markdown, "session saved") {
		t.Fatalf("events = %+v, want the /save echo then a saved system line", []tui.Event{echo, saved})
	}
	data, err := s.store.LoadChat(s.ID())
	if err != nil {
		t.Fatalf("LoadChat after /save: %v", err)
	}
	var history []llm.Message
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatalf("decode chat.json: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("persisted history = %d messages, want the exchange (2)", len(history))
	}
}

// TestQuitResumeRoundTrip is the quit→resume contract end to end: a
// scripted turn, Close, then Resume must restore the conversation (the
// next model call carries the earlier exchange), replay the recorded play
// exactly once and in order without duplicating a single log entry, and
// keep the game state intact.
func TestQuitResumeRoundTrip(t *testing.T) {
	root := t.TempDir()
	client := llm.NewScripted(
		scriptedText("The door opens."),
		scriptedText("It closes behind you."),
	)
	cfg := func() Config {
		return Config{
			Name:        "Round Trip",
			Seed:        42,
			Store:       state.NewJSONStore(root, nil),
			Client:      client,
			Registry:    tools.NewRegistry(nil),
			SessionsDir: filepath.Join(root, "sessions"),
			Autosave:    true,
		}
	}
	first, err := New(cfg())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() }) // idempotent with the explicit Close below
	const sent = "I open the door."
	if err := first.Send(t.Context(), tui.ChannelIC, sent); err != nil {
		t.Fatalf("Send: %v", err)
	}
	nextEvents(t, first, 4) // echo, thinking, narration, turn end
	logBefore, err := first.store.ReadLog(first.ID())
	if err != nil {
		t.Fatalf("ReadLog before Close: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	resumed, err := Resume(cfg(), first.ID())
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	t.Cleanup(func() {
		if err := resumed.Close(); err != nil {
			t.Errorf("cleanup Close resumed: %v", err)
		}
	})

	// The prior play replays first, in recorded order, exactly once:
	// the player's echo, then the GM's narration.
	replay := nextEvents(t, resumed, 2)
	if replay[0].Kind != tui.EventNarrative || replay[0].Markdown != sent ||
		replay[1].Kind != tui.EventNarrative || replay[1].Markdown != "The door opens." {
		t.Fatalf("replay = %+v, want the recorded echo then narration", replay)
	}
	// Replay is read-only: the log did not grow by a re-recorded line.
	logAfter, err := resumed.store.ReadLog(resumed.ID())
	if err != nil {
		t.Fatalf("ReadLog after replay: %v", err)
	}
	if len(logAfter) != len(logBefore) {
		t.Errorf("log grew across replay: %d → %d entries, want no duplicates",
			len(logBefore), len(logAfter))
	}

	// History is seeded: the next model call carries the earlier exchange.
	if err := resumed.Send(t.Context(), tui.ChannelIC, "I shut it again."); err != nil {
		t.Fatalf("Send after resume: %v", err)
	}
	nextEvents(t, resumed, 4)
	req, ok := client.LastRequest()
	if !ok {
		t.Fatal("the model was never called after resume")
	}
	var sawUser, sawAssistant bool
	for _, m := range req.Messages {
		if m.Role == llm.RoleUser && m.Content == sent {
			sawUser = true
		}
		if m.Role == llm.RoleAssistant && m.Content == "The door opens." {
			sawAssistant = true
		}
	}
	if !sawUser || !sawAssistant {
		t.Errorf("post-resume request misses the seeded history (user=%v assistant=%v): %+v",
			sawUser, sawAssistant, req.Messages)
	}

	// State intact: same identity and session name on both sides.
	if resumed.ID() != first.ID() {
		t.Errorf("resumed id = %q, want %q", resumed.ID(), first.ID())
	}
	if name := resumed.Metadata().SessionName; name != "Round Trip" {
		t.Errorf("SessionName = %q, want %q", name, "Round Trip")
	}
}

// TestResumeWithoutChatStartsEmptyWithNote covers the recovery path: a
// corrupt chat.json on resume yields an empty history plus one system
// event saying so, while the recorded play still replays behind it.
func TestResumeWithoutChatStartsEmptyWithNote(t *testing.T) {
	root := t.TempDir()
	client := llm.NewScripted(scriptedText("A hush falls."))
	cfg := func() Config {
		return Config{
			Name:        "Lost Chat",
			Seed:        42,
			Store:       state.NewJSONStore(root, nil),
			Client:      client,
			Registry:    tools.NewRegistry(nil),
			SessionsDir: filepath.Join(root, "sessions"),
			Autosave:    true,
		}
	}
	first, err := New(cfg())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() }) // idempotent with the explicit Close below
	if err := first.Send(t.Context(), tui.ChannelIC, "I listen."); err != nil {
		t.Fatalf("Send: %v", err)
	}
	nextEvents(t, first, 4)
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Corrupt the conversation snapshot; the log stays intact.
	chatPath := filepath.Join(root, "sessions", first.ID(), "chat.json")
	if err := os.WriteFile(chatPath, []byte(`{truncated`), 0o644); err != nil {
		t.Fatalf("corrupt chat.json: %v", err)
	}
	resumed, err := Resume(cfg(), first.ID())
	if err != nil {
		t.Fatalf("Resume with corrupt chat: %v", err)
	}
	t.Cleanup(func() {
		if err := resumed.Close(); err != nil {
			t.Errorf("cleanup Close resumed: %v", err)
		}
	})
	if len(resumed.history) != 0 {
		t.Errorf("history = %d messages, want empty on a corrupt chat.json", len(resumed.history))
	}
	note := nextEvent(t, resumed)
	if note.Kind != tui.EventSystem || !strings.Contains(note.Markdown, "empty history") {
		t.Errorf("first event = %+v, want the resume note", note)
	}
	echo := nextEvent(t, resumed)
	if echo.Kind != tui.EventNarrative || echo.Markdown != "I listen." {
		t.Errorf("second event = %+v, want the replayed echo after the note", echo)
	}
}

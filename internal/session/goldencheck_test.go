// SPDX-License-Identifier: EUPL-1.2

package session

// Offline tests for the golden-rule check (docs/design/session-loop.md
// "Golden rule check"): the decision model gates an in-character assistant
// reply after its tools ran — confidence boundary, exact "Y/n" prompt
// coexisting with the channel check's "N/y" hold, the off-screen rewrite
// loop (nudge request, stripped tool calls, MsgSeq replacement event,
// tombstone + note), "n" keeping the reply as-is, and every way the check
// can be off or broken passing silently. The decide side is an httptest
// /v1/systemone stub, so nothing here touches a network.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mythras-ultra/myul/internal/harness/decide"
	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/state"
	"github.com/mythras-ultra/myul/internal/tui"
)

// goldenVerdict is one canned decision-model answer.
type goldenVerdict struct {
	choice     string
	confidence float64
}

// decideSeqStub is an offline decision model that answers each call with
// the next canned verdict (the last one repeats), counting its calls and
// remembering the last request body — the golden-rule loop needs
// violate → comply sequences the channel check's fixed-choice decideStub
// cannot express.
type decideSeqStub struct {
	client   *decide.Client
	calls    atomic.Int64
	mu       sync.Mutex
	lastBody string
	answers  []goldenVerdict
}

// newDecideSeqStub starts a stub replying with answers in call order,
// wired into an enabled decide.Client. It fails the test on any request
// that is not POST /v1/systemone, or on a question that does not offer
// the verdict it is about to answer with.
func newDecideSeqStub(t *testing.T, answers ...goldenVerdict) *decideSeqStub {
	t.Helper()
	if len(answers) == 0 {
		t.Fatal("the sequence stub needs at least one verdict")
	}
	stub := &decideSeqStub{answers: answers}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(stub.calls.Add(1)) - 1
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			t.Errorf("decide request = %s %s, want POST /v1/systemone", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read decide request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var req struct {
			Questions map[string]struct {
				Criteria map[string]*string `json:"criteria"`
			} `json:"questions"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("decode decide request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		stub.mu.Lock()
		stub.lastBody = string(body)
		stub.mu.Unlock()
		if n >= len(stub.answers) {
			n = len(stub.answers) - 1 // the last verdict repeats
		}
		want := stub.answers[n]
		answers := make(map[string]any, len(req.Questions))
		for label, q := range req.Questions {
			if _, ok := q.Criteria[want.choice]; !ok {
				t.Errorf("question %q does not offer %q", label, want.choice)
			}
			answers[label] = map[string]any{
				"type":          "choice",
				"choice":        want.choice,
				"probabilities": map[string]float64{want.choice: want.confidence},
				"confidence":    want.confidence,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "tev1:4b", "answers": answers})
	}))
	t.Cleanup(srv.Close)
	stub.client = decide.New(decide.Config{Enabled: true, URL: srv.URL})
	return stub
}

// body returns the last decide request the stub received.
func (s *decideSeqStub) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastBody
}

// goldenSession builds a session whose golden-rule check runs against stub
// (the channel check stays off unless a test says otherwise).
func goldenSession(t *testing.T, client llm.Client, stub *decideSeqStub) *Session {
	t.Helper()
	return newTestSession(t, client, func(cfg *Config) {
		cfg.GMCheck = true
		cfg.Decide = stub.client
	})
}

// goldenMessage is a player message the golden-rule check judges.
const goldenMessage = "I cross the courtyard and knock twice on the iron door."

// goldenChecks decodes every golden_rule_check audit entry of the log, in
// recorded order.
func goldenChecks(t *testing.T, s *Session) []goldenCheckMove {
	t.Helper()
	entries, err := s.store.ReadLog(s.ID())
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	var out []goldenCheckMove
	for _, e := range entries {
		if e.Kind != state.GoldenRuleCheckKind {
			continue
		}
		var m goldenCheckMove
		if err := json.Unmarshal(e.Move, &m); err != nil {
			t.Fatalf("decode golden-rule move %q: %v", e.Move, err)
		}
		out = append(out, m)
	}
	return out
}

// chatLines decodes every chat line of the log, in recorded order.
func chatLines(t *testing.T, s *Session) []state.ChatLine {
	t.Helper()
	entries, err := s.store.ReadLog(s.ID())
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	var out []state.ChatLine
	for _, e := range entries {
		if e.Chat != nil {
			out = append(out, *e.Chat)
		}
	}
	return out
}

// TestGoldenCheckGateTable pins the gate: a "violates" verdict acts only
// at channelCheckMinConfidence (0.70 — 0.69 passes silently) and only when
// the check is on, the turn is in character, the reply carries text and a
// decision client exists — every other path never prompts, and the off
// switches never make a single decide call.
func TestGoldenCheckGateTable(t *testing.T) {
	t.Run("violates at 0.70 prompts and 0.69 passes", func(t *testing.T) {
		for _, tt := range []struct {
			name       string
			verdict    goldenVerdict
			wantPrompt bool
		}{
			{name: "0.70 prompts", verdict: goldenVerdict{goldenOptionViolates, 0.70}, wantPrompt: true},
			{name: "0.69 passes silently", verdict: goldenVerdict{goldenOptionViolates, 0.69}},
			{name: "complies passes silently", verdict: goldenVerdict{goldenOptionComplies, 0.99}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				stub := newDecideSeqStub(t, tt.verdict)
				client := llm.NewScripted(scriptedText("The iron door opens a hand's width."))
				s := goldenSession(t, client, stub)
				if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
					t.Fatalf("Send: %v", err)
				}
				events := nextEvents(t, s, 4)
				wantKinds := []tui.EventKind{
					tui.EventNarrative, // the echo
					tui.EventThinking,
					tui.EventNarrative, // the scripted reply
					tui.EventTurnEnd,
				}
				if tt.wantPrompt {
					wantKinds[3] = tui.EventConfirmRegen
				}
				for i, want := range wantKinds {
					if events[i].Kind != want {
						t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
					}
				}
				if got := stub.calls.Load(); got != 1 {
					t.Errorf("decide calls = %d, want 1", got)
				}
				// The player's message is the judgement context, tagged
				// with the reply sequence so every check is a fresh
				// question for decide's verdict cache.
				if body := stub.body(); !strings.Contains(body, goldenMessage) ||
					!strings.Contains(body, "Reply under judgement") {
					t.Errorf("decide state should carry the player message and reply sequence, got %.300q", body)
				}
				if tt.wantPrompt {
					if err := s.Command("confirm", "n"); err != nil {
						t.Fatalf("Command confirm: %v", err)
					}
					if ev := nextEvent(t, s); ev.Kind != tui.EventTurnEnd {
						t.Errorf("event after n = %v, want the turn end", ev.Kind)
					}
				}
			})
		}
	})

	t.Run("check off makes no decide calls", func(t *testing.T) {
		var calls atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			t.Errorf("decide called with decision.check_gm_output off: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)
		client := llm.NewScripted(scriptedText("Straight through."))
		s := newTestSession(t, client, func(cfg *Config) {
			cfg.GMCheck = false
			cfg.Decide = decide.New(decide.Config{Enabled: true, URL: srv.URL})
		})
		if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
			t.Fatalf("Send: %v", err)
		}
		events := nextEvents(t, s, 4)
		if events[3].Kind != tui.EventTurnEnd {
			t.Errorf("fourth event = %v, want a turn end with no prompt", events[3].Kind)
		}
		if got := calls.Load(); got != 0 {
			t.Errorf("decide calls = %d, want 0 with the check off", got)
		}
	})

	t.Run("OOC turns are skipped", func(t *testing.T) {
		stub := newDecideSeqStub(t, goldenVerdict{goldenOptionViolates, 0.99})
		client := llm.NewScripted(scriptedText("Out of character: sure, we can pause here."))
		s := goldenSession(t, client, stub)
		if err := s.Send(t.Context(), tui.ChannelOOC, goldenMessage); err != nil {
			t.Fatalf("Send: %v", err)
		}
		events := nextEvents(t, s, 4)
		if events[3].Kind != tui.EventTurnEnd {
			t.Errorf("fourth event = %v, want a turn end with no prompt", events[3].Kind)
		}
		if got := stub.calls.Load(); got != 0 {
			t.Errorf("decide calls = %d, want 0 on the OOC channel", got)
		}
	})

	t.Run("empty replies are skipped", func(t *testing.T) {
		stub := newDecideSeqStub(t, goldenVerdict{goldenOptionViolates, 0.99})
		client := llm.NewScripted(scriptedText("   "))
		s := goldenSession(t, client, stub)
		if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
			t.Fatalf("Send: %v", err)
		}
		// Echo, thinking, turn end: whitespace-only text emits no
		// narrative batch and is never judged.
		events := nextEvents(t, s, 3)
		wantKinds := []tui.EventKind{tui.EventNarrative, tui.EventThinking, tui.EventTurnEnd}
		for i, want := range wantKinds {
			if events[i].Kind != want {
				t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
			}
		}
		if got := stub.calls.Load(); got != 0 {
			t.Errorf("decide calls = %d, want 0 for an empty reply", got)
		}
	})

	t.Run("nil decision client passes silently", func(t *testing.T) {
		client := llm.NewScripted(scriptedText("Nothing stops the scene."))
		s := newTestSession(t, client, func(cfg *Config) { cfg.GMCheck = true })
		if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
			t.Fatalf("Send: %v", err)
		}
		events := nextEvents(t, s, 4)
		if events[3].Kind != tui.EventTurnEnd {
			t.Errorf("fourth event = %v, want a turn end with no prompt", events[3].Kind)
		}
	})

	t.Run("unreachable decision service passes silently", func(t *testing.T) {
		dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		dead.Close() // nothing listens: the decide call fails with ErrUnavailable
		client := llm.NewScripted(scriptedText("Nothing stops the scene."))
		s := newTestSession(t, client, func(cfg *Config) {
			cfg.GMCheck = true
			cfg.Decide = decide.New(decide.Config{Enabled: true, URL: dead.URL})
		})
		if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
			t.Fatalf("Send: %v", err)
		}
		events := nextEvents(t, s, 4)
		if events[3].Kind != tui.EventTurnEnd {
			t.Errorf("fourth event = %v, want a turn end with no prompt (a broken decide never fails a turn)", events[3].Kind)
		}
		if got := len(goldenChecks(t, s)); got != 0 {
			t.Errorf("golden_rule_check entries = %d, want 0 when decide errored (no verdict to audit)", got)
		}
	})
}

// TestGoldenCheckPromptsCoexistWithChannelCheck pins both confirm
// variants in one session: the channel check still holds its message with
// the exact "N/y" question (Enter = n there), and the golden-rule check
// prompts afterwards with the exact "Y/n" question and no draft text
// (Enter = y here — the TUI side is tui.TestRegenConfirmMode).
func TestGoldenCheckPromptsCoexistWithChannelCheck(t *testing.T) {
	const (
		channelQuestion = "You are in IC mode, but your response looks like a message for OOC. Is this correct? N/y"
		regenQuestion   = "The Game Master appears to have decided your character's actions for you. This is a violation of a golden rule in roleplaying. Regenerate Game Master's response? Y/n"
	)
	// Call order: the channel gate judges the message before its echo,
	// the golden gate the reply afterwards.
	stub := newDecideSeqStub(t,
		goldenVerdict{"out-of-character", 0.9},
		goldenVerdict{goldenOptionViolates, 0.9},
	)
	client := llm.NewScripted(scriptedText("The door swings inward on oiled hinges."))
	s := newTestSession(t, client, func(cfg *Config) {
		cfg.ChannelCheck = true
		cfg.GMCheck = true
		cfg.Decide = stub.client
	})
	if err := s.Send(t.Context(), tui.ChannelIC, longMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	ev := nextEvent(t, s)
	if ev.Kind != tui.EventConfirm || ev.Markdown != channelQuestion {
		t.Fatalf("first event = %+v, want the channel check's N/y hold", ev)
	}
	if ev.ConfirmText != longMessage {
		t.Errorf("ConfirmText = %q, want the held message", ev.ConfirmText)
	}
	if err := s.Command("confirm", "y"); err != nil {
		t.Fatalf("Command confirm: %v", err)
	}
	events := nextEvents(t, s, 4)
	wantKinds := []tui.EventKind{
		tui.EventNarrative, // the echo
		tui.EventThinking,
		tui.EventNarrative, // the scripted reply
		tui.EventConfirmRegen,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	if events[3].Markdown != regenQuestion {
		t.Errorf("regen question = %q, want the exact Y/n prompt", events[3].Markdown)
	}
	if events[3].ConfirmText != "" {
		t.Errorf("ConfirmText = %q, want empty — the regen prompt carries no draft", events[3].ConfirmText)
	}
	if err := s.Command("confirm", "n"); err != nil {
		t.Fatalf("Command confirm: %v", err)
	}
	if ev := nextEvent(t, s); ev.Kind != tui.EventTurnEnd {
		t.Errorf("event after n = %v, want the turn end", ev.Kind)
	}
}

// TestGoldenCheckDispatchEventsBeforeConfirm pins the order inside one
// turn: a reply's tool dispatch (annotation and dice on screen) happens
// BEFORE the regeneration prompt is filed — the check waits until the
// message's tools have run, and the reply has not entered history yet.
func TestGoldenCheckDispatchEventsBeforeConfirm(t *testing.T) {
	stub := newDecideSeqStub(t,
		goldenVerdict{goldenOptionViolates, 0.9},
		goldenVerdict{goldenOptionComplies, 0.95},
	)
	call := llm.ToolCall{ID: "t1", Name: "roll_dice",
		Args: json.RawMessage(`{"expr":"1d6","reason":"omen"}`)}
	client := llm.NewScripted(
		llm.ScriptedTurn{Response: llm.ChatResponse{Message: llm.Message{
			Role:      llm.RoleAssistant,
			Content:   "You steady your nerve.",
			ToolCalls: []llm.ToolCall{call},
		}}},
		scriptedText("The relic glimmers in the dark."),
	)
	s := goldenSession(t, client, stub)

	if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 6)
	wantKinds := []tui.EventKind{
		tui.EventNarrative, // echo
		tui.EventThinking,
		tui.EventNarrative, // the reply that carries the call
		tui.EventToolAnnotation,
		tui.EventDice,
		tui.EventConfirmRegen,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	if err := s.Command("confirm", "n"); err != nil {
		t.Fatalf("Command confirm: %v", err)
	}
	// n keeps the reply and its calls: the loop re-invokes the model,
	// whose closing prose is judged again (this time it complies).
	if ev := nextEvents(t, s, 3); ev[0].Kind != tui.EventThinking ||
		ev[1].Kind != tui.EventNarrative || ev[2].Kind != tui.EventTurnEnd {
		t.Fatalf("events after n = %v, want thinking, prose, turn end", kindsOf(ev))
	}
	// History: the original reply stands with its tool call, followed by
	// the result the dispatch produced, then the closing prose.
	if len(s.history) != 4 {
		t.Fatalf("history = %d messages, want the exchange (4): %+v", len(s.history), s.history)
	}
	if a := s.history[1]; a.Role != llm.RoleAssistant ||
		a.Content != "You steady your nerve." || len(a.ToolCalls) != 1 || a.ToolCalls[0].ID != "t1" {
		t.Errorf("assistant message = %+v, want the original reply with its call", a)
	}
	if s.history[2].Role != llm.RoleTool || s.history[2].ToolCallID != "t1" {
		t.Errorf("third message = %+v, want the t1 tool result", s.history[2])
	}
}

// TestGoldenCheckYesRewritesOffscreen pins the y-loop end to end: the
// original reply streams with its tool call, dispatch runs, the check
// prompts, y re-asks the model off-screen with history + the original
// reply + the exact nudge, the rewrite's own tool calls are stripped
// (never dispatched, never in history), the accepted rewrite swaps into
// the transcript as one Replace event with a new MsgSeq, the original's
// chat lines are tombstoned, the note is left behind, and the turn ends
// with the final reply in history.
func TestGoldenCheckYesRewritesOffscreen(t *testing.T) {
	const (
		originalText = "You force the door and step inside."
		rewriteText  = "The door yields to your push."
		finalText    = "Dust hangs in the lamplight."
		nudge        = "Rewrite the Game Master reply shown above. Rewrite ONLY the narration: never decide the player character's actions for the player — choosing or narrating what they do — because the player keeps every choice. Emit no tool calls."
	)
	stub := newDecideSeqStub(t,
		goldenVerdict{goldenOptionViolates, 0.9},
		goldenVerdict{goldenOptionComplies, 0.95}, // the rewrite complies; repeats for the closing reply
	)
	call := llm.ToolCall{ID: "t1", Name: "roll_dice",
		Args: json.RawMessage(`{"expr":"1d6","reason":"omen"}`)}
	bogus := llm.ToolCall{ID: "t2", Name: "end_session_recap",
		Args: json.RawMessage(`{"summary":"never runs"}`)}
	client := llm.NewScripted(
		llm.ScriptedTurn{Response: llm.ChatResponse{Message: llm.Message{
			Role: llm.RoleAssistant, Content: originalText, ToolCalls: []llm.ToolCall{call},
		}}},
		// The rewrite answers in prose AND asks for a tool: the call is
		// stripped — the original's already ran.
		llm.ScriptedTurn{Response: llm.ChatResponse{Message: llm.Message{
			Role: llm.RoleAssistant, Content: rewriteText, ToolCalls: []llm.ToolCall{bogus},
		}}},
		scriptedText(finalText),
	)
	s := goldenSession(t, client, stub)

	if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 6)
	wantKinds := []tui.EventKind{
		tui.EventNarrative, // echo
		tui.EventThinking,
		tui.EventNarrative, // the original reply (MsgSeq 1)
		tui.EventToolAnnotation,
		tui.EventDice,
		tui.EventConfirmRegen,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	// y re-asks the model off-screen; everything after the prompt waits
	// for the answer.
	if err := s.Command("confirm", "y"); err != nil {
		t.Fatalf("Command confirm: %v", err)
	}
	events = append(events, nextEvents(t, s, 6)...)
	wantKinds = []tui.EventKind{
		tui.EventThinking,  // the off-screen rewrite
		tui.EventNarrative, // the replacement (Replace, MsgSeq 2)
		tui.EventSystem,    // the note
		tui.EventThinking,  // the closing reply
		tui.EventNarrative, // the closing reply (MsgSeq 3)
		tui.EventTurnEnd,
	}
	for i, want := range wantKinds {
		if events[6+i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", 6+i, events[6+i].Kind, want, kindsOf(events))
		}
	}
	if ev := events[2]; ev.MsgSeq != 1 || ev.Markdown != originalText {
		t.Errorf("original = MsgSeq %d %q, want the streamed message 1", ev.MsgSeq, ev.Markdown)
	}
	if ev := events[7]; !ev.Replace || ev.ReplaceSeq != 1 || ev.MsgSeq != 2 ||
		ev.Markdown != rewriteText || ev.Player {
		t.Errorf("replacement = %+v, want Replace of MsgSeq 1 as message 2 with the full rewrite", ev)
	}
	if ev := events[8]; ev.Markdown != goldenRuleNote {
		t.Errorf("note = %q, want %q", ev.Markdown, goldenRuleNote)
	}
	if ev := events[10]; ev.MsgSeq != 3 || ev.Markdown != finalText {
		t.Errorf("closing reply = MsgSeq %d %q, want message 3", ev.MsgSeq, ev.Markdown)
	}
	// No annotation ever filed the rewrite's stripped call.
	annotations := 0
	for _, ev := range events {
		if ev.Kind == tui.EventToolAnnotation {
			annotations++
			if ev.Annotation.ToolName != "roll_dice" {
				t.Errorf("annotation for %q, want only the original roll_dice", ev.Annotation.ToolName)
			}
		}
	}
	if annotations != 1 {
		t.Errorf("annotations = %d, want 1 (the rewrite's call must never dispatch)", annotations)
	}

	// The rewrite request: history + the original reply (text AND the
	// tool call it already ran) + the exact nudge — and no tools to call.
	reqs := client.Requests()
	if len(reqs) != 3 {
		t.Fatalf("model calls = %d, want 3 (turn, rewrite, closing reply)", len(reqs))
	}
	rewrite := reqs[1].Messages
	if len(rewrite) != 4 { // system, player, original assistant, nudge
		t.Fatalf("rewrite request = %d messages, want 4: %+v", len(rewrite), rewrite)
	}
	if a := rewrite[2]; a.Role != llm.RoleAssistant || a.Content != originalText ||
		len(a.ToolCalls) != 1 || a.ToolCalls[0].ID != "t1" {
		t.Errorf("rewrite original = %+v, want the original reply with its original call", a)
	}
	if u := rewrite[3]; u.Role != llm.RoleUser || u.Content != nudge {
		t.Errorf("rewrite nudge = %+v, want the exact corrective nudge", u)
	}
	if len(rewrite[0].Content) == 0 || rewrite[0].Role != llm.RoleSystem {
		t.Errorf("rewrite request = %+v, want the system prompt first", rewrite[0])
	}
	if len(reqs[1].Tools) != 0 {
		t.Errorf("rewrite advertised %d tools, want none", len(reqs[1].Tools))
	}

	// History: final text with the ORIGINAL call, the tool result, the
	// closing reply — the discarded drafts never entered.
	if len(s.history) != 4 {
		t.Fatalf("history = %d messages, want 4: %+v", len(s.history), s.history)
	}
	if a := s.history[1]; a.Content != rewriteText || len(a.ToolCalls) != 1 ||
		a.ToolCalls[0].ID != "t1" {
		t.Errorf("assistant = %+v, want the rewritten text with the original call", a)
	}
	for _, m := range s.history {
		if m.Content == originalText {
			t.Errorf("history carries the discarded draft: %+v", m)
		}
		if len(m.ToolCalls) > 0 && m.ToolCalls[0].ID == "t2" {
			t.Errorf("history carries the rewrite's stripped call: %+v", m)
		}
	}
	if a := s.history[3]; a.Content != finalText || len(a.ToolCalls) != 0 {
		t.Errorf("closing message = %+v, want the final prose reply", a)
	}

	// The log: tombstone + rewrite line + note, and one audit per check.
	lines := chatLines(t, s)
	var (
		sawOriginal, sawRetract, sawRewrite, sawNote, sawFinal bool
	)
	for _, c := range lines {
		switch {
		case c.Kind == "narrative" && c.MsgSeq == 1 && c.Text == originalText:
			sawOriginal = true
		case c.Kind == state.ChatKindRetract && c.MsgSeq == 1:
			sawRetract = true
		case c.Kind == "narrative" && c.MsgSeq == 2 && c.Text == rewriteText:
			sawRewrite = true
		case c.Kind == state.ChatKindNote && c.MsgSeq == 2 && c.Text == goldenRuleNote:
			sawNote = true
		case c.Kind == "narrative" && c.MsgSeq == 3 && c.Text == finalText:
			sawFinal = true
		}
	}
	if !sawOriginal || !sawRetract || !sawRewrite || !sawNote || !sawFinal {
		t.Errorf("log lines = %+v, want original + retract(1) + rewrite(2) + note(2) + final(3)", lines)
	}
	checks := goldenChecks(t, s)
	wantChecks := []goldenCheckMove{
		{MsgSeq: 1, Verdict: goldenOptionViolates, Confidence: 0.9, Decision: goldenDecisionRegen, Attempt: 1},
		{MsgSeq: 2, Verdict: goldenOptionComplies, Confidence: 0.95, Decision: goldenDecisionPass, Attempt: 2},
		{MsgSeq: 3, Verdict: goldenOptionComplies, Confidence: 0.95, Decision: goldenDecisionPass, Attempt: 1},
	}
	if len(checks) != len(wantChecks) {
		t.Fatalf("golden checks = %+v, want %+v", checks, wantChecks)
	}
	for i, want := range wantChecks {
		if checks[i] != want {
			t.Errorf("check %d = %+v, want %+v", i, checks[i], want)
		}
	}
}

// TestGoldenCheckPersistentViolationPromptsAgain pins the loop: a rewrite
// that still violates draws the same prompt a second time; answering n
// keeps the rewrite on screen (it is what is displayed), recorded with
// the note the rewrite earned, and the original stays out of history.
func TestGoldenCheckPersistentViolationPromptsAgain(t *testing.T) {
	const (
		originalText = "You force the door and step inside."
		rewriteText  = "The door yields to your push."
	)
	stub := newDecideSeqStub(t,
		goldenVerdict{goldenOptionViolates, 0.9},
		goldenVerdict{goldenOptionViolates, 0.85},
	)
	client := llm.NewScripted(scriptedText(originalText), scriptedText(rewriteText))
	s := goldenSession(t, client, stub)

	if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 4)
	wantKinds := []tui.EventKind{
		tui.EventNarrative, // echo
		tui.EventThinking,
		tui.EventNarrative, // the original
		tui.EventConfirmRegen,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	// y rewrites off-screen; the re-check still violates, so the same
	// prompt comes again.
	if err := s.Command("confirm", "y"); err != nil {
		t.Fatalf("Command confirm: %v", err)
	}
	events = append(events, nextEvents(t, s, 3)...)
	wantKinds = []tui.EventKind{
		tui.EventThinking,     // the off-screen rewrite
		tui.EventNarrative,    // the replacement
		tui.EventConfirmRegen, // the same prompt again
	}
	for i, want := range wantKinds {
		if events[4+i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", 4+i, events[4+i].Kind, want, kindsOf(events))
		}
	}
	if events[3].Markdown != events[6].Markdown {
		t.Errorf("second prompt = %q, want the same question as the first %q",
			events[6].Markdown, events[3].Markdown)
	}
	if err := s.Command("confirm", "n"); err != nil {
		t.Fatalf("Command confirm: %v", err)
	}
	// n keeps the displayed rewrite: the note lands, then the turn ends.
	if ev := nextEvent(t, s); ev.Kind != tui.EventSystem || ev.Markdown != goldenRuleNote {
		t.Fatalf("event after n = %+v, want the rewrite note", ev)
	}
	if ev := nextEvent(t, s); ev.Kind != tui.EventTurnEnd {
		t.Errorf("event = %v, want the turn end", ev.Kind)
	}
	if len(s.history) != 2 {
		t.Fatalf("history = %d messages, want 2: %+v", len(s.history), s.history)
	}
	if a := s.history[1]; a.Role != llm.RoleAssistant || a.Content != rewriteText {
		t.Errorf("assistant = %+v, want the kept rewrite", a)
	}
	if stub.calls.Load() != 2 {
		t.Errorf("decide calls = %d, want 2 (one per attempt)", stub.calls.Load())
	}
	checks := goldenChecks(t, s)
	wantChecks := []goldenCheckMove{
		{MsgSeq: 1, Verdict: goldenOptionViolates, Confidence: 0.9, Decision: goldenDecisionRegen, Attempt: 1},
		{MsgSeq: 2, Verdict: goldenOptionViolates, Confidence: 0.85, Decision: goldenDecisionKeep, Attempt: 2},
	}
	if len(checks) != len(wantChecks) {
		t.Fatalf("golden checks = %+v, want %+v", checks, wantChecks)
	}
	for i, want := range wantChecks {
		if checks[i] != want {
			t.Errorf("check %d = %+v, want %+v", i, checks[i], want)
		}
	}
}

// TestGoldenCheckNoKeepsOriginal pins the n path: nothing is rewritten,
// the original message stands in history exactly as streamed, no note and
// no tombstone are written — but the verdict and the decision are still
// audited.
func TestGoldenCheckNoKeepsOriginal(t *testing.T) {
	const originalText = "You force the door and step inside."
	stub := newDecideSeqStub(t, goldenVerdict{goldenOptionViolates, 0.93})
	client := llm.NewScripted(scriptedText(originalText))
	s := goldenSession(t, client, stub)

	if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 4)
	wantKinds := []tui.EventKind{
		tui.EventNarrative, // echo
		tui.EventThinking,
		tui.EventNarrative,
		tui.EventConfirmRegen,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	if err := s.Command("confirm", "n"); err != nil {
		t.Fatalf("Command confirm: %v", err)
	}
	if ev := nextEvent(t, s); ev.Kind != tui.EventTurnEnd {
		t.Fatalf("event after n = %v, want the turn end", ev.Kind)
	}
	if len(s.history) != 2 || s.history[1].Content != originalText {
		t.Fatalf("history = %+v, want the original reply as-is", s.history)
	}
	for _, c := range chatLines(t, s) {
		if c.Kind == state.ChatKindNote || c.Kind == state.ChatKindRetract {
			t.Errorf("n must leave no mark, found chat line %+v", c)
		}
	}
	checks := goldenChecks(t, s)
	want := goldenCheckMove{MsgSeq: 1, Verdict: goldenOptionViolates,
		Confidence: 0.93, Decision: goldenDecisionKeep, Attempt: 1}
	if len(checks) != 1 || checks[0] != want {
		t.Errorf("golden checks = %+v, want %+v", checks, want)
	}
}

// TestCloseDuringRegenPromptReleases: Close discards a pending
// regeneration prompt — the runner leaves waitRegen through quit, so
// shutdown never waits for an answer that can no longer arrive.
func TestCloseDuringRegenPromptReleases(t *testing.T) {
	stub := newDecideSeqStub(t, goldenVerdict{goldenOptionViolates, 0.9})
	s := goldenSession(t, llm.NewScripted(scriptedText("The door stands."), scriptedText("never reached")), stub)
	if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	var sawPrompt bool
	for _, ev := range nextEvents(t, s, 4) {
		if ev.Kind == tui.EventConfirmRegen {
			sawPrompt = true
		}
	}
	if !sawPrompt {
		t.Fatal("the regeneration prompt never arrived")
	}
	done := make(chan error, 1)
	go func() { done <- s.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung behind a pending regeneration prompt")
	}
}

// TestGoldenReplayHidesRetractedLines pins the two-phase replay: a
// tombstone hides exactly the chat lines of its MsgSeq (the originals),
// the rewrite and its note play back — the note as a system event — while
// legacy MsgSeq-0 lines replay unchanged and golden-rule audit entries
// never replay at all.
func TestGoldenReplayHidesRetractedLines(t *testing.T) {
	at := time.Date(2026, time.November, 15, 10, 0, 0, 0, time.UTC)
	entries := []state.LogEntry{
		{Kind: "chat", At: at, Chat: &state.ChatLine{Channel: "IC", Player: true, Kind: "narrative", Text: "I knock twice."}},
		{Kind: "chat", At: at, Chat: &state.ChatLine{Channel: "IC", Kind: "narrative", MsgSeq: 1, Text: "You force the door."}},
		{Kind: "chat", At: at, Chat: &state.ChatLine{Channel: "IC", Kind: state.ChatKindRetract, MsgSeq: 1}},
		{Kind: "chat", At: at, Chat: &state.ChatLine{Channel: "IC", Kind: "narrative", MsgSeq: 2, Text: "The door yields."}},
		{Kind: "chat", At: at, Chat: &state.ChatLine{Channel: "IC", Kind: state.ChatKindNote, MsgSeq: 2, Text: goldenRuleNote}},
		{Kind: "chat", At: at, Chat: &state.ChatLine{Channel: "IC", Kind: "narrative", Text: "A legacy line."}}, // MsgSeq 0
		{Kind: state.GoldenRuleCheckKind, At: at, Move: json.RawMessage(`{"msg_seq":1,"verdict":"violates"}`)},
	}
	events := replayEvents(entries, false)

	var texts []string
	for _, ev := range events {
		texts = append(texts, ev.Markdown)
	}
	joined := strings.Join(texts, "\n")
	for _, want := range []string{"I knock twice.", "The door yields.", goldenRuleNote, "A legacy line."} {
		if !strings.Contains(joined, want) {
			t.Errorf("replay missing %q: %q", want, joined)
		}
	}
	if strings.Contains(joined, "You force the door.") {
		t.Errorf("the retracted original must not replay: %q", joined)
	}
	var sawNote bool
	for _, ev := range events {
		switch ev.Markdown {
		case goldenRuleNote:
			sawNote = true
			if ev.Kind != tui.EventSystem || !ev.Replay || ev.MsgSeq != 2 {
				t.Errorf("note event = %+v, want a replayed system event of MsgSeq 2", ev)
			}
		case "The door yields.":
			if ev.Kind != tui.EventNarrative || ev.MsgSeq != 2 || !ev.Replay {
				t.Errorf("rewrite event = %+v, want a replayed narrative of MsgSeq 2", ev)
			}
		}
	}
	if !sawNote {
		t.Error("no note event replayed")
	}
	for _, ev := range events {
		if ev.Kind == tui.EventNarrative && ev.Markdown == "A legacy line." && ev.MsgSeq != 0 {
			t.Errorf("legacy line MsgSeq = %d, want 0 (unaffected by the sequence)", ev.MsgSeq)
		}
	}
}

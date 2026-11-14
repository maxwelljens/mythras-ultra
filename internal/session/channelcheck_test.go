// SPDX-License-Identifier: EUPL-1.2

package session

// Offline tests for the channel check (docs/design/session-loop.md
// "Channel check"): the decision model gates a player message before its
// echo — confidence and length boundaries, both exact prompt texts — "y"
// continues the message through the normal turn, "n" discards it without a
// trace, and every way the check can be off or broken passes silently
// without a single HTTP call. The decide side is an httptest /v1/systemone
// stub, so nothing here touches a network.

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
	"github.com/mythras-ultra/myul/internal/tui"
)

// decideStub is an offline decision model: an httptest /v1/systemone server
// answering every question with one fixed choice and confidence, counting
// its calls and remembering the last request body it saw.
type decideStub struct {
	client   *decide.Client
	calls    atomic.Int64
	mu       sync.Mutex
	lastBody string
}

// newDecideStub starts a stub answering with choice at confidence, wired
// into an enabled decide.Client. It fails the test on any request that is
// not POST /v1/systemone, and on a question that does not offer choice.
func newDecideStub(t *testing.T, choice string, confidence float64) *decideStub {
	t.Helper()
	stub := &decideStub{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.calls.Add(1)
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
		answers := make(map[string]any, len(req.Questions))
		for label, q := range req.Questions {
			if _, ok := q.Criteria[choice]; !ok {
				t.Errorf("question %q does not offer %q", label, choice)
			}
			answers[label] = map[string]any{
				"type":          "choice",
				"choice":        choice,
				"probabilities": map[string]float64{choice: confidence},
				"confidence":    confidence,
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
func (s *decideStub) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastBody
}

// gatedSession builds a session whose channel check runs against stub.
func gatedSession(t *testing.T, client llm.Client, stub *decideStub) *Session {
	t.Helper()
	return newTestSession(t, client, func(cfg *Config) {
		cfg.ChannelCheck = true
		cfg.Decide = stub.client
	})
}

// longMessage is a channel-check-sized player message (≥32 bytes).
const longMessage = "I slide the forged writ across the table and hold my breath."

// TestChannelCheckGatesOnConfidenceAndLength pins the three gate
// conditions: the verdict must differ from the current channel, reach
// channelCheckMinConfidence (0.70 —0.69 passes) and the text must be at
// least channelCheckMinLen (32 — 31 bytes never even asks the decision
// model). Everything that passes does so silently, with the echo running
// as if no check existed.
func TestChannelCheckGatesOnConfidenceAndLength(t *testing.T) {
	tests := []struct {
		name       string
		choice     string
		confidence float64
		text       string
		wantCalls  int64
		wantHold   bool
	}{
		{name: "mismatch at 0.70 holds", choice: "out-of-character",
			confidence: 0.70, text: longMessage, wantCalls: 1, wantHold: true},
		{name: "mismatch at 0.69 passes silently", choice: "out-of-character",
			confidence: 0.69, text: longMessage, wantCalls: 1, wantHold: false},
		{name: "31 bytes never asks", choice: "out-of-character",
			confidence: 0.99, text: strings.Repeat("x", 31), wantCalls: 0, wantHold: false},
		{name: "32 bytes asks", choice: "out-of-character",
			confidence: 0.99, text: strings.Repeat("x", 32), wantCalls: 1, wantHold: true},
		{name: "verdict matching the channel passes", choice: "in-character",
			confidence: 0.99, text: longMessage, wantCalls: 1, wantHold: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newDecideStub(t, tt.choice, tt.confidence)
			s := gatedSession(t, llm.NewScripted(scriptedText("The GM answers.")), stub)
			if err := s.Send(t.Context(), tui.ChannelIC, tt.text); err != nil {
				t.Fatalf("Send: %v", err)
			}
			ev := nextEvent(t, s)
			switch {
			case tt.wantHold && ev.Kind != tui.EventConfirm:
				t.Fatalf("first event = %+v, want a hold (EventConfirm)", ev)
			case !tt.wantHold && (ev.Kind != tui.EventNarrative || ev.Markdown != tt.text):
				t.Fatalf("first event = %+v, want the verbatim echo with no hold", ev)
			}
			if got := stub.calls.Load(); got != tt.wantCalls {
				t.Errorf("decide calls = %d, want %d", got, tt.wantCalls)
			}
			if tt.wantHold {
				// The message under judgement reaches the model: the
				// pinned mode sentence plus the text itself.
				if body := stub.body(); !strings.Contains(body, channelCheckMode) ||
					!strings.Contains(body, tt.text) {
					t.Errorf("decide state should carry the mode sentence and the message, got %.200q", body)
				}
				if err := s.Command("confirm", "y"); err != nil {
					t.Fatalf("Command confirm: %v", err)
				}
			}
		})
	}
}

// TestChannelCheckPromptsExact pins the two player-facing questions, the
// held text they carry and the channel they arrive on.
func TestChannelCheckPromptsExact(t *testing.T) {
	const held = "Could we pause here — I need to check my sheet before deciding."
	tests := []struct {
		name   string
		ch     tui.Channel
		choice string
		want   string
	}{
		{name: "IC mode", ch: tui.ChannelIC, choice: "out-of-character",
			want: "You are in IC mode, but your response looks like a message for OOC. Is this correct? N/y"},
		{name: "OOC mode", ch: tui.ChannelOOC, choice: "in-character",
			want: "You are in OOC mode, but your response looks like a message for IC. Is this correct? N/y"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newDecideStub(t, tt.choice, 0.9)
			s := gatedSession(t, llm.NewScripted(), stub)
			if err := s.Send(t.Context(), tt.ch, held); err != nil {
				t.Fatalf("Send: %v", err)
			}
			ev := nextEvent(t, s)
			if ev.Kind != tui.EventConfirm {
				t.Fatalf("first event = %+v, want EventConfirm", ev)
			}
			if ev.Markdown != tt.want {
				t.Errorf("question = %q, want %q", ev.Markdown, tt.want)
			}
			if ev.ConfirmText != held {
				t.Errorf("ConfirmText = %q, want the held message", ev.ConfirmText)
			}
			if ev.Channel != tt.ch {
				t.Errorf("channel = %v, want %v", ev.Channel, tt.ch)
			}
			if ev.Player {
				t.Error("a hold must not be marked as the player's echo")
			}
		})
	}
}

// TestChannelCheckConfirmYesContinues pins the "y" path: the hold arrives
// before any echo, and answering y continues the message exactly as a Send
// that was never held — echo, thinking, narration, turn end, in that
// order, with the text in the model's conversation.
func TestChannelCheckConfirmYesContinues(t *testing.T) {
	stub := newDecideStub(t, "out-of-character", 0.9)
	client := llm.NewScripted(scriptedText("The seal breaks under your thumb."))
	s := gatedSession(t, client, stub)

	if err := s.Send(t.Context(), tui.ChannelIC, longMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if ev := nextEvent(t, s); ev.Kind != tui.EventConfirm {
		t.Fatalf("first event = %+v, want the hold before any echo", ev)
	}
	if err := s.Command("confirm", "y"); err != nil {
		t.Fatalf("Command confirm: %v", err)
	}
	events := nextEvents(t, s, 4)
	wantKinds := []tui.EventKind{
		tui.EventNarrative, // the verbatim echo
		tui.EventThinking,
		tui.EventNarrative, // scripted narration
		tui.EventTurnEnd,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	if ev := events[0]; ev.Markdown != longMessage || !ev.Player || ev.Channel != tui.ChannelIC {
		t.Errorf("echo = %+v, want the verbatim player message on IC", ev)
	}
	if ev := events[2]; ev.Markdown != "The seal breaks under your thumb." {
		t.Errorf("narration = %q, want the scripted reply", ev.Markdown)
	}
	reqs := client.Requests()
	if len(reqs) != 1 {
		t.Fatalf("model calls = %d, want 1", len(reqs))
	}
	var sawUser bool
	for _, m := range reqs[0].Messages {
		if m.Role == llm.RoleUser && m.Content == longMessage {
			sawUser = true
		}
	}
	if !sawUser {
		t.Errorf("model conversation should carry the confirmed message, got %+v", reqs[0].Messages)
	}
}

// TestChannelCheckConfirmNoDiscards pins the "n" path: nothing is echoed,
// nothing reaches the model, and no history entry appears — the next
// message the runner processes is the one sent after the answer, which
// proves the hold really resolved first.
func TestChannelCheckConfirmNoDiscards(t *testing.T) {
	stub := newDecideStub(t, "out-of-character", 0.9)
	client := llm.NewScripted(scriptedText("You wait a moment longer."))
	s := gatedSession(t, client, stub)

	if err := s.Send(t.Context(), tui.ChannelIC, longMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if ev := nextEvent(t, s); ev.Kind != tui.EventConfirm {
		t.Fatalf("first event = %+v, want the hold", ev)
	}
	if err := s.Command("confirm", "n"); err != nil {
		t.Fatalf("Command confirm: %v", err)
	}
	// A short follow-up (under the length threshold, so it is not gated
	// itself) proves the runner moved on — and it, not the held message,
	// is what the session processes next.
	const followUp = "ok"
	if err := s.Send(t.Context(), tui.ChannelIC, followUp); err != nil {
		t.Fatalf("Send follow-up: %v", err)
	}
	events := nextEvents(t, s, 4)
	wantKinds := []tui.EventKind{
		tui.EventNarrative, tui.EventThinking, tui.EventNarrative, tui.EventTurnEnd,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	if ev := events[0]; ev.Markdown != followUp {
		t.Errorf("echo = %q, want the follow-up — the held message must leave no trace", ev.Markdown)
	}
	reqs := client.Requests()
	if len(reqs) != 1 {
		t.Fatalf("model calls = %d, want 1 (the discarded message never reaches the model)", len(reqs))
	}
	for _, m := range reqs[0].Messages {
		if m.Role == llm.RoleUser {
			if m.Content != followUp {
				t.Errorf("history user message = %q, want only %q", m.Content, followUp)
			}
			if strings.Contains(m.Content, longMessage) {
				t.Error("the discarded message must not be in history")
			}
		}
	}
}

// TestChannelCheckQueuedInputsWaitForAnswer pins the hold's queue
// behaviour: messages sent while a confirm is pending keep queueing — no
// event reaches the TUI until the answer arrives — and are then processed
// in arrival order behind the confirmed message.
func TestChannelCheckQueuedInputsWaitForAnswer(t *testing.T) {
	stub := newDecideStub(t, "out-of-character", 0.9)
	client := llm.NewScripted(
		scriptedText("The writ is genuine."),
		scriptedText("You wait."),
	)
	s := gatedSession(t, client, stub)

	if err := s.Send(t.Context(), tui.ChannelIC, longMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if ev := nextEvent(t, s); ev.Kind != tui.EventConfirm {
		t.Fatalf("first event = %+v, want the hold", ev)
	}
	if err := s.Send(t.Context(), tui.ChannelIC, "ok"); err != nil {
		t.Fatalf("Send queued: %v", err)
	}
	select {
	case ev := <-s.Events():
		t.Fatalf("event before the answer: %+v (queued input was processed early)", ev)
	case <-time.After(150 * time.Millisecond):
	}
	if err := s.Command("confirm", "y"); err != nil {
		t.Fatalf("Command confirm: %v", err)
	}
	events := nextEvents(t, s, 8)
	wantKinds := []tui.EventKind{
		tui.EventNarrative, tui.EventThinking, tui.EventNarrative, tui.EventTurnEnd,
		tui.EventNarrative, tui.EventThinking, tui.EventNarrative, tui.EventTurnEnd,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
		}
	}
	if events[0].Markdown != longMessage {
		t.Errorf("first echo = %q, want the held message first", events[0].Markdown)
	}
	if events[4].Markdown != "ok" {
		t.Errorf("second echo = %q, want the queued follow-up", events[4].Markdown)
	}
}

// TestChannelCheckSilentWhenDecideIsOff pins the never-blocks rule: a nil
// client, a disabled client and an unreachable decision service all pass
// the message through untouched — echo first, no hold, no error line.
func TestChannelCheckSilentWhenDecideIsOff(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	dead.Close() // nothing listens: every call fails with ErrUnavailable

	tests := []struct {
		name   string
		decide *decide.Client
	}{
		{name: "nil client", decide: nil},
		{name: "disabled client", decide: decide.New(decide.Config{Enabled: false, URL: dead.URL})},
		{name: "unreachable service", decide: decide.New(decide.Config{Enabled: true, URL: dead.URL})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := llm.NewScripted(scriptedText("Nothing stops the scene."))
			s := newTestSession(t, client, func(cfg *Config) {
				cfg.ChannelCheck = true
				cfg.Decide = tt.decide
			})
			if err := s.Send(t.Context(), tui.ChannelIC, longMessage); err != nil {
				t.Fatalf("Send: %v", err)
			}
			events := nextEvents(t, s, 4)
			wantKinds := []tui.EventKind{
				tui.EventNarrative, tui.EventThinking, tui.EventNarrative, tui.EventTurnEnd,
			}
			for i, want := range wantKinds {
				if events[i].Kind != want {
					t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
				}
			}
			if events[0].Markdown != longMessage {
				t.Errorf("echo = %q, want the message sent as if unchecked", events[0].Markdown)
			}
		})
	}
}

// TestChannelCheckDisabledMakesNoCalls pins config decision.channel_check =
// false: the runner skips the decide call entirely — the handler fails the
// test if it is ever hit — and the message goes straight to the echo.
func TestChannelCheckDisabledMakesNoCalls(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		t.Errorf("decide called with the channel check off: %s %s", r.Method, r.URL.Path)
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := llm.NewScripted(scriptedText("Straight through."))
	s := newTestSession(t, client, func(cfg *Config) {
		cfg.ChannelCheck = false
		cfg.Decide = decide.New(decide.Config{Enabled: true, URL: srv.URL})
	})
	if err := s.Send(t.Context(), tui.ChannelIC, longMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	events := nextEvents(t, s, 4)
	if events[0].Kind != tui.EventNarrative || events[0].Markdown != longMessage {
		t.Errorf("first event = %+v, want the verbatim echo", events[0])
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("decide calls = %d, want 0 with the check off", got)
	}
}

// TestConfirmWithoutPendingHoldErrors: a confirm that arrives with no
// message waiting is an error line, never a silent no-op.
func TestConfirmWithoutPendingHoldErrors(t *testing.T) {
	s := newTestSession(t, llm.NewScripted())
	if err := s.Command("confirm", "y"); err != nil {
		t.Fatalf("Command: %v", err)
	}
	ev := nextEvent(t, s)
	if ev.Kind != tui.EventError || !strings.Contains(ev.Markdown, "nothing pending") {
		t.Errorf("event = %+v, want a \"nothing pending\" error line", ev)
	}
	// The session stays usable: a later hold still answers normally.
	stub := newDecideStub(t, "out-of-character", 0.9)
	s2 := gatedSession(t, llm.NewScripted(), stub)
	if err := s2.Send(t.Context(), tui.ChannelIC, longMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if ev := nextEvent(t, s2); ev.Kind != tui.EventConfirm {
		t.Errorf("event = %+v, want a hold after the stray answer", ev)
	}
}

// TestCloseWhileConfirmPendingDoesNotHang: Close discards a pending hold —
// the runner leaves waitConfirm through quit, so shutdown never waits for
// an answer that can no longer arrive.
func TestCloseWhileConfirmPendingDoesNotHang(t *testing.T) {
	stub := newDecideStub(t, "out-of-character", 0.9)
	s := gatedSession(t, llm.NewScripted(), stub)
	if err := s.Send(t.Context(), tui.ChannelIC, longMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if ev := nextEvent(t, s); ev.Kind != tui.EventConfirm {
		t.Fatalf("first event = %+v, want the hold", ev)
	}
	done := make(chan error, 1)
	go func() { done <- s.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung behind a pending channel-check hold")
	}
}

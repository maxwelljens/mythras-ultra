// SPDX-License-Identifier: EUPL-1.2

package session

// Offline tests for the decision-model status bracket (docs/design/
// session-loop.md "Channel check" / "Golden rule check"):
// tui.EventDecideStart lands immediately before each Decide HTTP call and
// tui.EventDecideStop immediately after it returns — before any
// EventConfirm, echo or turn event the verdict leads to — around BOTH
// call sites (channelcheck.go and goldencheck.go). The decide side is the
// same httptest /v1/systemone stub as channelcheck_test.go, so nothing
// here touches a network. These tests read the stream RAW (nextEvent
// filters the status events out for the transcript-sequence tests).

import (
	"testing"
	"time"

	"github.com/maxwelljens/mythras-ultra/internal/harness/llm"
	"github.com/maxwelljens/mythras-ultra/internal/tui"
)

// rawEvent waits for the next event exactly as it arrives, the
// decision-model status events included.
func rawEvent(t *testing.T, s *Session) tui.Event {
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

// rawEvents reads exactly n events in arrival order, status events
// included.
func rawEvents(t *testing.T, s *Session, n int) []tui.Event {
	t.Helper()
	events := make([]tui.Event, 0, n)
	for range n {
		events = append(events, rawEvent(t, s))
	}
	return events
}

// indexOfKind returns the index of the first event of kind k, or -1.
func indexOfKind(events []tui.Event, k tui.EventKind) int {
	for i, ev := range events {
		if ev.Kind == k {
			return i
		}
	}
	return -1
}

// countKind returns how many events of kind k the sequence carries.
func countKind(events []tui.Event, k tui.EventKind) int {
	n := 0
	for _, ev := range events {
		if ev.Kind == k {
			n++
		}
	}
	return n
}

// TestChannelCheckDecideBracket pins the channel gate's bracket around
// the decide call: start immediately before it, stop immediately after —
// the stop lands BEFORE the EventConfirm a mismatch raises, and before
// the echo and EventThinking of a clean pass. Exactly one pair wraps the
// one call the stub saw, and the hold itself (question, ConfirmText)
// is unchanged.
func TestChannelCheckDecideBracket(t *testing.T) {
	t.Run("mismatch: stop lands before the hold", func(t *testing.T) {
		stub := newDecideStub(t, "out-of-character", 0.9)
		s := gatedSession(t, llm.NewScripted(scriptedText("The GM answers.")), stub)
		if err := s.Send(t.Context(), tui.ChannelIC, longMessage); err != nil {
			t.Fatalf("Send: %v", err)
		}
		events := rawEvents(t, s, 3)
		wantKinds := []tui.EventKind{tui.EventDecideStart, tui.EventDecideStop, tui.EventConfirm}
		for i, want := range wantKinds {
			if events[i].Kind != want {
				t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
			}
		}
		if ev := events[2]; ev.ConfirmText != longMessage {
			t.Errorf("ConfirmText = %q, want the held message unchanged", ev.ConfirmText)
		}
		if got := countKind(events, tui.EventDecideStart); got != 1 {
			t.Errorf("decide starts = %d, want 1 for one decide call", got)
		}
		if got := stub.calls.Load(); got != 1 {
			t.Fatalf("decide calls = %d, want 1", got)
		}
		// The hold resolves exactly as before: echo, thinking, reply,
		// turn end — no further bracket (the check ran once).
		if err := s.Command("confirm", "y"); err != nil {
			t.Fatalf("Command confirm: %v", err)
		}
		more := rawEvents(t, s, 4)
		wantKinds = []tui.EventKind{tui.EventNarrative, tui.EventThinking,
			tui.EventNarrative, tui.EventTurnEnd}
		for i, want := range wantKinds {
			if more[i].Kind != want {
				t.Fatalf("event %d = %v, want %v (sequence: %v)", i, more[i].Kind, want, kindsOf(more))
			}
		}
		if got := countKind(more, tui.EventDecideStart); got != 0 {
			t.Errorf("decide starts after the hold = %d, want 0", got)
		}
	})

	t.Run("clean: stop lands before the echo and EventThinking", func(t *testing.T) {
		stub := newDecideStub(t, "in-character", 0.9)
		s := gatedSession(t, llm.NewScripted(scriptedText("The seal breaks.")), stub)
		if err := s.Send(t.Context(), tui.ChannelIC, longMessage); err != nil {
			t.Fatalf("Send: %v", err)
		}
		events := rawEvents(t, s, 6)
		wantKinds := []tui.EventKind{tui.EventDecideStart, tui.EventDecideStop,
			tui.EventNarrative, tui.EventThinking, tui.EventNarrative, tui.EventTurnEnd}
		for i, want := range wantKinds {
			if events[i].Kind != want {
				t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
			}
		}
		stop := indexOfKind(events, tui.EventDecideStop)
		thinking := indexOfKind(events, tui.EventThinking)
		if stop < 0 || thinking < 0 || stop > thinking {
			t.Errorf("stop at %d must precede EventThinking at %d", stop, thinking)
		}
		if ev := events[2]; ev.Markdown != longMessage || !ev.Player {
			t.Errorf("echo = %+v, want the verbatim player message, unchanged by the bracket", ev)
		}
		if got := countKind(events, tui.EventDecideStart); got != 1 {
			t.Errorf("decide starts = %d, want 1 for one decide call", got)
		}
	})
}

// TestGoldenCheckDecideBracket pins the golden-rule check's bracket:
// start/stop around its Decide call, the stop landing before the
// EventConfirmRegen a violation raises and before the EventTurnEnd of a
// clean pass (the reply streams as writing first — the TUI shows
// writing → deciding → confirm/turn end).
func TestGoldenCheckDecideBracket(t *testing.T) {
	t.Run("violation: stop lands before the regen prompt", func(t *testing.T) {
		stub := newDecideSeqStub(t, goldenVerdict{goldenOptionViolates, 0.9})
		s := goldenSession(t, llm.NewScripted(scriptedText("You force the door.")), stub)
		if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
			t.Fatalf("Send: %v", err)
		}
		events := rawEvents(t, s, 6)
		wantKinds := []tui.EventKind{tui.EventNarrative, tui.EventThinking,
			tui.EventNarrative, tui.EventDecideStart, tui.EventDecideStop, tui.EventConfirmRegen}
		for i, want := range wantKinds {
			if events[i].Kind != want {
				t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
			}
		}
		if ev := events[5]; ev.Markdown != goldenRegenPrompt || ev.ConfirmText != "" {
			t.Errorf("regen prompt = %+v, want the exact Y/n question with no draft", ev)
		}
		if got := countKind(events, tui.EventDecideStart); got != 1 {
			t.Errorf("decide starts = %d, want 1 for one decide call", got)
		}
		if err := s.Command("confirm", "n"); err != nil {
			t.Fatalf("Command confirm: %v", err)
		}
		if ev := rawEvent(t, s); ev.Kind != tui.EventTurnEnd {
			t.Errorf("event after n = %v, want the turn end", ev.Kind)
		}
	})

	t.Run("clean pass: stop lands before the turn end", func(t *testing.T) {
		stub := newDecideSeqStub(t, goldenVerdict{goldenOptionComplies, 0.95})
		s := goldenSession(t, llm.NewScripted(scriptedText("The lamp gutters.")), stub)
		if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
			t.Fatalf("Send: %v", err)
		}
		events := rawEvents(t, s, 6)
		wantKinds := []tui.EventKind{tui.EventNarrative, tui.EventThinking,
			tui.EventNarrative, tui.EventDecideStart, tui.EventDecideStop, tui.EventTurnEnd}
		for i, want := range wantKinds {
			if events[i].Kind != want {
				t.Fatalf("event %d = %v, want %v (sequence: %v)", i, events[i].Kind, want, kindsOf(events))
			}
		}
		if got := countKind(events, tui.EventDecideStart); got != 1 {
			t.Errorf("decide starts = %d, want 1 for one decide call", got)
		}
		if got := stub.calls.Load(); got != 1 {
			t.Errorf("decide calls = %d, want 1", got)
		}
	})
}

// TestGoldenCheckDecideBracketAroundRewrite pins the loop's brackets end
// to end: the first check brackets into the regen prompt, y re-asks the
// model OFF-SCREEN (EventThinking — the status line reads "Thinking...",
// not "Deciding..."), the rewrite is swapped in, the re-check brackets
// again before the note, and the bracket count equals the number of
// decide calls the stub served.
func TestGoldenCheckDecideBracketAroundRewrite(t *testing.T) {
	stub := newDecideSeqStub(t,
		goldenVerdict{goldenOptionViolates, 0.9},
		goldenVerdict{goldenOptionComplies, 0.95},
	)
	s := goldenSession(t, llm.NewScripted(
		scriptedText("You force the door and step inside."),
		scriptedText("The door yields to your push."),
	), stub)
	if err := s.Send(t.Context(), tui.ChannelIC, goldenMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	first := rawEvents(t, s, 6)
	wantKinds := []tui.EventKind{tui.EventNarrative, tui.EventThinking,
		tui.EventNarrative, tui.EventDecideStart, tui.EventDecideStop, tui.EventConfirmRegen}
	for i, want := range wantKinds {
		if first[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, first[i].Kind, want, kindsOf(first))
		}
	}
	if err := s.Command("confirm", "y"); err != nil {
		t.Fatalf("Command confirm: %v", err)
	}
	// The rewrite runs off-screen under EventThinking, the replacement
	// files, the re-check brackets the second decide call, then the note
	// and the turn end close the sequence (the original carried no tool
	// calls, so no closing model call follows the rewrite).
	second := rawEvents(t, s, 6)
	wantKinds = []tui.EventKind{
		tui.EventThinking,    // the off-screen rewrite
		tui.EventNarrative,   // the replacement (Replace)
		tui.EventDecideStart, // the second decide call
		tui.EventDecideStop,  // … stopped before the note
		tui.EventSystem,      // the golden-rule note
		tui.EventTurnEnd,
	}
	for i, want := range wantKinds {
		if second[i].Kind != want {
			t.Fatalf("event %d = %v, want %v (sequence: %v)", i, second[i].Kind, want, kindsOf(second))
		}
	}
	total := append(append([]tui.Event{}, first...), second...)
	if got := countKind(total, tui.EventDecideStart); got != 2 {
		t.Errorf("decide starts = %d, want 2 — one bracket per decide call", got)
	}
	if got := countKind(total, tui.EventDecideStop); got != 2 {
		t.Errorf("decide stops = %d, want 2 — one bracket per decide call", got)
	}
	if got := stub.calls.Load(); got != 2 {
		t.Errorf("decide calls = %d, want 2", got)
	}
}

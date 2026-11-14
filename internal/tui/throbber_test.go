package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
)

// TestThrobberStateTransitions checks the component itself: the arming
// rules (exactly one tick chain per working state, none when idle) and the
// two visibly distinct frames.
func TestThrobberStateTransitions(t *testing.T) {
	th, err := Get(defaultTheme)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	rob := newThrobber(th)

	if got := rob.view(); got != "" {
		t.Errorf("idle throbber renders %q, want nothing", got)
	}
	if cmd := rob.setState(throbThinking); cmd == nil {
		t.Fatal("idle → thinking must arm the tick chain")
	}
	if cmd := rob.setState(throbThinking); cmd != nil {
		t.Error("staying in thinking must not arm a second chain")
	}
	thinking := plain(rob.view())
	if thinking == "" {
		t.Fatal("the thinking state must render a frame")
	}
	if cmd := rob.setState(throbWriting); cmd == nil {
		t.Error("thinking → writing must arm the fresh model's chain")
	}
	if writing := plain(rob.view()); writing == thinking {
		t.Errorf("thinking (%q) and writing (%q) must render different frames", thinking, writing)
	}
	if cmd := rob.update(spinner.TickMsg{ID: rob.model.ID()}); cmd == nil {
		t.Error("a tick while writing should re-arm the chain")
	}
	if cmd := rob.setState(throbIdle); cmd != nil {
		t.Error("going idle must not arm anything")
	}
	if got := rob.view(); got != "" {
		t.Errorf("idle again renders %q, want nothing", got)
	}
	if cmd := rob.update(spinner.TickMsg{ID: rob.model.ID()}); cmd != nil {
		t.Error("a tick after idle must not re-arm the spinner")
	}
}

// TestThrobberDrivenByEvents drives the throbber through the event
// sequence a scripted turn produces: thinking → writing → thinking on a
// tool round → idle at turn end — with no tick surviving the end.
func TestThrobberDrivenByEvents(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	if got := plain(view(a)); strings.Contains(got, "⠋") || strings.Contains(got, "▉") {
		t.Fatalf("idle header must show no throbber:\n%s", got)
	}

	_, _ = a.Update(eventMsg{ev: Event{Kind: EventThinking}})
	if a.throb.state != throbThinking {
		t.Fatalf("state = %v, want throbThinking after EventThinking", a.throb.state)
	}
	if got := plain(view(a)); !strings.Contains(got, "⠋") {
		t.Errorf("header should show the thinking frame:\n%s", got)
	}

	_, _ = a.Update(eventMsg{ev: Event{Kind: EventNarrative, Channel: ChannelIC, Markdown: "The door swings open."}})
	if a.throb.state != throbWriting {
		t.Fatalf("state = %v, want throbWriting after the narrative delta", a.throb.state)
	}
	if got := plain(view(a)); !strings.Contains(got, "▉") {
		t.Errorf("header should show the writing frame:\n%s", got)
	}

	_, _ = a.Update(eventMsg{ev: Event{Kind: EventToolAnnotation, Channel: ChannelIC,
		Annotation: &Annotation{ToolName: "world:set-flag", Summary: "torch-lit set", OK: true}}})
	if a.throb.state != throbThinking {
		t.Fatalf("state = %v, want throbThinking after the annotation", a.throb.state)
	}

	// While the turn runs, a tick advances the spinner and re-arms it.
	if _, cmd := a.Update(spinner.TickMsg{ID: a.throb.model.ID()}); cmd == nil {
		t.Fatal("a tick while the turn runs should re-arm the spinner")
	}

	_, _ = a.Update(eventMsg{ev: Event{Kind: EventTurnEnd}})
	if a.throb.state != throbIdle {
		t.Fatalf("state = %v, want throbIdle after EventTurnEnd", a.throb.state)
	}
	if got := plain(view(a)); strings.Contains(got, "⠋") || strings.Contains(got, "▉") {
		t.Errorf("the header must drop the throbber at turn end:\n%s", got)
	}
	for i := range 3 {
		if _, cmd := a.Update(spinner.TickMsg{ID: 0}); cmd != nil {
			t.Fatalf("tick %d after EventTurnEnd re-armed the spinner", i+1)
		}
	}
}

// TestThrobberErrorClears: an EventError still lands in the transcript and
// leaves nothing ticking.
func TestThrobberErrorClears(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventThinking}})
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventError, Channel: ChannelIC, Markdown: "turn failed"}})
	if a.throb.state != throbIdle {
		t.Fatalf("state = %v, want throbIdle after EventError", a.throb.state)
	}
	if !strings.Contains(plain(view(a)), "turn failed") {
		t.Errorf("the error must still be filed in the transcript:\n%s", plain(view(a)))
	}
	if _, cmd := a.Update(spinner.TickMsg{ID: 0}); cmd != nil {
		t.Error("a tick after the error must not re-arm the spinner")
	}
}

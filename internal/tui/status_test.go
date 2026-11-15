package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
)

// statusRow returns the status line of the current view: content is
// header, body…, status, input, footer — so the status sits three rows
// from the end, above the input prompt.
func statusRow(a *App) string {
	lines := strings.Split(view(a), "\n")
	if len(lines) < 3 {
		return ""
	}
	return lines[len(lines)-3]
}

// TestStatusLineLabelsAndFrames pins the status line's content per
// throbber state: spinner frame, one space and the exact label
// ("Thinking..." for both LLM-working states, "Deciding..." for the
// decision model), left-aligned on one row — and the indicator lives on
// the status line only, never in the header.
func TestStatusLineLabelsAndFrames(t *testing.T) {
	tests := []struct {
		name  string
		drive func(*App)
		frame string
		label string
	}{
		{name: "thinking", frame: spinner.MiniDot.Frames[0], label: "Thinking...",
			drive: func(a *App) {
				_, _ = a.Update(eventMsg{ev: Event{Kind: EventThinking}})
			}},
		{name: "writing", frame: writeFrames.Frames[0], label: "Thinking...",
			drive: func(a *App) {
				_, _ = a.Update(eventMsg{ev: Event{Kind: EventThinking}})
				_, _ = a.Update(eventMsg{ev: Event{Kind: EventNarrative,
					Channel: ChannelIC, Markdown: "The door swings open."}})
			}},
		{name: "deciding", frame: decideFrames.Frames[0], label: "Deciding...",
			drive: func(a *App) {
				_, _ = a.Update(eventMsg{ev: Event{Kind: EventDecideStart}})
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _ := newTestApp(t, Options{}, 80, 20)
			tt.drive(a)
			if got := a.renderStatus(); strings.Contains(got, "\n") {
				t.Errorf("renderStatus must stay one row, got %q", got)
			}
			want := tt.frame + " " + tt.label
			if got := plain(statusRow(a)); got != want {
				t.Errorf("status row = %q, want exactly %q (labels are pinned)", got, want)
			}
			if header := plain(a.renderHeader()); strings.Contains(header, tt.frame) ||
				strings.Contains(header, tt.label) {
				t.Errorf("header must not carry the progress indicator: %q", header)
			}
		})
	}
}

// TestStatusLineIdleIsEmptyRow pins the always-rendered row: idle shows
// an empty line (so the layout never jumps), the input prompt follows it
// and the footer closes the window — the row count is unchanged.
func TestStatusLineIdleIsEmptyRow(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	if got := a.renderStatus(); got != "" {
		t.Errorf("idle renderStatus = %q, want an empty string", got)
	}
	lines := strings.Split(view(a), "\n")
	if len(lines) != 20 {
		t.Fatalf("view has %d lines, want 20", len(lines))
	}
	if got := lines[len(lines)-3]; got != "" {
		t.Errorf("idle status row = %q, want an empty row", got)
	}
	if got := plain(lines[len(lines)-2]); !strings.Contains(got, "> ") {
		t.Errorf("input row = %q, want the prompt right under the status line", got)
	}
}

// TestStatusLineAboveInputInEveryViewMode pins the composition: the
// status line is outside the View switch, so sheet, palette, help and
// panel overlays only replace the body — the row still sits above the
// input prompt, and the window height holds.
func TestStatusLineAboveInputInEveryViewMode(t *testing.T) {
	modes := []struct {
		name string
		open func(*App)
	}{
		{"transcript", func(a *App) {}},
		{"panel", func(a *App) { a.togglePanel() }},
		{"help", func(a *App) { a.helpOpen = true }},
		{"palette", func(a *App) { _ = a.pal.openWith(PaletteCommands(a.channel)) }},
		{"sheet", func(a *App) { a.openSheet() }},
	}
	for _, tt := range modes {
		t.Run(tt.name, func(t *testing.T) {
			a, _ := newTestApp(t, Options{}, 80, 20)
			_, _ = a.Update(eventMsg{ev: Event{Kind: EventDecideStart}})
			tt.open(a)
			lines := strings.Split(view(a), "\n")
			if len(lines) != 20 {
				t.Fatalf("%s: view has %d lines, want 20", tt.name, len(lines))
			}
			status := plain(statusRow(a))
			if !strings.Contains(status, "Deciding...") {
				t.Errorf("%s: status row = %q, want the deciding line above the input", tt.name, status)
			}
			if input := plain(lines[len(lines)-2]); !strings.Contains(input, "> ") {
				t.Errorf("%s: input row = %q, want the prompt under the status line", tt.name, input)
			}
		})
	}
}

// TestStatusLineFitsNarrowWindow: a status row too long for the window
// is clipped, never wrapped — every line of the view stays within the
// window and the row count holds.
func TestStatusLineFitsNarrowWindow(t *testing.T) {
	states := map[string]Event{
		"deciding": {Kind: EventDecideStart},
		"thinking": {Kind: EventThinking},
		"writing":  {Kind: EventThinking},
	}
	for name, ev := range states {
		t.Run(name, func(t *testing.T) {
			a, _ := newTestApp(t, Options{}, 10, 12)
			_, _ = a.Update(eventMsg{ev: ev})
			if name == "writing" {
				_, _ = a.Update(eventMsg{ev: Event{Kind: EventNarrative,
					Channel: ChannelIC, Markdown: "The door swings open."}})
			}
			lines := strings.Split(view(a), "\n")
			if len(lines) != 12 {
				t.Fatalf("view has %d lines, want 12", len(lines))
			}
			status := statusRow(a)
			if strings.Contains(status, "\n") {
				t.Errorf("the status row wrapped: %q", status)
			}
			if w := lipgloss.Width(status); w > 10 {
				t.Errorf("status row is %d columns, want at most 10: %q", w, plain(status))
			}
			for i, line := range lines {
				if w := lipgloss.Width(line); w > 10 {
					t.Errorf("line %d is %d columns, want at most 10: %q", i, w, plain(line))
				}
			}
		})
	}
}

// TestDecidingStatusTransitions pins the TUI side of the deciding
// bracket: any state (idle included — the channel gate runs before a
// turn) moves to deciding on EventDecideStart; EventThinking takes it
// over (the model wins); the stop returns to idle; a confirm wait shows
// an empty line (the stop has already landed, the prompt is the
// signal); EventTurnEnd is always idle.
func TestDecidingStatusTransitions(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)

	// idle → deciding: the pre-echo channel gate consults the model
	// before any turn exists.
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventDecideStart}})
	if a.throb.state != throbDeciding {
		t.Fatalf("state = %v, want throbDeciding after EventDecideStart from idle", a.throb.state)
	}

	// deciding → thinking: the model call that follows wins.
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventThinking}})
	if a.throb.state != throbThinking {
		t.Fatalf("state = %v, want throbThinking (the model wins over a pending decide)", a.throb.state)
	}

	// deciding → idle on its stop, from a busy state too.
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventDecideStart}})
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventDecideStop}})
	if a.throb.state != throbIdle {
		t.Fatalf("state = %v, want throbIdle after EventDecideStop", a.throb.state)
	}
	if got := plain(statusRow(a)); got != "" {
		t.Errorf("status row after the stop = %q, want an empty line", got)
	}

	// writing → deciding: the golden-rule check judges a streamed reply.
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventThinking}})
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventNarrative, Channel: ChannelIC,
		Markdown: "The door swings open."}})
	if a.throb.state != throbWriting {
		t.Fatalf("state = %v, want throbWriting after the narrative delta", a.throb.state)
	}
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventDecideStart}})
	if a.throb.state != throbDeciding {
		t.Fatalf("state = %v, want throbDeciding over a writing turn", a.throb.state)
	}

	// The stop precedes the regeneration question: the confirm wait
	// shows an EMPTY line — the prompt is the signal to act.
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventDecideStop}})
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventConfirmRegen, Channel: ChannelIC,
		Markdown: "Regenerate Game Master's response? Y/n"}})
	if !a.confirm || a.throb.state != throbIdle {
		t.Fatalf("confirm=%v state=%v, want a pending regen prompt over an idle status line",
			a.confirm, a.throb.state)
	}
	if got := plain(statusRow(a)); got != "" {
		t.Errorf("status row during the confirm wait = %q, want an empty line", got)
	}

	// EventTurnEnd → idle always, whatever state it interrupts.
	a.leaveConfirm(false)
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventDecideStart}})
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventTurnEnd}})
	if a.throb.state != throbIdle {
		t.Fatalf("state = %v, want throbIdle after EventTurnEnd", a.throb.state)
	}
	if got := plain(statusRow(a)); got != "" {
		t.Errorf("status row at turn end = %q, want an empty line", got)
	}
}

// TestHeaderAndFooterCarryNoProgress: the throbber is the only chrome
// progress element — the header keeps its static segments (session,
// channel, model, save/decision placeholders) and the footer its hints
// and readout, with no spinner or label anywhere but the status line.
func TestHeaderAndFooterCarryNoProgress(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	_, _ = a.Update(eventMsg{ev: Event{Kind: EventThinking}})
	got := plain(view(a))
	for _, want := range []string{"Test Session", "[IC]", "test-model", "save –", "decision –"} {
		if !strings.Contains(plain(a.renderHeader()), want) {
			t.Errorf("header should keep %q, header: %q", want, plain(a.renderHeader()))
		}
	}
	if header := plain(a.renderHeader()); strings.Contains(header, "⠋") ||
		strings.Contains(header, "Thinking...") {
		t.Errorf("header carries progress: %q", header)
	}
	if footer := plain(a.renderFooter()); strings.Contains(footer, "⠋") ||
		strings.Contains(footer, "Thinking...") {
		t.Errorf("footer carries progress: %q", footer)
	}
	if !strings.Contains(got, "Thinking...") {
		t.Errorf("the status line should carry the label, view:\n%s", got)
	}
}

package tui

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// rollEvent builds the DiceEvent the theatre tests stage: 3d6+2 = 14, a
// success — a finished engine roll the TUI only replays.
func rollEvent() DiceEvent {
	return DiceEvent{
		Expr: "3d6+2", Dice: []int{4, 6, 2}, Modifier: 2, Total: 14, Outcome: "success",
	}
}

// theatreTestTheme loads a built-in theme for style assertions.
func theatreTestTheme(t *testing.T) Theme {
	t.Helper()
	th, err := Get("mythras")
	if err != nil {
		t.Fatalf("Get(mythras): %v", err)
	}
	return th
}

func TestDiceTheatrePhaseMachine(t *testing.T) {
	th := theatreTestTheme(t)
	d := NewDiceTheatre(th, true)
	d.SetWidth(60)

	if d.Active() {
		t.Fatal("a fresh theatre is idle")
	}
	if v := d.View(); v != "" {
		t.Errorf("idle view = %q, want empty", v)
	}

	if cmd := d.Start(rollEvent()); cmd == nil {
		t.Fatal("animation on: Start must schedule the first tick")
	}
	if d.phase != diceRolling {
		t.Fatalf("phase after Start = %v, want rolling", d.phase)
	}
	got := plain(d.View())
	for _, want := range []string{"⟳", "3d6+2", "rolling"} {
		if !strings.Contains(got, want) {
			t.Errorf("rolling view should mention %q, view:\n%s", want, got)
		}
	}
	if strings.Contains(got, "= 14") {
		t.Errorf("rolling view must not spoil the total, view:\n%s", got)
	}

	// Drive the spin with synthetic ticks; every Update schedules the next.
	for i := 0; i < diceRollingTicks; i++ {
		cmd, done := d.Update(diceTickMsg{})
		if done {
			t.Fatalf("tick %d: roll finished early", i)
		}
		if cmd == nil {
			t.Fatalf("tick %d: rolling must schedule the next tick", i)
		}
	}
	if d.phase != diceSettle {
		t.Fatalf("after %d ticks phase = %v, want settle", diceRollingTicks, d.phase)
	}
	if got = plain(d.View()); !strings.Contains(got, "settling") {
		t.Errorf("settle view should say so, view:\n%s", got)
	}

	// Each settle tick snaps exactly one die; the first snap is visible.
	ev := rollEvent()
	if cmd, done := d.Update(diceSettleMsg{}); done || cmd == nil {
		t.Fatal("first settle tick must snap a die and schedule the next")
	}
	if !strings.Contains(d.View(), th.Styles.Highlight.Render("["+strconv.Itoa(ev.Dice[0])+"]")) {
		t.Errorf("first settle tick should highlight die 0's final face, view:\n%s", plain(d.View()))
	}
	// The remaining ticks snap the rest of the dice; the snap on the last
	// die lifts the banner and starts the result hold.
	for i := 1; i < len(ev.Dice); i++ {
		if cmd, done := d.Update(diceSettleMsg{}); done || cmd == nil {
			t.Fatalf("settle tick %d: want another snap", i)
		}
	}
	if d.phase != diceResult {
		t.Fatalf("phase = %v, want result after the last die snapped", d.phase)
	}
	got = plain(d.View())
	for _, want := range []string{"SUCCESS", "3d6+2", "[4 6 2]", "+2", "= 14"} {
		if !strings.Contains(got, want) {
			t.Errorf("result view should contain %q, view:\n%s", want, got)
		}
	}

	// The hold ends with a finish message: the host flushes and resets.
	cmd, done := d.Update(diceFinishMsg{})
	if !done || cmd != nil {
		t.Errorf("finish: done=%v cmd=%v, want done with no follow-up", done, cmd)
	}
	d.Reset()
	if d.Active() || d.View() != "" {
		t.Error("after Reset the theatre is idle and renders nothing")
	}
}

func TestDiceTheatreIgnoresStaleTicks(t *testing.T) {
	d := NewDiceTheatre(theatreTestTheme(t), true)
	if cmd, done := d.Update(diceTickMsg{}); cmd != nil || done {
		t.Error("an idle theatre must ignore stray ticks")
	}
	d.Start(rollEvent())
	if cmd, done := d.Update(diceFinishMsg{}); cmd != nil || done {
		t.Error("a finish message outside the result phase must be ignored")
	}
}

func TestDiceTheatreAnimationOff(t *testing.T) {
	th := theatreTestTheme(t)
	d := NewDiceTheatre(th, false)
	d.SetWidth(60)

	if cmd := d.Start(rollEvent()); cmd != nil {
		t.Error("animation off: Start must not schedule ticks")
	}
	if d.phase != diceResult {
		t.Fatalf("phase = %v, want result rendered immediately", d.phase)
	}
	got := plain(d.View())
	for _, want := range []string{"SUCCESS", "3d6+2", "[4 6 2]", "+2", "= 14"} {
		if !strings.Contains(got, want) {
			t.Errorf("instant result should contain %q, view:\n%s", want, got)
		}
	}
	// A stray tick after the instant result must not restart anything.
	if cmd, done := d.Update(diceTickMsg{}); cmd != nil || done {
		t.Error("ticks are ignored outside the rolling phase")
	}
}

func TestDiceBannerOutcomeStyling(t *testing.T) {
	th := theatreTestTheme(t)
	tests := []struct {
		outcome string
		label   string
		style   func(Styles) lipgloss.Style
	}{
		{"critical", "✦ CRITICAL ✦", func(s Styles) lipgloss.Style { return s.Critical }},
		{"success", "SUCCESS", func(s Styles) lipgloss.Style { return s.Success }},
		{"failure", "FAILURE", func(s Styles) lipgloss.Style { return s.Failure }},
		{"fumble", "☠ FUMBLE ☠", func(s Styles) lipgloss.Style { return s.Fumble }},
		{"", "", func(s Styles) lipgloss.Style { return s.DiceBanner }},
	}
	for _, tt := range tests {
		t.Run("outcome="+tt.outcome, func(t *testing.T) {
			ev := rollEvent()
			ev.Outcome = tt.outcome
			d := NewDiceTheatre(th, false)
			d.SetWidth(60)
			d.Start(ev)
			got := d.View()
			if tt.label == "" {
				if strings.Contains(got, "✦") || strings.Contains(got, "☠") {
					t.Errorf("a roll without outcome carries no outcome label, view:\n%s", plain(got))
				}
			} else if want := tt.style(th.Styles).Render(tt.label); !strings.Contains(got, want) {
				t.Errorf("outcome %q should render %q in its outcome style,\ngot: %q\nwant: %q",
					tt.outcome, tt.label, got, want)
			}
			if want := th.Styles.DiceBanner.Render(rollSummary(ev)); !strings.Contains(got, want) {
				t.Errorf("the summary line should be rendered in the dice banner style,\ngot: %q\nwant: %q",
					got, want)
			}
			if !strings.Contains(plain(got), "= 14") {
				t.Errorf("every banner shows the total, view:\n%s", plain(got))
			}
		})
	}
}

func TestDiceFacesAreStaggered(t *testing.T) {
	d := NewDiceTheatre(theatreTestTheme(t), true)
	d.Start(rollEvent())
	before := append([]int(nil), d.faces...)
	if _, done := d.Update(diceTickMsg{}); done {
		t.Fatal("roll ended early")
	}
	changed := 0
	for i := range d.faces {
		if d.faces[i] != before[i] {
			changed++
		}
	}
	if changed == len(d.faces) {
		t.Error("every die changed on one tick — faces must be staggered")
	}
	if changed == 0 {
		t.Error("no die changed on the first tick — the spin must be visibly running")
	}
}

func TestDieSides(t *testing.T) {
	tests := []struct {
		expr string
		want int
	}{
		{"3d6+2", 6},
		{"d20", 20},
		{"1D100", 100},
		{"2d6+1d8", 8}, // the largest die in the pool
		{"", 6},
		{"no dice here", 6},
		{"d", 6},
		{"1d", 6},
	}
	for _, tt := range tests {
		if got := dieSides(tt.expr); got != tt.want {
			t.Errorf("dieSides(%q) = %d, want %d", tt.expr, got, tt.want)
		}
	}
}

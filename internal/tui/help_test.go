package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestRenderHelpListsKeysAndCommands(t *testing.T) {
	th, err := Get("mythras")
	if err != nil {
		t.Fatal(err)
	}
	raw := renderHelp(th, 80, 21)
	got := plain(raw)
	if raw == "" {
		t.Fatal("renderHelp produced nothing")
	}
	for _, k := range helpKeys {
		if !strings.Contains(got, "  "+k[0]) {
			t.Errorf("help should list key %q, help:\n%s", k[0], got)
		}
	}
	// The command list wraps across lines (and the box borders sit in the
	// way); compare on border-stripped, collapsed whitespace.
	strip := strings.NewReplacer("│", "", "─", "", "┌", "", "┐", "", "└", "", "┘", "")
	flat := strings.Join(strings.Fields(strip.Replace(got)), " ")
	for _, want := range []string{"ctrl+t", "ctrl+e", "ctrl+m", "ctrl+l",
		"ctrl+p", "Character sheet", "Roll dice", "Save session", "Session recap",
		"Switch to OOC", "Theme: mythras", "Quit", "enter or esc closes"} {
		if !strings.Contains(flat, want) {
			t.Errorf("help should mention %q, help:\n%s", want, got)
		}
	}
	// The overlay occupies exactly its area: no layout spill.
	if lines := strings.Split(raw, "\n"); len(lines) != 21 {
		t.Errorf("help has %d lines, want 21", len(lines))
	}
	for _, line := range strings.Split(raw, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("help line wider than the window (%d cols): %q", w, plain(line))
		}
	}

	// Degenerate sizes must not panic; an unplaced size renders nothing.
	renderHelp(th, 5, 3)
	renderHelp(th, 10, 1)
	if s := renderHelp(th, 0, 0); s != "" {
		t.Errorf("zero size should render nothing, got %q", s)
	}
}

func TestHelpOverlayOpenCloseCycles(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	open := func(t *testing.T, step string) {
		t.Helper()
		paletteChoose(t, a, "help")
		if !a.helpOpen {
			t.Fatalf("%s: help should be open", step)
		}
	}
	closeVia := func(t *testing.T, k tea.KeyPressMsg, step string) {
		t.Helper()
		press(t, a, k)
		if a.helpOpen {
			t.Fatalf("%s: help should be closed", step)
		}
		if strings.Contains(plain(view(a)), "MYTHRAS ULTRA · HELP") {
			t.Fatalf("%s: closed help must not render", step)
		}
	}

	// The palette's Help entry opens the overlay instead of a transcript line.
	open(t, "first open")
	got := plain(view(a))
	for _, want := range []string{"Roll dice", "Theme: mythras", "ctrl+t"} {
		if !strings.Contains(got, want) {
			t.Errorf("open help should mention %q, view:\n%s", want, got)
		}
	}
	if lines := strings.Split(view(a), "\n"); len(lines) != 20 {
		t.Errorf("overlay should keep the window height, got %d lines", len(lines))
	}

	// While open the overlay captures keys: typing reaches the input line,
	// but q never quits.
	cmd := press(t, a, key('q'))
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("q must not quit while the help overlay is up")
		}
	}
	if a.input.Value() != "q" {
		t.Errorf("typing while open should reach the input line, got %q", a.input.Value())
	}
	a.input.Reset()

	closeVia(t, tea.KeyPressMsg{Code: tea.KeyEsc}, "esc")

	// enter dismisses it too; a message typed while reading survives it.
	open(t, "second open")
	a.input.SetValue("I wait and listen.")
	closeVia(t, tea.KeyPressMsg{Code: tea.KeyEnter}, "enter")
	if a.input.Value() != "I wait and listen." {
		t.Errorf("dismissing with a typed message should keep the line, got %q", a.input.Value())
	}
	a.input.Reset()

	// ctrl+p swaps the palette in for help.
	open(t, "third open")
	press(t, a, ctrl('p'))
	if a.helpOpen {
		t.Error("ctrl+p should close help")
	}
	if !a.pal.open {
		t.Error("ctrl+p should open the palette")
	}
	press(t, a, tea.KeyPressMsg{Code: tea.KeyEsc})

	// ctrl+c still quits while the overlay is up.
	open(t, "fourth open")
	if cmd = press(t, a, ctrl('c')); cmd == nil {
		t.Fatal("ctrl+c should still quit from the overlay")
	} else if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c from the overlay should quit")
	}
	closeVia(t, tea.KeyPressMsg{Code: tea.KeyEsc}, "esc after quit check")

	// A message typed while open survives the dismissal for review.
	open(t, "fifth open")
	a.input.SetValue("I wait and listen.")
	closeVia(t, tea.KeyPressMsg{Code: tea.KeyEsc}, "esc keeps input")
	if a.input.Value() != "I wait and listen." {
		t.Errorf("esc must keep the typed text, got %q", a.input.Value())
	}
}

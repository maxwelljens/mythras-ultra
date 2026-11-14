package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// testSheet is a display-ready snapshot (what the session's buildMetadata
// produces) with every section filled, in sorted order.
func testSheet() Sheet {
	return Sheet{
		Name:    "Wren Ashdown",
		Culture: "Civilised",
		Career:  "Hunter",
		Age:     "27",
		Concept: "a hunter who owes a debt she cannot pay",
		Characteristics: []NameValue{
			{Name: "STR", Value: "12"}, {Name: "CON", Value: "10"},
			{Name: "SIZ", Value: "11"}, {Name: "DEX", Value: "13"},
			{Name: "INT", Value: "12"}, {Name: "POW", Value: "15"},
			{Name: "CHA", Value: "9"},
		},
		Derived: []NameValue{
			{Name: "HP", Value: "34/40"},
			{Name: "Damage Modifier", Value: "1d6"},
			{Name: "Power Points", Value: "15"},
			{Name: "Luck", Value: "15"},
			{Name: "Experience Bonus", Value: "2"},
		},
		Skills: []NameValue{
			{Name: "Athletics", Value: "80%"}, {Name: "Dodge", Value: "60%"},
			{Name: "Stealth", Value: "40%"}, {Name: "Track", Value: "20%"},
		},
		Styles:    []NameValue{{Name: "Broadsword", Value: "65%"}},
		Passions:  []NameValue{{Name: "Loyalty: the party", Value: "60%"}},
		Inventory: []string{"Rope ×3", "Torch (equipped: hand)"},
		Locations: []LocationHP{
			{Name: "Head", Current: 7, Max: 7},
			{Name: "Right Arm", Current: 3, Max: 5, Severity: "wound"},
		},
		Conditions: []string{"Poisoned (Right Arm)"},
	}
}

// openTestSheet drives the palette entry (ctrl+p → filter → enter) so the
// overlay opens exactly the way a player opens it.
func openTestSheet(t *testing.T, a *App) {
	t.Helper()
	paletteChoose(t, a, "character sheet")
	if !a.sheetOpen {
		t.Fatal("Character sheet should open the overlay")
	}
}

// TestSheetOverlayRenders pins the overlay's rendering: the palette opens
// it, the header and every section of a prepared session snapshot show,
// the window is filled exactly, and the footer switches to its own hint.
func TestSheetOverlayRenders(t *testing.T) {
	a, d := newTestApp(t, Options{}, 80, 45) // tall enough for the whole sheet
	d.md = Metadata{Sheet: testSheet()}
	openTestSheet(t, a)

	got := plain(view(a))
	for _, want := range []string{
		"Character sheet — Wren Ashdown",
		"IDENTITY", "Civilised · Hunter · 27", "owes a debt",
		"CHARACTERISTICS", "DERIVED", "SKILLS", "COMBAT STYLES", "PASSIONS",
		"INVENTORY", "HIT LOCATIONS", "CONDITIONS",
		"STR", "CHA", "34/40", "Damage Modifier", "Dodge",
		"Broadsword", "Rope ×3", "Right Arm 3/5", "Poisoned (Right Arm)",
		"esc close · pgup/pgdn scroll",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("sheet should contain %q, view:\n%s", want, got)
		}
	}
	if strings.Contains(got, "No character yet") {
		t.Errorf("a prepared sheet must not show the empty state, view:\n%s", got)
	}
	if lines := strings.Split(view(a), "\n"); len(lines) != 45 {
		t.Errorf("sheet view has %d lines, want 45", len(lines))
	}
	for _, line := range strings.Split(view(a), "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("sheet line wider than the window (%d cols): %q", w, plain(line))
		}
	}
	if a.pal.open {
		t.Error("the palette must close when the sheet opens")
	}

	// The sheet is view-only: a typed message still sends normally.
	typeKeys(a, "noted")
	if cmd := press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		_ = cmd()
	}
	if len(d.sends) != 1 || d.sends[0].text != "noted" {
		t.Errorf("sends = %+v, want the typed message sent under the sheet", d.sends)
	}
}

// TestSheetEmptyState pins the no-character-yet rendering: a zero Sheet
// (no PC in the session) shows the empty state instead of empty sections.
func TestSheetEmptyState(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	openTestSheet(t, a) // Metadata zero: no character yet

	got := plain(view(a))
	if !strings.Contains(got, sheetEmptyState) {
		t.Errorf("empty sheet should show %q, view:\n%s", sheetEmptyState, got)
	}
	if strings.Contains(got, "CHARACTERISTICS") {
		t.Errorf("empty sheet must not render sections, view:\n%s", got)
	}
}

// TestSheetSkillsTwoColumns pins the layout rule: two skill columns once
// the width reaches 100, one column below that.
func TestSheetSkillsTwoColumns(t *testing.T) {
	t.Run("wide window splits the skill list", func(t *testing.T) {
		a, d := newTestApp(t, Options{}, 120, 30)
		d.md = Metadata{Sheet: testSheet()}
		openTestSheet(t, a)
		for _, line := range strings.Split(plain(view(a)), "\n") {
			if strings.Contains(line, "Dodge") && strings.Contains(line, "Track") {
				return // skills from both halves share a row: two columns
			}
		}
		t.Errorf("no two-column skill row found at width 120, view:\n%s", plain(view(a)))
	})

	t.Run("narrow window keeps one column", func(t *testing.T) {
		a, d := newTestApp(t, Options{}, 80, 30)
		d.md = Metadata{Sheet: testSheet()}
		openTestSheet(t, a)
		for _, line := range strings.Split(plain(view(a)), "\n") {
			if strings.Contains(line, "Dodge") && strings.Contains(line, "Track") {
				t.Errorf("skills must stay one per line below width 100: %q", line)
			}
		}
	})
}

// TestSheetScrollRouting pins the overlay's own scrolling: pgup/pgdn and
// the mouse wheel move the sheet (never the transcript), esc closes it,
// and the transcript's follow mode survives untouched throughout.
func TestSheetScrollRouting(t *testing.T) {
	a, d := newTestApp(t, Options{}, 80, 20)
	d.md = Metadata{Sheet: testSheet()}
	// The player had scrolled the transcript up: follow mode is off and
	// must survive the overlay round trip.
	a.follow = false

	openTestSheet(t, a)
	if a.sheetScroll != 0 {
		t.Fatalf("the sheet starts at its top, got scroll %d", a.sheetScroll)
	}
	first := plain(view(a))
	if !strings.Contains(first, "CHARACTERISTICS") {
		t.Fatalf("top of sheet should show CHARACTERISTICS, view:\n%s", first)
	}

	press(t, a, tea.KeyPressMsg{Code: tea.KeyPgDown})
	if a.sheetScroll <= 0 {
		t.Fatal("pgdown should scroll the sheet")
	}
	scrolled := plain(view(a))
	if strings.Contains(scrolled, "CHARACTERISTICS") {
		t.Errorf("scrolled sheet should have left the top, view:\n%s", scrolled)
	}
	press(t, a, tea.KeyPressMsg{Code: tea.KeyPgUp})
	if a.sheetScroll != 0 {
		t.Errorf("pgup should return to the top, got scroll %d", a.sheetScroll)
	}

	// The wheel routes to the sheet while it is open…
	_, _ = a.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if a.sheetScroll == 0 {
		t.Error("wheel down should scroll the sheet")
	}
	// …and clamps at the bottom instead of running past the content.
	for range 100 {
		_, _ = a.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	}
	if max := len(sheetBody(a.theme, a.meta.Sheet, a.width)) - a.sheetBodyHeight(); a.sheetScroll > max {
		t.Errorf("scroll %d past the content bound %d", a.sheetScroll, max)
	}

	if a.follow {
		t.Error("sheet scrolling must not touch the transcript's follow mode")
	}
	press(t, a, tea.KeyPressMsg{Code: tea.KeyEsc})
	if a.sheetOpen {
		t.Error("esc should close the sheet")
	}
	if strings.Contains(plain(view(a)), "Character sheet —") {
		t.Error("closed sheet must not render")
	}
	if a.follow {
		t.Error("closing the sheet must not touch follow mode")
	}
	if hints := plain(view(a)); !strings.Contains(hints, "ctrl+p commands") {
		t.Errorf("the normal footer should return after closing, view:\n%s", hints)
	}
}

// TestCtrlMTogglesPanelUnderSheet keeps ctrl+m a plain two-state panel
// toggle regardless of overlays: the sheet neither opens nor closes it, and
// the panel state is intact when the sheet goes away.
func TestCtrlMTogglesPanelUnderSheet(t *testing.T) {
	a, d := newTestApp(t, Options{}, 80, 20)
	d.md = Metadata{Sheet: testSheet()}
	if a.panelOpen {
		t.Fatal("panel should start closed")
	}
	openTestSheet(t, a)

	press(t, a, ctrl('m'))
	if !a.panelOpen {
		t.Fatal("ctrl+m should open the panel")
	}
	if !a.sheetOpen {
		t.Fatal("ctrl+m must not disturb the sheet overlay")
	}
	press(t, a, ctrl('m'))
	if a.panelOpen {
		t.Fatal("second ctrl+m should close the panel")
	}
	if !a.sheetOpen {
		t.Fatal("ctrl+m must not disturb the sheet overlay")
	}

	// And the toggle still behaves after the sheet closes.
	press(t, a, tea.KeyPressMsg{Code: tea.KeyEsc})
	press(t, a, ctrl('m'))
	if !a.panelOpen || !strings.Contains(plain(view(a)), "METADATA") {
		t.Errorf("panel should open normally after the sheet closes (open=%t)", a.panelOpen)
	}
}

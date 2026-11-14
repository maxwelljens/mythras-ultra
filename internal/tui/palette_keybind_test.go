package tui

import (
	"strings"
	"testing"
)

// TestPaletteKeybinds pins the catalogue's keybind annotations against the
// bindings the app actually handles (ctrl+t channel, ctrl+l clear, ctrl+e
// editor, ctrl+c quit — see the Update keybinding tests). If a binding
// moves, this table is the reminder to move the display with it.
func TestPaletteKeybinds(t *testing.T) {
	want := map[string]string{
		"Switch to OOC":   "ctrl+t",
		"Switch to IC":    "ctrl+t",
		"Clear screen":    "ctrl+l",
		"Open editor":     "ctrl+e",
		"Quit":            "ctrl+c",
		"Character sheet": "",
		"Roll dice":       "",
		"Save session":    "",
		"Session recap":   "",
		"Help":            "",
		"Theme: mythras":  "",
		"Theme: obsidian": "",
	}
	for _, ch := range []Channel{ChannelIC, ChannelOOC} {
		for _, c := range PaletteCommands(ch) {
			if w, ok := want[c.Name]; ok && c.Keybind != w {
				t.Errorf("%s keybind = %q, want %q", c.Name, c.Keybind, w)
			}
		}
	}
	// The dynamic entry: on OOC the offer is "Switch to IC" with ctrl+t.
	for _, c := range PaletteCommands(ChannelOOC) {
		if c.Name == "Switch to IC" && c.Keybind != "ctrl+t" {
			t.Errorf("Switch to IC keybind = %q, want ctrl+t", c.Keybind)
		}
	}
}

// TestPaletteKeybindsRendered pins the visible result: a bound command's
// row carries its shortcut at the right edge, and unbound rows render
// without stray padding text (layout width is covered by the overlay test).
func TestPaletteKeybindsRendered(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	paletteOpen(t, a)
	lines := strings.Split(plain(view(a)), "\n")
	find := func(want string) string {
		for _, l := range lines {
			if strings.Contains(l, want) {
				return l
			}
		}
		return ""
	}
	clearLine := find("Clear screen")
	if clearLine == "" {
		t.Fatalf("Clear screen row missing from view:\n%s", plain(view(a)))
	}
	if !strings.Contains(clearLine, "ctrl+l") {
		t.Errorf("bound row must show its keybind: %q", clearLine)
	}
	sheetLine := find("Character sheet")
	if sheetLine == "" {
		t.Fatalf("Character sheet row missing from view:\n%s", plain(view(a)))
	}
	if strings.Contains(sheetLine, "ctrl+") {
		t.Errorf("unbound row must show no keybind: %q", sheetLine)
	}
}

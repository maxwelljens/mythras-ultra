package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// typeKeys feeds each rune of s to the app as a key press (palette filter
// or input line — whichever holds focus).
func typeKeys(a *App, s string) {
	for _, r := range s {
		_, _ = a.Update(key(r))
	}
}

// paletteOpen opens the palette with ctrl+p (a no-op when it is already
// up).
func paletteOpen(t *testing.T, a *App) {
	t.Helper()
	if a.pal.open {
		return
	}
	press(t, a, ctrl('p'))
	if !a.pal.open {
		t.Fatal("ctrl+p should open the palette")
	}
}

// paletteChoose opens the palette (unless already open), types the filter
// and presses enter. It returns the command executing the entry produced —
// run it to observe driver calls or quit — and fails if the filter matched
// nothing to execute.
func paletteChoose(t *testing.T, a *App, filter string) tea.Cmd {
	t.Helper()
	if !a.pal.open {
		paletteOpen(t, a)
	}
	typeKeys(a, filter)
	cmd := press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
	if a.pal.open {
		t.Fatalf("filter %q selected nothing to execute", filter)
	}
	return cmd
}

// TestPaletteCatalogue pins the palette's source of truth: the exact names
// it renders (in order), the one dynamic channel entry, and the split
// between view commands — executed by the app itself and never sent over
// Driver.Command (the sheet overlay is view-only by design) — and session
// commands, which reach the harness over Driver.Command. The session keeps
// its own dispatch switch (sessionCommands in internal/session/turn.go);
// this list is the TUI side of that contract.
func TestPaletteCatalogue(t *testing.T) {
	want := []string{
		"Character sheet",
		"Roll dice",
		"Save session",
		"Session recap",
		"Switch to OOC", // while on IC; the OOC session offers "Switch to IC"
		"Clear screen",
		"Help",
		"Open editor",
		"Theme: mythras",
		"Theme: obsidian",
		"Theme: parchment",
		"Theme: terminal",
		"Quit",
	}
	got := paletteNames(ChannelIC)
	if len(got) != len(want) {
		t.Fatalf("palette names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("palette name %d = %q, want %q", i, got[i], want[i])
		}
	}

	ooc := paletteNames(ChannelOOC)
	if len(ooc) != len(want) {
		t.Fatalf("OOC palette names = %v, want %d entries", ooc, len(want))
	}
	if ooc[4] != "Switch to IC" {
		t.Errorf("on OOC the switch entry = %q, want %q (only the channel you are NOT on)",
			ooc[4], "Switch to IC")
	}
	for i, name := range ooc {
		if i != 4 && name != want[i] {
			t.Errorf("OOC palette name %d = %q, want %q", i, name, want[i])
		}
	}

	// View commands vs session commands: only "save", "recap" and (after
	// its typed expression) "roll" ever reach Driver.Command; everything
	// else — the character sheet above all — is app-local view state.
	for _, c := range PaletteCommands(ChannelIC) {
		switch c.Name {
		case "Character sheet":
			if c.local != actSheet || c.Session != "" {
				t.Errorf("%q: local=%v Session=%q, want the view-only sheet action",
					c.Name, c.local, c.Session)
			}
		case "Save session", "Session recap":
			if c.Session == "" || c.local != actNone {
				t.Errorf("%q: Session=%q local=%v, want a pure session command",
					c.Name, c.Session, c.local)
			}
		case "Roll dice":
			if c.Session != "roll" || c.local != actRollArg {
				t.Errorf("%q: Session=%q local=%v, want arg mode for roll",
					c.Name, c.Session, c.local)
			}
		default:
			if c.Session != "" || c.local == actNone {
				t.Errorf("%q: Session=%q local=%v, want a local (view) action",
					c.Name, c.Session, c.local)
			}
		}
	}
}

// TestPaletteFilterScoring pins the match order: name prefix beats name
// substring beats name subsequence beats description substring, all
// case-insensitive, and a non-matching query is dropped.
func TestPaletteFilterScoring(t *testing.T) {
	roll := PaletteCommand{Name: "Roll dice", Description: "throw a dice expression (e.g. 2d6+3)"}
	save := PaletteCommand{Name: "Save session", Description: "write state, chat and log to disk"}
	recap := PaletteCommand{Name: "Session recap", Description: "what happened so far"}
	tests := []struct {
		name  string
		query string
		want  []PaletteCommand // in display order; nil = no match
	}{
		{"empty query keeps catalogue order", "", []PaletteCommand{roll, save, recap}},
		{"case-insensitive prefix", "ROLL", []PaletteCommand{roll}},
		{"prefix beats substring, stable within a rank", "sess",
			[]PaletteCommand{recap, save}}, // "session recap" prefixes, "save session" only contains
		{"name substring", "oll", []PaletteCommand{roll}},
		{"name subsequence", "rle", []PaletteCommand{roll}},
		{"description substring last", "throw", []PaletteCommand{roll}},
		{"no match", "zzz", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newPalette()
			p.openWith([]PaletteCommand{roll, save, recap})
			p.filter.SetValue(tt.query)
			p.refilter()
			if len(p.matches) != len(tt.want) {
				t.Fatalf("matches = %d (%v), want %d", len(p.matches), p.matches, len(tt.want))
			}
			for i, want := range tt.want {
				if got := p.entries[p.matches[i]]; got != want {
					t.Errorf("match %d = %q, want %q", i, got.Name, want.Name)
				}
			}
		})
	}
}

// TestPaletteFilterRanks checks the rank ladder directly: prefix (0) >
// substring (1) > subsequence (2) > description (3), and non-matches fail.
func TestPaletteFilterRanks(t *testing.T) {
	c := PaletteCommand{Name: "Roll dice", Description: "throw a dice expression"}
	tests := []struct {
		query  string
		want   int
		wantOK bool
	}{
		{"", 0, true},
		{"roll", 0, true},
		{"oll", 1, true},
		{"rll", 2, true},
		{"hrow", 3, true},
		{"zz", 0, false},
	}
	for _, tt := range tests {
		rank, ok := matchRank(c, tt.query)
		if ok != tt.wantOK || (ok && rank != tt.want) {
			t.Errorf("matchRank(%q) = (%d, %t), want (%d, %t)",
				tt.query, rank, ok, tt.want, tt.wantOK)
		}
	}
}

// TestPaletteNavigation pins selection movement: up/down wrap around, the
// visible window stays at paletteRows rows, and the selection is
// highlighted. Every transition is a key press — no timers.
func TestPaletteNavigation(t *testing.T) {
	a, d := newTestApp(t, Options{}, 60, 24)
	paletteOpen(t, a)
	total := len(a.pal.entries) // 13 entries in the default catalogue
	if total <= paletteRows {
		t.Fatalf("catalogue has %d entries, want more than the %d-row window", total, paletteRows)
	}
	if a.pal.sel != 0 {
		t.Fatalf("selection starts at %d, want 0", a.pal.sel)
	}

	// Down walks to the last row and wraps to the first.
	for i := 1; i < total; i++ {
		press(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
		if a.pal.sel != i {
			t.Fatalf("after %d downs sel = %d, want %d", i, a.pal.sel, i)
		}
	}
	press(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	if a.pal.sel != 0 {
		t.Errorf("down past the last row wrapped to %d, want 0", a.pal.sel)
	}
	// Up from the first row wraps to the last.
	press(t, a, tea.KeyPressMsg{Code: tea.KeyUp})
	if a.pal.sel != total-1 {
		t.Errorf("up from row 0 wrapped to %d, want %d", a.pal.sel, total-1)
	}
	// The window shows at most paletteRows rows and keeps the selection in
	// view (selected at the end → window scrolled by one).
	if got := a.pal.top + paletteRows; a.pal.sel < a.pal.top || a.pal.sel >= got {
		t.Errorf("selection %d outside the window [%d,%d)", a.pal.sel, a.pal.top, got)
	}
	got := plain(a.pal.view(a.theme, a.width, a.height))
	if !strings.Contains(got, "> "+a.pal.entries[a.pal.matches[a.pal.sel]].Name) {
		t.Errorf("selected row should carry the > marker, view:\n%s", got)
	}
	for _, want := range []string{"command ▸ ", "enter run", "esc close"} {
		if !strings.Contains(got, want) {
			t.Errorf("palette should show %q, view:\n%s", want, got)
		}
	}

	// No commands match: the placeholder row shows and enter does nothing.
	typeKeys(a, "zzzzzz")
	if !strings.Contains(plain(a.pal.view(a.theme, a.width, a.height)), "no commands match") {
		t.Errorf("empty filter should show the placeholder, view:\n%s",
			plain(a.pal.view(a.theme, a.width, a.height)))
	}
	press(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !a.pal.open {
		t.Error("enter with no matches must not close the palette")
	}

	// esc closes without running anything; ctrl+c still quits.
	press(t, a, tea.KeyPressMsg{Code: tea.KeyEsc})
	if a.pal.open {
		t.Error("esc should close the palette")
	}
	if len(d.commands) != 0 || len(d.sends) != 0 {
		t.Errorf("closing must not act: commands=%v sends=%v", d.commands, d.sends)
	}
	// ctrl+p toggles the palette shut when it is already up.
	paletteOpen(t, a)
	press(t, a, ctrl('p'))
	if a.pal.open {
		t.Error("ctrl+p should close the open palette")
	}
	// ctrl+c still quits while the palette is up.
	paletteOpen(t, a)
	if cmd := press(t, a, ctrl('c')); cmd == nil {
		t.Fatal("ctrl+c should quit from the palette")
	} else if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c from the palette should quit")
	}
	// q types into the filter — it must never quit while the palette is up.
	cmd := press(t, a, key('q'))
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("q must not quit while the palette is open")
		}
	}
	if a.pal.filter.Value() != "q" {
		t.Errorf("filter = %q, want %q (typing reaches the filter line)", a.pal.filter.Value(), "q")
	}
}

// TestPaletteOverlayLayout checks the rendered overlay: a centred bordered
// box that fills the body area exactly and never spills the window.
func TestPaletteOverlayLayout(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 80, 20)
	paletteOpen(t, a)
	got := view(a)
	if lines := strings.Split(got, "\n"); len(lines) != 20 {
		t.Errorf("palette view has %d lines, want 20", len(lines))
	}
	for _, line := range strings.Split(got, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line wider than the window (%d cols): %q", w, plain(line))
		}
	}
	if txt := plain(got); !strings.Contains(txt, "COMMANDS") ||
		!strings.Contains(txt, "Character sheet") {
		t.Errorf("overlay should list the catalogue, view:\n%s", txt)
	}
}

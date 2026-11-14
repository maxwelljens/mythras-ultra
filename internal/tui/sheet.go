package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// sheetEmptyState is what the overlay says while the session has no player
// character yet (the zero-value Sheet).
const sheetEmptyState = "No character yet — the GM will ask you to create one."

// sheetHeader renders the overlay's fixed header line: "Character sheet —
// <name>", dropping the dash while no name is known.
func sheetHeader(s Sheet) string {
	if s.Name == "" {
		return "Character sheet"
	}
	return "Character sheet — " + s.Name
}

// sheetBody renders every scrollable content line of the sheet for a width
// of w columns: section headers in the accent slot, values in the narrative
// slot, skills in two columns once the width allows it (≥100 columns), and
// a dim empty-state line when no character exists yet. It is a pure
// function of theme, sheet and size — the app renders from its cached
// Metadata snapshot, never from the driver mid-view.
func sheetBody(th Theme, s Sheet, w int) []string {
	if s.Name == "" && len(s.Characteristics) == 0 {
		return []string{th.Styles.Muted.Render(sheetEmptyState)}
	}
	if w < 1 {
		w = 80
	}
	p := sheetWriter{th: th, w: w}
	p.identity(s)
	p.header("CHARACTERISTICS", nameValueRows(s.Characteristics, 4))
	p.header("DERIVED", nameValueRows(s.Derived, 16))
	p.header("SKILLS", p.rated(s.Skills, w >= 100))
	p.header("COMBAT STYLES", nameValueRows(s.Styles, 0))
	p.header("PASSIONS", nameValueRows(s.Passions, 0))
	p.header("INVENTORY", textRows(s.Inventory))
	p.header("HIT LOCATIONS", locationSheetRows(s.Locations))
	p.header("CONDITIONS", textRows(s.Conditions))
	return p.lines
}

// sheetWriter accumulates the sheet's rendered lines at a fixed width.
type sheetWriter struct {
	th    Theme
	w     int
	lines []string
}

// add appends s wrapped to the sheet width, each wrapped row styled with st.
func (p *sheetWriter) add(st lipgloss.Style, s string) {
	for _, line := range strings.Split(lipgloss.Wrap(s, p.w, ""), "\n") {
		p.lines = append(p.lines, st.Render(line))
	}
}

// identity writes the background line(s) — culture · career · age and the
// concept — when the character records any of them; a bare sheet skips the
// section entirely.
func (p *sheetWriter) identity(s Sheet) {
	var origin []string
	for _, part := range []string{s.Culture, s.Career, s.Age} {
		if strings.TrimSpace(part) != "" {
			origin = append(origin, part)
		}
	}
	var rows []string
	if len(origin) > 0 {
		rows = append(rows, strings.Join(origin, " · "))
	}
	if s.Concept != "" {
		rows = append(rows, s.Concept)
	}
	if len(rows) > 0 {
		p.header("IDENTITY", rows)
	}
}

// header writes a section header followed by its rows — or the dim "—"
// placeholder when the section has nothing to show.
func (p *sheetWriter) header(name string, rows []string) {
	p.lines = append(p.lines, p.th.Styles.Accent.Render(name))
	if len(rows) == 0 {
		p.lines = append(p.lines, p.th.Styles.Muted.Render("—"))
		return
	}
	for _, row := range rows {
		p.add(p.th.Styles.Narrative, row)
	}
}

// rated lays skills/styles out in two columns when two is true (name value
// pairs side by side), otherwise one per line.
func (p *sheetWriter) rated(rows []NameValue, two bool) []string {
	plain := nameValueRows(rows, 0)
	if !two || len(plain) == 0 {
		return plain
	}
	half := (len(plain) + 1) / 2
	left, right := plain[:half], plain[half:]
	col := 0
	for _, s := range left {
		if w := lipgloss.Width(s); w > col {
			col = w
		}
	}
	out := make([]string, 0, half)
	for i, s := range left {
		cell := s
		if i < len(right) {
			cell = lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Width(col+2).Render(s), right[i])
		}
		out = append(out, cell)
	}
	return out
}

// nameValueRows renders NameValue entries as "name value", name padded to
// pad columns (0 = pad to the longest name in the list).
func nameValueRows(rows []NameValue, pad int) []string {
	if len(rows) == 0 {
		return nil
	}
	if pad == 0 {
		for _, r := range rows {
			if w := lipgloss.Width(r.Name); w > pad {
				pad = w
			}
		}
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, padRight(r.Name, pad+1)+r.Value)
	}
	return out
}

// textRows passes a string list through (nil for none).
func textRows(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	return append([]string(nil), items...)
}

// locationSheetRows renders one "Name current/max" row per hit location.
func locationSheetRows(locs []LocationHP) []string {
	if len(locs) == 0 {
		return nil
	}
	out := make([]string, 0, len(locs))
	for _, loc := range locs {
		out = append(out, loc.Name+" "+strconv.Itoa(loc.Current)+"/"+strconv.Itoa(loc.Max))
	}
	return out
}

// openSheet shows the character-sheet overlay over the transcript and
// panel. The snapshot is refreshed first so the overlay renders the
// session's current state. The transcript keeps accepting events behind
// it; follow mode and the transcript viewport are left exactly as they
// were — the sheet scrolls on its own (see scrollSheet) and starts at its
// top.
func (a *App) openSheet() {
	a.refreshMetadata()
	a.sheetOpen = true
	a.sheetScroll = 0
}

// closeSheet hides the overlay; the transcript reappears where it was.
func (a *App) closeSheet() { a.sheetOpen = false }

// sheetBodyHeight is how many body lines one overlay screen shows: the
// window minus the app header, the sheet header, the input line and the
// footer.
func (a *App) sheetBodyHeight() int {
	h := a.height - 4
	if h < 1 {
		h = 1
	}
	return h
}

// sheetPage is the scroll step of one pgup/pgdn press (a full screen).
func (a *App) sheetPage(up bool) int {
	if up {
		return -a.sheetBodyHeight()
	}
	return a.sheetBodyHeight()
}

// wheelDelta is the sheet's scroll step for one wheel notch.
func wheelDelta(msg tea.MouseWheelMsg) int {
	if msg.Button == tea.MouseWheelUp {
		return -3
	}
	return 3
}

// scrollSheet moves the overlay's own scroll position by delta lines and
// clamps it into the content. It never touches the transcript viewport or
// follow mode.
func (a *App) scrollSheet(delta int) {
	body := sheetBody(a.theme, a.meta.Sheet, a.width)
	max := len(body) - a.sheetBodyHeight()
	if max < 0 {
		max = 0
	}
	a.sheetScroll += delta
	if a.sheetScroll < 0 {
		a.sheetScroll = 0
	}
	if a.sheetScroll > max {
		a.sheetScroll = max
	}
}

// renderSheet builds the overlay: the fixed header line over the visible
// window of body lines at the current scroll position. It only reads state
// (an out-of-range scroll from a resize is clamped locally, never written
// back), so View stays a pure function.
func (a *App) renderSheet() string {
	body := sheetBody(a.theme, a.meta.Sheet, a.width)
	h := a.sheetBodyHeight()
	top := a.sheetScroll
	if max := len(body) - h; top > max {
		top = max
	}
	if top < 0 {
		top = 0
	}
	end := top + h
	if end > len(body) {
		end = len(body)
	}
	lines := make([]string, 0, end-top+1)
	lines = append(lines, a.theme.Styles.Header.Render(sheetHeader(a.meta.Sheet)))
	lines = append(lines, body[top:end]...)
	// Pad with blank lines so the overlay always fills the body area exactly.
	for len(lines) < h+1 {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

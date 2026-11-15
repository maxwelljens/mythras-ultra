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

// sheetTwoColumnMin is the sheet width from which the skill sections lay
// their rows out in two columns; below it one column keeps every name and
// value on one readable line.
const sheetTwoColumnMin = 100

// sheetHeader renders the overlay's fixed title line — the banner below
// carries the character's name, so the title stays the same on every sheet.
func sheetHeader() string { return "Character sheet" }

// severityCells maps the panel's wound vocabulary onto the wound table's
// state cell: a single-width glyph plus its word.
var severityCells = map[string]string{
	"":         "● healthy",
	"wound":    "▲ wounded",
	"severe":   "✖ severe",
	"critical": "✖ critical",
}

// derivedIcons maps each derived readout onto its single-width glyph, in
// the order the row shows them.
var derivedIcons = []struct{ name, glyph string }{
	{"HP", "♥"},
	{"Damage Modifier", "⚔"},
	{"Power Points", "✧"},
	{"Luck", "✦"},
	{"Experience Bonus", "▲"},
}

// sheetBody renders every scrollable content line of the sheet for a width
// of w columns: a name banner, the characteristics as inline chips, the
// derived readouts as an icon row, then the wound table and the sections
// (skills, styles, passions, inventory, conditions). It is a pure function
// of theme, sheet and size — the app renders from its cached Metadata
// snapshot, never from the driver mid-view — and every line it returns
// fits w columns.
func sheetBody(th Theme, s Sheet, w int) []string {
	if s.Name == "" && len(s.Characteristics) == 0 {
		return emptySheet(th, w)
	}
	if w < 1 {
		w = 80
	}
	p := sheetWriter{th: th, w: w}
	p.banner(s)
	p.characteristics(s)
	p.derived(s)
	p.wounds(s.Locations)
	p.skills(s)
	p.ratedSection("⚔ Styles", s.Styles)
	p.ratedSection("✺ Passions", s.Passions)
	p.inventory(s.Inventory)
	p.section("Conditions", textRows(s.Conditions))
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

// rule writes a section header in the accent slot ("── Skills ──").
func (p *sheetWriter) rule(title string) {
	p.lines = append(p.lines, p.th.Styles.Accent.Render("── "+title+" ──"))
}

// placeholder is the dim dash a section with nothing to show renders.
func (p *sheetWriter) placeholder() {
	p.lines = append(p.lines, p.th.Styles.Muted.Render("—"))
}

// section writes a section header followed by its rows — or the dim "—"
// placeholder when the section has nothing to show.
func (p *sheetWriter) section(title string, rows []string) {
	p.rule(title)
	if len(rows) == 0 {
		p.placeholder()
		return
	}
	for _, row := range rows {
		p.add(p.th.Styles.Narrative, row)
	}
}

// banner writes the header block: the character's name in a bordered bar,
// the origin line (culture · career · age) under it, and the concept in
// the muted slot. Parts the character does not record are left out.
func (p *sheetWriter) banner(s Sheet) {
	if s.Name != "" {
		p.lines = append(p.lines, p.nameBar(s.Name)...)
	}
	var origin []string
	for _, part := range []string{s.Culture, s.Career, s.Age} {
		if strings.TrimSpace(part) != "" {
			origin = append(origin, part)
		}
	}
	if len(origin) > 0 {
		p.add(p.th.Styles.Panel, strings.Join(origin, " · "))
	}
	if s.Concept != "" {
		p.add(p.th.Styles.Muted, s.Concept)
	}
}

// nameBar renders the name inside a bordered bar sized to the sheet width,
// in the highlight slot so the identity reads first. Below six columns a
// border cannot hold its own padding, so the bare name shows instead.
func (p *sheetWriter) nameBar(name string) []string {
	if p.w < 6 {
		return []string{p.th.Styles.Highlight.Render(name)}
	}
	inner := p.w - 4 // normal border (2) + horizontal padding (2)
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(p.th.Palette[SlotBorder]).
		Padding(0, 1).
		Width(p.w).
		Render(p.th.Styles.Highlight.Render(lipgloss.Wrap(name, inner, "")))
	return strings.Split(box, "\n")
}

// characteristics writes the seven characteristics as inline chips —
// "[ STR 14 ]" — packed into as many rows as the width needs: one row on a
// wide sheet, two on a narrow one. The value carries the highlight slot.
func (p *sheetWriter) characteristics(s Sheet) {
	p.rule("Characteristics")
	if len(s.Characteristics) == 0 {
		p.placeholder()
		return
	}
	chips := make([]string, 0, len(s.Characteristics))
	for _, c := range s.Characteristics {
		chips = append(chips,
			p.th.Styles.Muted.Render("[ ")+
				p.th.Styles.Accent.Render(c.Name)+" "+
				p.th.Styles.Highlight.Render(c.Value)+
				p.th.Styles.Muted.Render(" ]"))
	}
	p.lines = append(p.lines, packRow(chips, p.w, " ")...)
}

// derived writes the pooled readouts as one evenly spaced icon row: ♥ HP
// with its block bar, ⚔ damage modifier, ✧ power points, ✦ luck and ▲
// experience bonus. Readouts the snapshot does not carry are skipped, and
// the row wraps rather than overflow.
func (p *sheetWriter) derived(s Sheet) {
	p.rule("Derived")
	if len(s.Derived) == 0 {
		p.placeholder()
		return
	}
	items := make([]string, 0, len(s.Derived))
	width := 0
	for _, d := range s.Derived {
		glyph, ok := derivedGlyph(d.Name)
		if !ok {
			continue
		}
		value := d.Value
		if d.Name == "HP" && s.HPMax > 0 {
			value += " " + hpBar(s.HPCurrent, s.HPMax)
		}
		item := p.th.Styles.Accent.Render(glyph+" ") + p.th.Styles.Highlight.Render(value)
		if w := lipgloss.Width(item); w > width {
			width = w
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		p.placeholder()
		return
	}
	// Even spacing: pad every item to the widest, then one space between.
	for i, item := range items {
		items[i] = item + strings.Repeat(" ", width-lipgloss.Width(item))
	}
	for _, row := range packRow(items, p.w, " ") {
		p.lines = append(p.lines, strings.TrimRight(row, " ")) // no trailing padding
	}
}

// derivedGlyph returns the row glyph of a derived readout by its engine
// name; unknown names read as "" so the caller skips them.
func derivedGlyph(name string) (string, bool) {
	for _, d := range derivedIcons {
		if d.name == name {
			return d.glyph, true
		}
	}
	return "", false
}

// wounds writes the hit locations as a compact box-drawing table of
// location, live HP and the severity glyph (● healthy, ▲ wounded, ✖ …),
// each state cell coloured through the palette's outcome slots.
func (p *sheetWriter) wounds(locs []LocationHP) {
	p.rule("Wounds")
	if len(locs) == 0 {
		p.placeholder()
		return
	}
	rows := make([][]string, 0, len(locs))
	for _, loc := range locs {
		rows = append(rows, []string{
			loc.Name,
			strconv.Itoa(loc.Current) + "/" + strconv.Itoa(loc.Max),
			p.severityCell(loc.Severity),
		})
	}
	p.lines = append(p.lines, p.table([]string{"Location", "HP", "State"}, rows)...)
}

// severityCell renders one wound-table state cell: the severity glyph and
// word, coloured by the severity slot (unknown levels keep the panel
// colour and show their own name).
func (p *sheetWriter) severityCell(severity string) string {
	cell, ok := severityCells[severity]
	if !ok {
		cell = "● " + severity
	}
	return severityStyle(p.th, severity).Render(cell)
}

// skills writes the skill sections: the snapshot's categories as their own
// sections when it carries them, otherwise one section in the shared
// value-descending order.
func (p *sheetWriter) skills(s Sheet) {
	if len(s.SkillGroups) > 0 {
		for _, group := range s.SkillGroups {
			p.section(group.Name, p.rated(group.Rows))
		}
		return
	}
	p.section("Skills", p.rated(s.Skills))
}

// ratedSection writes a rated section under its icon-headed rule
// ("── ⚔ Styles ──").
func (p *sheetWriter) ratedSection(title string, rows []NameValue) {
	p.section(title, p.rated(rows))
}

// rated lays rated rows out in two columns once the sheet is wide enough —
// values right-aligned inside each column, so "Stealth 45%" lines up with
// the rows above and below — and one per line below that (or whenever two
// columns would not fit).
func (p *sheetWriter) rated(rows []NameValue) []string {
	plain := alignedRows(rows)
	if len(plain) < 2 || p.w < sheetTwoColumnMin {
		return plain
	}
	half := (len(plain) + 1) / 2
	left, right := plain[:half], plain[half:]
	colW, rightW := maxWidth(left), maxWidth(right)
	if colW+2+rightW > p.w {
		return plain // two columns would overflow: one column always fits
	}
	out := make([]string, 0, half)
	for i, row := range left {
		cell := row
		if i < len(right) {
			cell = padDisplay(row, colW) + "  " + right[i]
		}
		out = append(out, cell)
	}
	return out
}

// alignedRows renders rated rows with the name padded to the widest name
// and the value padded left to the widest value, so every value ends at
// the same column of its row.
func alignedRows(rows []NameValue) []string {
	if len(rows) == 0 {
		return nil
	}
	nameW, valueW := maxWidthNames(rows), 0
	for _, r := range rows {
		if w := lipgloss.Width(r.Value); w > valueW {
			valueW = w
		}
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		name := r.Name + strings.Repeat(" ", nameW-lipgloss.Width(r.Name))
		value := strings.Repeat(" ", valueW-lipgloss.Width(r.Value)) + r.Value
		out = append(out, name+" "+value)
	}
	return out
}

// maxWidthNames is the widest name of a rated list.
func maxWidthNames(rows []NameValue) int {
	width := 0
	for _, r := range rows {
		if w := lipgloss.Width(r.Name); w > width {
			width = w
		}
	}
	return width
}

// inventory writes the carried items as a small table: quantity, item and
// the equip marker (✔ plus the slot when one is named).
func (p *sheetWriter) inventory(items []InventoryItem) {
	p.rule("Inventory")
	if len(items) == 0 {
		p.placeholder()
		return
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		equipped := ""
		if item.Equipped {
			equipped = "✔"
			if item.Slot != "" {
				equipped += " " + item.Slot
			}
		}
		rows = append(rows, []string{strconv.Itoa(item.Qty), item.Name, equipped})
	}
	p.lines = append(p.lines, p.table([]string{"Qty", "Item", "Equipped"}, rows)...)
}

// table renders a compact box-drawing table (┌ ─ ┬ │ …): top border,
// header row, rule, data rows and bottom border, every column padded to
// its widest cell. When the box would not fit the sheet width — a narrow
// window or long cells — the same content falls back to plain wrapped
// rows, so no line ever overflows and nothing is dropped.
func (p *sheetWriter) table(headers []string, rows [][]string) []string {
	if len(rows) == 0 {
		return nil
	}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				if w := lipgloss.Width(cell); w > widths[i] {
					widths[i] = w
				}
			}
		}
	}
	total := 2 // outer corners
	for _, w := range widths {
		total += w + 3
	}
	if total > p.w {
		return p.plainTable(headers, rows)
	}
	rule := func(left, mid, right string) string {
		var b strings.Builder
		b.WriteString(left)
		for i, w := range widths {
			if i > 0 {
				b.WriteString(mid)
			}
			b.WriteString(strings.Repeat("─", w+2))
		}
		b.WriteString(right)
		return b.String()
	}
	cells := func(row []string) string {
		var b strings.Builder
		for i, cell := range row {
			b.WriteString("│ ")
			b.WriteString(padDisplay(cell, widths[i]))
			b.WriteString(" ")
		}
		return b.String() + "│"
	}
	out := []string{rule("┌", "┬", "┐"), cells(headers), rule("├", "┼", "┤")}
	for _, row := range rows {
		out = append(out, cells(row))
	}
	return append(out, rule("└", "┴", "┘"))
}

// plainTable is the table fallback for a width the box cannot fit: the
// header in the accent slot and one space-joined row per line, each
// wrapped to the sheet width (cell styling survives the wrap).
func (p *sheetWriter) plainTable(headers []string, rows [][]string) []string {
	out := make([]string, 0, len(rows)+1)
	for _, line := range strings.Split(lipgloss.Wrap(strings.Join(headers, "  "), p.w, ""), "\n") {
		out = append(out, p.th.Styles.Accent.Render(line))
	}
	for _, row := range rows {
		out = append(out, strings.Split(lipgloss.Wrap(strings.Join(row, "  "), p.w, ""), "\n")...)
	}
	return out
}

// packRow packs styled items into rows of at most w display columns,
// separated by sep. An item wider than w gets a row of its own, so nothing
// is ever dropped.
func packRow(items []string, w int, sep string) []string {
	var rows []string
	current := ""
	for _, item := range items {
		switch {
		case current == "":
			current = item
		case lipgloss.Width(current)+lipgloss.Width(sep)+lipgloss.Width(item) <= w:
			current += sep + item
		default:
			rows = append(rows, current)
			current = item
		}
	}
	if current != "" {
		rows = append(rows, current)
	}
	return rows
}

// padDisplay pads s with spaces to exactly n display columns; a cell
// already at least that wide (it never should be — n is the widest cell)
// passes through untouched.
func padDisplay(s string, n int) string {
	if gap := n - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// maxWidth is the widest display width of a list of rendered rows.
func maxWidth(rows []string) int {
	width := 0
	for _, row := range rows {
		if w := lipgloss.Width(row); w > width {
			width = w
		}
	}
	return width
}

// textRows passes a string list through (nil for none).
func textRows(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	return append([]string(nil), items...)
}

// emptySheet renders the no-character-yet state as a bordered box around
// the muted message, sized to the sheet width (bare text when the window
// is too narrow for a border).
func emptySheet(th Theme, w int) []string {
	if w < 1 {
		w = 80
	}
	if w < 6 {
		return []string{th.Styles.Muted.Render(sheetEmptyState)}
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(th.Palette[SlotBorder]).
		Padding(0, 1).
		Width(w).
		Render(th.Styles.Muted.Render(lipgloss.Wrap(sheetEmptyState, w-4, "")))
	return strings.Split(box, "\n")
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
// window minus the app header, the status line, the sheet header, the
// input line and the footer.
func (a *App) sheetBodyHeight() int {
	h := a.height - 5
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
	lines = append(lines, a.theme.Styles.Header.Render(sheetHeader()))
	lines = append(lines, body[top:end]...)
	// Pad with blank lines so the overlay always fills the body area exactly.
	for len(lines) < h+1 {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

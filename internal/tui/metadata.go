package tui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// metadataTick is the slow refresh cadence of the cached Metadata snapshot.
// Rendering is always a pure read of the app's cache; the cache itself is
// refreshed on every processed event, on every panel toggle, and on this
// tick (see Driver.Metadata and App.refreshMetadata).
const metadataTick = 2 * time.Second

// hpBarCells is the fixed width, in block glyphs, of one location's HP bar.
const hpBarCells = 4

// metadataTickMsg is the slow panel refresh tick; Update re-arms it.
type metadataTickMsg struct{}

// severityStyle maps a hit location's severity onto the theme's outcome
// slots, escalating Success (unhurt) → Failure → Critical → Fumble. Fumble
// is red in every built-in theme, so a critical location always reads as the
// alarm it is; an unrecognised severity falls back to the panel colour.
func severityStyle(th Theme, severity string) lipgloss.Style {
	switch severity {
	case "":
		return th.Styles.Success
	case "wound":
		return th.Styles.Failure
	case "severe":
		return th.Styles.Critical
	case "critical":
		return th.Styles.Fumble
	default:
		return th.Styles.Panel
	}
}

// hpBar renders a block-glyph bar of hpBarCells cells: ▓ for the current
// fraction, ░ for the rest, rounded to the nearest cell. cur is clamped into
// [0, max] so an over-max value shows a full bar, and a max of 0 or less
// renders an empty bar rather than dividing by zero.
func hpBar(cur, max int) string {
	if max <= 0 {
		return strings.Repeat("░", hpBarCells)
	}
	if cur < 0 {
		cur = 0
	}
	if cur > max {
		cur = max
	}
	filled := (2*cur*hpBarCells + max) / (2 * max) // round half up
	return strings.Repeat("▓", filled) + strings.Repeat("░", hpBarCells-filled)
}

// panelLine is one styled line destined for the metadata panel.
type panelLine struct {
	st   lipgloss.Style
	text string
}

// panelWriter accumulates the panel's rendered lines.
type panelWriter struct {
	th    Theme
	w     int
	lines []string
}

// add appends s wrapped to the panel width and styled with st, one output
// line per wrapped row, so no line can push the panel past its slot.
func (p *panelWriter) add(st lipgloss.Style, s string) {
	for _, line := range strings.Split(lipgloss.Wrap(s, p.w, ""), "\n") {
		p.lines = append(p.lines, st.Render(line))
	}
}

// section writes a section header followed by its lines — or the dim "—"
// placeholder when the section has nothing to show.
func (p *panelWriter) section(name string, ls []panelLine) {
	p.add(p.th.Styles.Accent, name)
	if len(ls) == 0 {
		p.add(p.th.Styles.Muted, "—")
		return
	}
	for _, l := range ls {
		p.add(l.st, l.text)
	}
}

// listLines shapes a string list into panel lines: one terse line joined with
// sep when sep is non-empty (inventory), one item per line when sep is empty
// (clocks, flags). An empty list yields nil so the section renders "—".
func listLines(th Theme, items []string, sep string) []panelLine {
	if len(items) == 0 {
		return nil
	}
	if sep != "" {
		return []panelLine{{st: th.Styles.Panel, text: strings.Join(items, sep)}}
	}
	ls := make([]panelLine, 0, len(items))
	for _, item := range items {
		ls = append(ls, panelLine{st: th.Styles.Panel, text: item})
	}
	return ls
}

// characterLines renders the character block: name, culture · career, the
// display strings HP/AP/PP/Luck, and any conditions.
func characterLines(th Theme, v Vitals) []panelLine {
	var ls []panelLine
	add := func(text string) {
		ls = append(ls, panelLine{st: th.Styles.Panel, text: text})
	}
	if v.Name != "" {
		add(v.Name)
	}
	var origin []string
	if culture := strings.TrimSpace(v.Culture); culture != "" {
		origin = append(origin, culture)
	}
	if career := strings.TrimSpace(v.Career); career != "" {
		origin = append(origin, career)
	}
	if len(origin) > 0 {
		add(strings.Join(origin, " · "))
	}
	var stats []string
	if v.HP != "" {
		stats = append(stats, "HP "+v.HP)
	}
	if v.AP != "" {
		stats = append(stats, "AP "+v.AP)
	}
	if v.PowerPoints != "" {
		stats = append(stats, "PP "+v.PowerPoints)
	}
	if v.Luck != "" {
		stats = append(stats, "Luck "+v.Luck)
	}
	for _, stat := range stats {
		add(stat) // one stat per line: the panel column is narrow
	}
	if len(v.Conditions) > 0 {
		add("Conditions: " + strings.Join(v.Conditions, ", "))
	}
	return ls
}

// locationLines renders one HP-barred line per hit location, coloured by
// severity.
func locationLines(th Theme, locs []LocationHP) []panelLine {
	ls := make([]panelLine, 0, len(locs))
	for _, loc := range locs {
		ls = append(ls, panelLine{
			st: severityStyle(th, loc.Severity),
			text: loc.Name + " " + hpBar(loc.Current, loc.Max) + " " +
				strconv.Itoa(loc.Current) + "/" + strconv.Itoa(loc.Max),
		})
	}
	return ls
}

// sceneLines renders the scene sentence and who is present, each when set.
func sceneLines(th Theme, m Metadata) []panelLine {
	var ls []panelLine
	if m.Scene != "" {
		ls = append(ls, panelLine{st: th.Styles.Panel, text: m.Scene})
	}
	if len(m.Present) > 0 {
		ls = append(ls, panelLine{
			st:   th.Styles.Panel,
			text: "Present: " + strings.Join(m.Present, ", "),
		})
	}
	return ls
}

// sessionLines renders the session name and in-game date, each when set.
func sessionLines(th Theme, m Metadata) []panelLine {
	var ls []panelLine
	if m.SessionName != "" {
		ls = append(ls, panelLine{st: th.Styles.Panel, text: m.SessionName})
	}
	if m.InGameDate != "" {
		ls = append(ls, panelLine{st: th.Styles.Panel, text: m.InGameDate})
	}
	return ls
}

// metadataLines builds every content line of the panel for a snapshot: the
// header plus one section per group, empty sections as dim "—" lines. It
// never panics on a zero-value Metadata.
func metadataLines(th Theme, m Metadata, w int) []string {
	p := panelWriter{th: th, w: w}
	p.add(th.Styles.Header, "METADATA")
	sections := []struct {
		name  string
		lines []panelLine
	}{
		{"CHARACTER", characterLines(th, m.Character)},
		{"LOCATIONS", locationLines(th, m.Character.Locations)},
		{"INVENTORY", listLines(th, m.Inventory, " · ")},
		{"SCENE", sceneLines(th, m)},
		{"CLOCKS", listLines(th, m.Clocks, "")},
		{"FLAGS", listLines(th, m.Flags, "")},
		{"SESSION", sessionLines(th, m)},
	}
	for _, s := range sections {
		p.section(s.name, s.lines)
	}
	return p.lines
}

// renderMetadata builds the right-hand metadata panel for a snapshot: a
// bordered box of styled sections, taking ~30% of the window width and the
// height of the transcript area. It is a pure function of theme, snapshot
// and size — the app renders from its cached Metadata, never from the
// driver mid-view — and clips to the available height, so opening the panel
// cannot break the chat layout.
func renderMetadata(th Theme, m Metadata, width, height int) string {
	panelW := width * 3 / 10
	if panelW < 3 {
		panelW = 3
	}
	innerW := panelW - 4 // normal border (2) + horizontal padding (2)
	if innerW < 1 {
		innerW = 1
	}
	bodyH := height - 4 // header, status line, input line, footer
	if bodyH < 1 {
		bodyH = 1
	}
	lines := metadataLines(th, m, innerW)
	maxLines := bodyH - 2 // top and bottom border
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	content := th.Styles.Panel.MaxWidth(innerW).Render(strings.Join(lines, "\n"))
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(th.Palette[SlotBorder]).
		Width(panelW).
		Height(bodyH).
		Padding(0, 1)
	return box.Render(content)
}

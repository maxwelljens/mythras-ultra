package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// helpKeys lists the keybindings the help overlay shows, as [key,
// description] pairs in footer-hint order. Rows stay few and short so the
// whole overlay fits an 80×20 window; the last row folds scrolling and
// history together.
var helpKeys = [][2]string{
	{"ctrl+t", "in/out of character"},
	{"ctrl+e", "edit input in $VISUAL"},
	{"ctrl+m", "metadata panel"},
	{"ctrl+l", "clear transcript"},
	{"esc", "clear input, close overlay"},
	{"enter", "send, close overlay"},
	{"q", "quit (empty input)"},
	{"pgup/pgdn", "scroll · up/down history"},
}

// helpCommands splits input.go's commandNames by where submit dispatches
// them: local commands act inside the TUI, harness commands go to the driver
// verbatim. Classifying by kind keeps this list from drifting from the
// parser.
func helpCommands(harness bool) []string {
	out := make([]string, 0, len(commandNames))
	for _, name := range commandNames {
		switch commandKinds[name] {
		case CmdRoll, CmdSave, CmdRecap, CmdMeta:
			if harness {
				out = append(out, "/"+name)
			}
		default:
			if !harness {
				out = append(out, "/"+name)
			}
		}
	}
	return out
}

// renderHelp builds the help overlay for a width×height area: a bordered,
// centred box listing every keybinding and slash command with a dim
// dismissal hint. It is a pure function of theme and size so tests can
// render it without an App. Rows assume a reasonably wide terminal; on
// narrower ones an over-long row spills and the terminal wraps it.
func renderHelp(th Theme, width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	innerW := width - 6 // border (2) + padding (2) + a margin on each side
	if innerW < 8 {
		innerW = 8
	}
	if innerW > 72 {
		innerW = 72 // long help lines read better in a narrow column
	}

	lines := []string{
		th.Styles.Highlight.Render("MYTHRAS ULTRA · HELP"),
		"",
		th.Styles.Header.Render("KEYS"),
	}
	for _, k := range helpKeys {
		lines = append(lines,
			th.Styles.Highlight.Render("  "+padRight(k[0], 10))+
				th.Styles.Panel.Render(k[1]))
	}
	lines = append(lines, th.Styles.Header.Render("COMMANDS"))
	lines = append(lines,
		wrapStyled(th.Styles.Panel, "  "+strings.Join(helpCommands(false), " "), innerW)...)
	lines = append(lines,
		wrapStyled(th.Styles.Panel, "  "+strings.Join(helpCommands(true), " "), innerW)...)
	lines = append(lines, th.Styles.Muted.Render("enter or esc closes · /help toggles"))

	// Clip to the box the border will add, so the overlay always occupies
	// exactly height lines.
	maxLines := height - 2
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(th.Palette[SlotBorder]).
		Width(innerW+4).
		Padding(0, 1)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		box.Render(strings.Join(lines, "\n")))
}

// wrapStyled wraps plain text to w columns and styles each resulting line.
func wrapStyled(st lipgloss.Style, s string, w int) []string {
	wrapped := strings.Split(lipgloss.Wrap(s, w, ""), "\n")
	out := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		out = append(out, st.Render(line))
	}
	return out
}

// padRight pads s with spaces to at least n display columns.
func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

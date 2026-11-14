package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// paletteRows is how many command rows the palette window shows at once;
// longer catalogues scroll inside the overlay.
const paletteRows = 12

// localAction identifies the app-side action behind a palette entry. Two
// kinds of entries live here:
//
//   - view commands, executed locally by the app and never sent over
//     Driver.Command (the character sheet, clear, help, editor, quit, the
//     channel switch, themes);
//   - actRollArg, which closes the palette and puts the input line into
//     argument mode: only its Enter (roll <expr>) reaches the session.
//
// Everything else a palette entry runs reaches the session through
// PaletteCommand.Session (Driver.Command) — see Driver's contract in
// events.go.
type localAction int

const (
	// actNone means "no local action": the entry runs over Driver.Command.
	actNone localAction = iota
	// actSheet opens the view-only character sheet overlay.
	actSheet
	// actClear resets the transcript view.
	actClear
	// actHelp opens the help overlay.
	actHelp
	// actEditor opens the input line in $VISUAL.
	actEditor
	// actQuit exits the app (state is saved by the CLI).
	actQuit
	// actChannelIC switches the input line to the in-character channel.
	actChannelIC
	// actChannelOOC switches the input line to the out-of-character channel.
	actChannelOOC
	// actTheme applies arg as the theme name.
	actTheme
	// actRollArg opens the input line's argument mode for Session.
	actRollArg
)

// PaletteCommand is one row of the command palette: what the player sees
// (Name, Description — the filter matches on these) and how the app runs it
// (Session for commands that reach the harness, local for view actions).
type PaletteCommand struct {
	// Name is the row's label, matched first: prefix > substring >
	// subsequence (case-insensitive).
	Name string
	// Description is the row's hint, matched only after the name fails
	// (substring, case-insensitive).
	Description string
	// Session names the command forwarded over Driver.Command when the
	// entry runs ("save", "recap", "roll" — the last one after its
	// argument is typed). Empty means the entry never reaches the session.
	Session string

	local localAction // app-side dispatch; actNone for pure session commands
	arg   string      // theme name for actTheme
}

// PaletteCommands builds the catalogue the palette renders, for the given
// channel. It is rebuilt every time the palette opens, so the one dynamic
// entry reflects the app's current state: only the channel the player is
// NOT on is offered ("Switch to IC" while OOC, "Switch to OOC" while IC) —
// offering the current channel would be a no-op row. All other entries are
// fixed. This list is the TUI's source of truth for command names; the
// session keeps its own dispatch switch (sessionCommands in turn.go).
func PaletteCommands(ch Channel) []PaletteCommand {
	cmds := []PaletteCommand{
		{Name: "Character sheet", Description: "full attributes, skills and equipment", local: actSheet},
		{Name: "Roll dice", Description: "throw a dice expression (e.g. 2d6+3)", Session: "roll", local: actRollArg},
		{Name: "Save session", Description: "write state, chat and log to disk", Session: "save"},
		{Name: "Session recap", Description: "what happened so far", Session: "recap"},
	}
	if ch == ChannelIC {
		cmds = append(cmds, PaletteCommand{
			Name: "Switch to OOC", Description: "in-character / out-of-character chat", local: actChannelOOC,
		})
	} else {
		cmds = append(cmds, PaletteCommand{
			Name: "Switch to IC", Description: "in-character / out-of-character chat", local: actChannelIC,
		})
	}
	cmds = append(cmds,
		PaletteCommand{Name: "Clear screen", Description: "reset the transcript view", local: actClear},
		PaletteCommand{Name: "Help", Description: "keybindings and commands", local: actHelp},
		PaletteCommand{Name: "Open editor", Description: "$VISUAL for a long message", local: actEditor},
	)
	// Themes carry their argument in the entry itself: no argument mode.
	for _, name := range Names() {
		cmds = append(cmds, PaletteCommand{
			Name: "Theme: " + name, Description: "switch the colour palette", local: actTheme, arg: name,
		})
	}
	return append(cmds, PaletteCommand{
		Name: "Quit", Description: "exit myul (state is saved)", local: actQuit,
	})
}

// paletteNames renders the catalogue's names for help output.
func paletteNames(ch Channel) []string {
	cmds := PaletteCommands(ch)
	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, c.Name)
	}
	return names
}

// matchRank scores one command against a lower-case query: name prefix (0)
// beats name substring (1) beats name subsequence (2) beats description
// substring (3); ok is false when nothing matches. An empty query matches
// everything at rank 0, so the catalogue keeps its stable order, and equal
// ranks keep catalogue order too (the caller sorts stably).
func matchRank(c PaletteCommand, query string) (rank int, ok bool) {
	if query == "" {
		return 0, true
	}
	name := strings.ToLower(c.Name)
	switch {
	case strings.HasPrefix(name, query):
		return 0, true
	case strings.Contains(name, query):
		return 1, true
	case subsequence(name, query):
		return 2, true
	}
	if strings.Contains(strings.ToLower(c.Description), query) {
		return 3, true
	}
	return 0, false
}

// subsequence reports whether every rune of query appears in s in order
// (a forgiving "type the letters in order" match).
func subsequence(s, query string) bool {
	rs := []rune(s)
	i := 0
	for _, r := range query {
		found := false
		for ; i < len(rs); i++ {
			if rs[i] == r {
				i++
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// palette is the ctrl+p command palette: a filter line over the command
// catalogue with keyboard selection. It is pure UI state — pressing keys
// only moves the selection; executing an entry is the App's job (local
// action or Driver.Command). No timers: every transition is a key press.
type palette struct {
	open    bool
	filter  textinput.Model
	entries []PaletteCommand
	matches []int // catalogue indexes passing the filter, in display order
	sel     int   // index into matches
	top     int   // first row of the visible window
}

// newPalette builds a closed palette seeded with the default catalogue
// (channel state is irrelevant until the first open).
func newPalette() palette {
	p := palette{filter: textinput.New()}
	p.filter.Prompt = "command ▸ "
	p.entries = PaletteCommands(ChannelIC)
	p.refilter()
	return p
}

// openWith (re)builds the palette over a fresh catalogue — the caller
// passes PaletteCommands(a.channel) so dynamic entries are current —
// resets the filter and the selection, and returns the focus command for
// the filter line.
func (p *palette) openWith(cmds []PaletteCommand) tea.Cmd {
	p.entries = cmds
	p.open = true
	p.filter.Reset()
	p.sel, p.top = 0, 0
	p.refilter()
	return p.filter.Focus()
}

// close shuts the palette; the next open rebuilds everything.
func (p *palette) close() { p.open = false }

// refilter recomputes the visible matches for the current query and clamps
// the selection (and window) into them.
func (p *palette) refilter() {
	query := strings.ToLower(strings.TrimSpace(p.filter.Value()))
	type hit struct {
		idx  int
		rank int
	}
	var hits []hit
	for i, c := range p.entries {
		if rank, ok := matchRank(c, query); ok {
			hits = append(hits, hit{idx: i, rank: rank})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].rank < hits[b].rank })
	p.matches = make([]int, 0, len(hits))
	for _, h := range hits {
		p.matches = append(p.matches, h.idx)
	}
	if p.sel >= len(p.matches) {
		p.sel = len(p.matches) - 1
	}
	if p.sel < 0 {
		p.sel = 0
	}
	p.scrollWindow()
}

// scrollWindow keeps the selected row inside the visible window.
func (p *palette) scrollWindow() {
	if p.sel < p.top {
		p.top = p.sel
	}
	if p.sel >= p.top+paletteRows {
		p.top = p.sel - paletteRows + 1
	}
	if p.top < 0 {
		p.top = 0
	}
}

// selected returns the highlighted command, if any row matches.
func (p *palette) selected() (PaletteCommand, bool) {
	if !p.open || p.sel < 0 || p.sel >= len(p.matches) {
		return PaletteCommand{}, false
	}
	return p.entries[p.matches[p.sel]], true
}

// paletteOutcome classifies one key press while the palette is open.
type paletteOutcome int

const (
	// outFilter means the key went to the filter line (typing).
	outFilter paletteOutcome = iota
	// outClose means esc or ctrl+p closed the palette without acting.
	outClose
	// outChosen means enter executed the selection (see selected).
	outChosen
	// outQuit means ctrl+c requested quit.
	outQuit
)

// handleKey processes one key press while the palette is open: typing
// filters, ↑/↓ move the selection with wrap-around, enter executes, esc
// closes, and ctrl+c still quits — q and every other key only ever reach
// the filter, never the app's shortcuts.
func (p *palette) handleKey(msg tea.KeyPressMsg) paletteOutcome {
	switch msg.String() {
	case "ctrl+c":
		return outQuit
	case "ctrl+p":
		// Toggles: while open, ctrl+p shuts the palette again.
		p.close()
		return outClose
	case "esc":
		p.close()
		return outClose
	case "enter":
		if _, ok := p.selected(); ok {
			return outChosen
		}
		return outFilter // nothing matches: enter does nothing
	case "up":
		if len(p.matches) > 0 {
			p.sel--
			if p.sel < 0 {
				p.sel = len(p.matches) - 1
			}
			p.scrollWindow()
		}
		return outFilter
	case "down":
		if len(p.matches) > 0 {
			p.sel++
			if p.sel >= len(p.matches) {
				p.sel = 0
			}
			p.scrollWindow()
		}
		return outFilter
	}
	ti, _ := p.filter.Update(msg)
	p.filter = ti
	p.refilter()
	return outFilter
}

// applyStyles pushes the theme's input styles into the filter line.
func (p *palette) applyStyles(th Theme) {
	st := p.filter.Styles()
	st.Focused.Prompt = th.Styles.InputPrompt
	st.Focused.Text = th.Styles.InputText
	st.Blurred.Prompt = th.Styles.Muted
	st.Blurred.Text = th.Styles.InputText
	st.Cursor.Color = th.Palette[SlotAccent]
	p.filter.SetStyles(st)
}

// view renders the overlay for a width×height area: a bordered, centred
// box (larger than the help modal) holding the filter line, the visible
// window of command rows with the selection highlighted, and a dim
// dismissal hint. Pure function of palette state and theme — tests drive
// it by injecting keys.
func (p *palette) view(th Theme, width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	innerW := width - 6 // border (2) + padding (2) + a margin on each side
	if innerW < 12 {
		innerW = 12
	}
	if innerW > 84 {
		innerW = 84
	}

	lines := []string{
		th.Styles.Highlight.Render("COMMANDS"),
		p.filter.View(),
		"",
	}
	if len(p.matches) == 0 {
		lines = append(lines, th.Styles.Muted.Render("  no commands match"))
	} else {
		end := p.top + paletteRows
		if end > len(p.matches) {
			end = len(p.matches)
		}
		for _, idx := range p.matches[p.top:end] {
			c := p.entries[idx]
			row := padRight(c.Name, 20)
			if idx == p.matches[p.sel] {
				lines = append(lines,
					th.Styles.Highlight.Render("> "+row)+
						fitDesc(th, c.Description, innerW-lipgloss.Width(row)-2))
				continue
			}
			lines = append(lines,
				th.Styles.Panel.Render("  "+row)+
					fitDesc(th, c.Description, innerW-lipgloss.Width(row)-2))
		}
	}
	lines = append(lines, th.Styles.Muted.Render("↑/↓ move · enter run · esc close"))

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

// fitDesc renders the row description clipped to the remaining columns.
func fitDesc(th Theme, desc string, w int) string {
	if w < 1 {
		return ""
	}
	return th.Styles.Muted.MaxWidth(w).Render(desc)
}

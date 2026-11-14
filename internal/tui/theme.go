// Package tui is the terminal front end for Mythras Ultra: an app shell with
// a chat transcript, an input line, themes, a command palette (ctrl+p) and
// an external editor. It renders facts it is given (events from the
// harness) and never computes game outcomes itself.
package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// NumSlots is the number of colour slots every theme defines: the project's
// 16-colour palette budget (ARCHITECTURE.md §7).
const NumSlots = 16

// Slot identifies one colour slot inside Theme.Palette. Every built-in theme
// fills all NumSlots slots; styles reference slots rather than raw colours so
// no hard-coded colour ever appears outside the theme definitions.
type Slot uint8

// The sixteen palette slots. Add new slots only by replacing an existing one
// — NumSlots stays 16.
const (
	// SlotBackground paints full-screen surfaces (terminal default when NoColor).
	SlotBackground Slot = iota
	// SlotForeground is body prose and input text.
	SlotForeground
	// SlotAccent highlights prompts and primary emphasis.
	SlotAccent
	// SlotMuted de-emphasises system notes and footer hints.
	SlotMuted
	// SlotHighlight marks the active element (segmented channel switch).
	SlotHighlight
	// SlotBorder draws panel borders.
	SlotBorder
	// SlotHeader colours the header/status line.
	SlotHeader
	// SlotFooter colours the keybinding footer.
	SlotFooter
	// SlotPanel colours the metadata panel contents.
	SlotPanel
	// SlotSpeaker colours speaker tags such as the OOC marker.
	SlotSpeaker
	// SlotTool colours tool-annotation strips.
	SlotTool
	// SlotDice colours dice banners.
	SlotDice
	// SlotCritical colours critical roll outcomes and hard errors.
	SlotCritical
	// SlotSuccess colours successful roll outcomes.
	SlotSuccess
	// SlotFailure colours failed roll outcomes.
	SlotFailure
	// SlotFumble colours fumbled roll outcomes.
	SlotFumble
)

// Styles is the set of Lip Gloss styles the app renders with. Every style is
// derived from a Theme palette slot; the theme system is the only place
// colours are chosen.
type Styles struct {
	// Narrative renders story text as plain, readable prose.
	Narrative lipgloss.Style
	// Speaker renders speaker tags (for example the OOC marker).
	Speaker lipgloss.Style
	// ToolAnnotation renders the compact one-line tool strip (gear/arrow glyph).
	ToolAnnotation lipgloss.Style
	// DiceBanner renders roll results as a bold centred banner.
	DiceBanner lipgloss.Style
	// InputPrompt renders the "> "/"ooc> " prompt.
	InputPrompt lipgloss.Style
	// InputText renders what the player is typing.
	InputText lipgloss.Style
	// Header renders the status line: session, channel, model, save state.
	Header lipgloss.Style
	// Footer renders the keybinding hints.
	Footer lipgloss.Style
	// Panel renders metadata panel contents.
	Panel lipgloss.Style
	// Border draws frame lines around panels.
	Border lipgloss.Style
	// Accent is general primary emphasis.
	Accent lipgloss.Style
	// Muted de-emphasises secondary text.
	Muted lipgloss.Style
	// Highlight marks the currently active element.
	Highlight lipgloss.Style
	// Critical marks critical outcomes and errors.
	Critical lipgloss.Style
	// Success marks successful outcomes.
	Success lipgloss.Style
	// Failure marks failed outcomes.
	Failure lipgloss.Style
	// Fumble marks fumbled outcomes.
	Fumble lipgloss.Style
	// Background paints full-screen surfaces.
	Background lipgloss.Style
	// Foreground is the default text style.
	Foreground lipgloss.Style
}

// Theme is a named 16-colour palette plus the Lip Gloss styles built from it.
//
// Palettes are authored as [light, dark] background pairs and resolved with
// lipgloss.LightDark, so a theme keeps its identity while its neutrals stay
// readable on either terminal background. Build a theme with Get and, when
// the terminal background is known, Adapt it.
type Theme struct {
	// Name is the built-in theme name ("mythras", "obsidian", …).
	Name string
	// Dark reports whether Palette was built for a dark terminal background;
	// it selects the matching glamour markdown style.
	Dark bool
	// Palette holds exactly NumSlots colours, indexed by Slot.
	Palette [NumSlots]color.Color
	// Styles are the render styles derived from Palette.
	Styles Styles
}

// themeSpec declares one built-in theme: its native background mode and its
// sixteen [light, dark] colour pairs. An empty string means "terminal
// default" (lipgloss.NoColor) — used by the terminal theme so the user's own
// palette wins.
type themeSpec struct {
	name  string
	dark  bool
	slots [NumSlots][2]string // [light-background variant, dark-background variant]
}

// defaultTheme is used when Options.Theme is empty (see New).
const defaultTheme = "mythras"

// builtins lists the built-in themes in Names() order.
var builtins = []themeSpec{
	{
		// mythras: deep warm purples and golds on a dark ground; on a light
		// terminal the neutrals flip to plum ink on warm paper while the gold
		// identity survives.
		name: "mythras",
		dark: true,
		slots: [NumSlots][2]string{
			SlotBackground: {"#f7f1e3", "#17101f"},
			SlotForeground: {"#2a1f36", "#ece4f4"},
			SlotAccent:     {"#8a5a12", "#d9a441"},
			SlotMuted:      {"#7a6a8c", "#8a7d9b"},
			SlotHighlight:  {"#6b3fa0", "#f2c971"},
			SlotBorder:     {"#d8c9ec", "#3b2b52"},
			SlotHeader:     {"#4a3568", "#cbb7e6"},
			SlotFooter:     {"#6f6284", "#7f7291"},
			SlotPanel:      {"#52406e", "#b9a8d4"},
			SlotSpeaker:    {"#7a4f0a", "#e0b354"},
			SlotTool:       {"#1f6f5f", "#7fb8a8"},
			SlotDice:       {"#6b4b12", "#f0d38a"},
			SlotCritical:   {"#7a5c00", "#ffe08a"},
			SlotSuccess:    {"#1e7d3c", "#7fd08a"},
			SlotFailure:    {"#a35a12", "#e0954f"},
			SlotFumble:     {"#b3183f", "#e05a78"},
		},
	},
	{
		// obsidian: cold blues and greys on near-black; light variant is
		// ink-blue on cool white.
		name: "obsidian",
		dark: true,
		slots: [NumSlots][2]string{
			SlotBackground: {"#f6f8fa", "#0d1117"},
			SlotForeground: {"#1f2328", "#d7e0ea"},
			SlotAccent:     {"#0969da", "#58a6ff"},
			SlotMuted:      {"#59636e", "#6b7684"},
			SlotHighlight:  {"#8250df", "#79c0ff"},
			SlotBorder:     {"#d0d7de", "#21262d"},
			SlotHeader:     {"#3b4552", "#9fb3c8"},
			SlotFooter:     {"#59636e", "#5c6670"},
			SlotPanel:      {"#3d444d", "#8ba3bb"},
			SlotSpeaker:    {"#0a6678", "#7fd1e0"},
			SlotTool:       {"#57606a", "#8b949e"},
			SlotDice:       {"#0550ae", "#a5d6ff"},
			SlotCritical:   {"#044f2e", "#7ee787"},
			SlotSuccess:    {"#1a7f37", "#3fb950"},
			SlotFailure:    {"#9a6700", "#d29922"},
			SlotFumble:     {"#cf222e", "#f85149"},
		},
	},
	{
		// parchment: ink on aged paper (light native); dark variant is the
		// same paper seen by candlelight.
		name: "parchment",
		dark: false,
		slots: [NumSlots][2]string{
			SlotBackground: {"#f4ead5", "#2a241c"},
			SlotForeground: {"#2b2320", "#e8dcc4"},
			SlotAccent:     {"#6b4a1f", "#d9a441"},
			SlotMuted:      {"#8a7a63", "#9b8a70"},
			SlotHighlight:  {"#a3541f", "#e0a95c"},
			SlotBorder:     {"#d8c9a8", "#4a3f30"},
			SlotHeader:     {"#4a3a28", "#d9c9a8"},
			SlotFooter:     {"#7a6b54", "#9b8a70"},
			SlotPanel:      {"#55452f", "#cbb893"},
			SlotSpeaker:    {"#7b2d26", "#e09080"},
			SlotTool:       {"#4f6b52", "#9fc3a0"},
			SlotDice:       {"#5b3a1a", "#e8c98a"},
			SlotCritical:   {"#8a6a00", "#e0c26a"},
			SlotSuccess:    {"#2f6b3a", "#8fd694"},
			SlotFailure:    {"#9a5a15", "#e0a05c"},
			SlotFumble:     {"#8a1e1e", "#e07a8a"},
		},
	},
	{
		// terminal: the user's own ANSI 16, so their palette wins. The
		// background stays the terminal default and only readability-critical
		// slots flip with the detected background.
		name: "terminal",
		dark: true,
		slots: [NumSlots][2]string{
			SlotBackground: {"", ""},
			SlotForeground: {"0", "7"},
			SlotAccent:     {"6", "14"},
			SlotMuted:      {"8", "8"},
			SlotHighlight:  {"5", "13"},
			SlotBorder:     {"8", "8"},
			SlotHeader:     {"7", "7"},
			SlotFooter:     {"8", "8"},
			SlotPanel:      {"7", "7"},
			SlotSpeaker:    {"5", "13"},
			SlotTool:       {"4", "12"},
			SlotDice:       {"3", "11"},
			SlotCritical:   {"6", "14"},
			SlotSuccess:    {"2", "10"},
			SlotFailure:    {"3", "11"},
			SlotFumble:     {"1", "9"},
		},
	},
}

// Names lists the built-in theme names in stable order.
func Names() []string {
	names := make([]string, 0, len(builtins))
	for _, s := range builtins {
		names = append(names, s.name)
	}
	return names
}

// Get returns the named built-in theme, built for its native background mode
// (dark for mythras, obsidian and terminal; light for parchment). Matching is
// case-insensitive and surrounding space is ignored. An unknown name returns
// an error listing every valid one.
func Get(name string) (Theme, error) {
	spec, err := lookup(name)
	if err != nil {
		return Theme{}, err
	}
	return build(spec, spec.dark), nil
}

// Adapt returns the theme rebuilt for an isDark terminal background — pass
// tea.BackgroundColorMsg.IsDark when the terminal reports it. The [light, dark]
// palette pairs are resolved with lipgloss.LightDark, so accents keep the
// theme's identity while neutrals remain readable. Themes not built in are
// returned unchanged (they cannot be re-derived).
func (t Theme) Adapt(isDark bool) Theme {
	spec, err := lookup(t.Name)
	if err != nil {
		return t
	}
	return build(spec, isDark)
}

// lookup resolves a (lower-cased, trimmed) theme name to its spec, or fails
// with an error listing the valid names.
func lookup(name string) (themeSpec, error) {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	for _, s := range builtins {
		if s.name == trimmed {
			return s, nil
		}
	}
	return themeSpec{}, fmt.Errorf("unknown theme %q — valid themes: %s",
		name, strings.Join(Names(), ", "))
}

// build materialises a spec into a Theme for the given background mode.
func build(s themeSpec, isDark bool) Theme {
	ld := lipgloss.LightDark(isDark)
	var pal [NumSlots]color.Color
	for i, pair := range s.slots {
		pal[i] = ld(lipgloss.Color(pair[0]), lipgloss.Color(pair[1]))
	}
	return Theme{
		Name:    s.name,
		Dark:    isDark,
		Palette: pal,
		Styles:  stylesFrom(pal),
	}
}

// stylesFrom derives every render style from a palette.
func stylesFrom(p [NumSlots]color.Color) Styles {
	base := lipgloss.NewStyle()
	return Styles{
		Narrative:      base.Foreground(p[SlotForeground]),
		Speaker:        base.Foreground(p[SlotSpeaker]).Bold(true),
		ToolAnnotation: base.Foreground(p[SlotTool]),
		DiceBanner:     base.Foreground(p[SlotDice]).Bold(true),
		InputPrompt:    base.Foreground(p[SlotAccent]).Bold(true),
		InputText:      base.Foreground(p[SlotForeground]),
		Header:         base.Foreground(p[SlotHeader]),
		Footer:         base.Foreground(p[SlotFooter]).Faint(true),
		Panel:          base.Foreground(p[SlotPanel]),
		Border:         base.Foreground(p[SlotBorder]),
		Accent:         base.Foreground(p[SlotAccent]).Bold(true),
		Muted:          base.Foreground(p[SlotMuted]),
		Highlight:      base.Foreground(p[SlotHighlight]).Bold(true),
		Critical:       base.Foreground(p[SlotCritical]).Bold(true),
		Success:        base.Foreground(p[SlotSuccess]),
		Failure:        base.Foreground(p[SlotFailure]),
		Fumble:         base.Foreground(p[SlotFumble]).Bold(true),
		Background:     base.Background(p[SlotBackground]).Foreground(p[SlotForeground]),
		Foreground:     base.Foreground(p[SlotForeground]),
	}
}

package world

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// CultureInfo is fiction-level guidance for one human culture: how its
// people speak and behave and what they hold dear, so the GM can role-play
// NPCs consistently. It deliberately carries no skill data — the culture
// skill pool (Standard skills, Professional picks, cultural Combat Style,
// Step 5 of character creation) is character data owned by another package
// and is not duplicated here.
type CultureInfo struct {
	// Name is the display name ("Barbarian").
	Name string
	// Description is a paragraph of background drawn from the rulebook's
	// culture section.
	Description string
	// Traits are behavioural hints — how a person of this culture comes
	// across in play.
	Traits []string
	// Values are the things this culture prizes, for motivation and
	// role-play hints.
	Values []string
}

// ErrUnknownCulture reports a culture name that does not exist in the
// world's culture map, so harness tools can translate it into a structured
// IMPOSSIBLE result for the GM.
var ErrUnknownCulture = errors.New("world: unknown culture")

// Culture returns the culture registered under name — trimmed and matched
// case-insensitively ("barbarian", "Barbarian", and " BARBARIAN " are the
// same lookup — see [DefaultCultures] for the four default keys). The
// returned struct is a copy whose Traits and Values slices are freshly
// cloned, so mutating it cannot corrupt the world. A missing culture is an
// error wrapping [ErrUnknownCulture]; worlds built with [New] seed the
// defaults, while a hand-built World may register its own under any key.
func (w *World) Culture(name string) (CultureInfo, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	c, ok := w.Cultures[key]
	if !ok {
		return CultureInfo{}, fmt.Errorf("%w %q", ErrUnknownCulture, name)
	}
	return c.clone(), nil
}

// clone returns a copy whose slices do not alias the original.
func (c CultureInfo) clone() CultureInfo {
	c.Traits = slices.Clone(c.Traits)
	c.Values = slices.Clone(c.Values)
	return c
}

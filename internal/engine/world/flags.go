package world

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Flag is a single world fact the GM consults — an economy state, a war
// state, a regional condition, a scarcity, or any free-form key the campaign
// invents. Flags are the only way world state changes (golden rule 1), and
// every transition is recorded in History.
type Flag struct {
	// Key is the flag's identifier (trimmed, case-sensitive).
	Key string
	// Value is the current state; it is free-form text. Namespaced keys
	// constrain it to a vocabulary through their typed setters.
	Value string
	// Reason is why the flag currently holds this value — the reason of
	// the most recent transition.
	Reason string
	// History is the append-only record of every transition this flag has
	// made, oldest first. It is never trimmed or rewritten.
	History []FlagChange
}

// FlagChange is one append-only record of a flag transition: what the flag
// said before, what it says now, and why the change was made. For a flag's
// creation, Old is the empty string.
type FlagChange struct {
	Key    string
	Old    string
	New    string
	Reason string
}

// Sentinel errors for flag operations, so harness tools can translate them
// into structured IMPOSSIBLE results for the GM.
var (
	// ErrUnknownFlag reports a missing flag key or an empty key.
	ErrUnknownFlag = errors.New("world: unknown flag")
	// ErrInvalidFlagValue reports a value outside the vocabulary of a
	// namespaced flag setter ([World.SetEconomyState],
	// [World.SetWarState]).
	ErrInvalidFlagValue = errors.New("world: invalid flag value")
)

// Namespaced flag keys written by the typed setters below. Free-form
// [World.SetFlag] keys remain available for one-off facts.
const (
	// FlagEconomy is the key of the world economy state; its value is one
	// of EconomyStable, EconomyStrained, EconomyCollapsed.
	FlagEconomy = "economy"
	// FlagWar is the key of the world war state; its value is one of
	// WarPeace, WarCold, WarActive.
	FlagWar = "war"
	// AspectBlockade is the region aspect consulted by
	// [World.CanAcquire]; it is written under the key
	// "region:<region>:blockade" by [World.SetRegionFlag].
	AspectBlockade = "blockade"
)

// Vocabulary of [FlagEconomy], from a stable footing to outright collapse.
// Mythras Imperative defines no price lists (digest §8), so these are
// coarse availability conditions, not economic rules.
const (
	// EconomyStable means ordinary trade; no availability strain.
	EconomyStable = "stable"
	// EconomyStrained means scarcity is biting; adds strain to
	// [World.CanAcquire].
	EconomyStrained = "strained"
	// EconomyCollapsed means trade has broken down; adds heavy strain to
	// [World.CanAcquire].
	EconomyCollapsed = "collapsed"
)

// Vocabulary of [FlagWar].
const (
	// WarPeace means no war; no availability strain.
	WarPeace = "peace"
	// WarCold means tense standoff or mobilisation; adds strain to
	// [World.CanAcquire].
	WarCold = "cold"
	// WarActive means open war; adds heavy strain to [World.CanAcquire].
	WarActive = "active"
)

// scarceFlagSuffix is the suffix of "<material>_scarce" flags consulted by
// [World.CanAcquire].
const scarceFlagSuffix = "_scarce"

// RegionFlagKey builds the namespaced flag key "region:<region>:<aspect>"
// from its parts. Region and aspect are trimmed and lower-cased so the key
// is canonical (matching is therefore case-insensitive), and both must be
// non-empty and free of ":" (the separator).
func RegionFlagKey(region, aspect string) (string, error) {
	r := strings.ToLower(strings.TrimSpace(region))
	a := strings.ToLower(strings.TrimSpace(aspect))
	if r == "" || a == "" {
		return "", fmt.Errorf("world: region flag needs both region and aspect (region %q, aspect %q)", region, aspect)
	}
	if strings.ContainsRune(r, ':') || strings.ContainsRune(a, ':') {
		return "", fmt.Errorf("world: region and aspect must not contain %q (region %q, aspect %q)", ":", region, aspect)
	}
	return "region:" + r + ":" + a, nil
}

// SetEconomyState sets [FlagEconomy] to state (trimmed and lower-cased),
// which must be one of [EconomyStable], [EconomyStrained],
// [EconomyCollapsed]; anything else is an error wrapping
// [ErrInvalidFlagValue]. reason becomes the flag's audit reason.
func (w *World) SetEconomyState(state, reason string) error {
	s := strings.ToLower(strings.TrimSpace(state))
	switch s {
	case EconomyStable, EconomyStrained, EconomyCollapsed:
	default:
		return fmt.Errorf("%w: economy state %q (valid: %s, %s, %s)",
			ErrInvalidFlagValue, state, EconomyStable, EconomyStrained, EconomyCollapsed)
	}
	return w.SetFlag(FlagEconomy, s, reason)
}

// SetWarState sets [FlagWar] to state (trimmed and lower-cased), which must
// be one of [WarPeace], [WarCold], [WarActive]; anything else is an error
// wrapping [ErrInvalidFlagValue]. reason becomes the flag's audit reason.
func (w *World) SetWarState(state, reason string) error {
	s := strings.ToLower(strings.TrimSpace(state))
	switch s {
	case WarPeace, WarCold, WarActive:
	default:
		return fmt.Errorf("%w: war state %q (valid: %s, %s, %s)",
			ErrInvalidFlagValue, state, WarPeace, WarCold, WarActive)
	}
	return w.SetFlag(FlagWar, s, reason)
}

// SetRegionFlag sets the region-namespaced flag "region:<region>:<aspect>" to
// value with the given audit reason. Region and aspect follow
// [RegionFlagKey]'s rules; a common pattern is
// SetRegionFlag("Altea", AspectBlockade, "true", reason).
func (w *World) SetRegionFlag(region, aspect, value, reason string) error {
	key, err := RegionFlagKey(region, aspect)
	if err != nil {
		return err
	}
	return w.SetFlag(key, value, reason)
}

// SetMaterialScarcity sets the "<material>_scarce" flag that
// [World.CanAcquire] consults for items mentioning that material. Material
// is trimmed and lower-cased ("Iron" → "iron_scarce"); the value is the
// canonical "true" or "false".
func (w *World) SetMaterialScarcity(material string, scarce bool, reason string) error {
	m := strings.ToLower(strings.TrimSpace(material))
	if m == "" {
		return errors.New("world: material scarcity: material must not be empty")
	}
	return w.SetFlag(m+scarceFlagSuffix, strconv.FormatBool(scarce), reason)
}

// flagActive reports whether a free-form boolean-ish flag value counts as
// "on": anything not explicitly off counts as on, so "true", "yes", and
// prose values such as "blockaded" all count, while "", "false", "0", "no",
// "off", "clear", and "none" (case-insensitively, trimmed) do not.
func flagActive(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "false", "0", "no", "off", "clear", "none":
		return false
	}
	return true
}

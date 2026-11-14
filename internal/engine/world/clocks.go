package world

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Clock is a progress clock — a segmented tension track the GM fills over
// sessions ("unrest reaches six", "the ritual completes"). Clocks are the
// world's tension model (architecture §4.4); Mythras Imperative defines no
// tension mechanic of its own, so segments, advancement reasons, and
// completion events are harness-level constructs driven only by explicit
// operations — never by wall-clock time or chance.
type Clock struct {
	// Name is the clock's display name and map key (trimmed).
	Name string
	// Segments is the clock's capacity; it is fixed at creation.
	Segments int
	// Filled is how many segments are spent, 0..Segments.
	Filled int
	// Tags are free-form labels for grouping and filtering.
	Tags []string
	// OnComplete are narration hooks that fire exactly once, when an
	// [World.AdvanceClock] fills the clock to its capacity. They are
	// deterministic event strings for the GM to narrate, not mechanical
	// effects. Set them at creation via [World.AddClock] or assign the
	// field directly before the clock fills.
	OnComplete []string
	// History is the append-only record of every advancement, oldest
	// first.
	History []ClockChange
}

// ClockChange is one append-only record of a clock advancement: segments
// filled before and after, and the reason given.
type ClockChange struct {
	Old    int
	New    int
	Reason string
}

// Sentinel errors for clock operations, so harness tools can translate them
// into structured IMPOSSIBLE results for the GM.
var (
	// ErrUnknownClock reports a clock name that does not exist.
	ErrUnknownClock = errors.New("world: unknown clock")
	// ErrClockComplete reports an advancement that would over-fill a
	// clock, or one applied to a clock that is already full.
	ErrClockComplete = errors.New("world: clock complete")
	// ErrClockExists reports an attempt to create a clock whose name is
	// already taken.
	ErrClockExists = errors.New("world: clock already exists")
)

// AddClock creates a clock with the given name and capacity (at least one
// segment), trimmed of surrounding whitespace, erroring if the name is
// empty or already taken ([ErrClockExists] wraps the duplicate-name
// refusal). The optional onComplete strings become the
// clock's [Clock.OnComplete] narration hooks; they are copied, so the
// caller's slice may be reused freely. Tags and further customisation are
// set by assigning the exported fields on the created clock.
func (w *World) AddClock(name string, segments int, onComplete ...string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("world: add clock: name must not be empty")
	}
	if segments < 1 {
		return fmt.Errorf("world: add clock %q: segments must be at least 1 (got %d)", name, segments)
	}
	if w.Clocks == nil {
		w.Clocks = make(map[string]*Clock)
	}
	if _, exists := w.Clocks[name]; exists {
		return fmt.Errorf("%w: %q", ErrClockExists, name)
	}
	w.Clocks[name] = &Clock{
		Name:       name,
		Segments:   segments,
		OnComplete: slices.Clone(onComplete),
	}
	return nil
}

// AdvanceClock fills the named clock by segments, recording reason, and
// returns the clock's [Clock.OnComplete] narration hooks when — and only
// when — this advancement fills it exactly to capacity. It returns nil
// events for every non-completing advancement. The returned slice is a
// copy; mutating it does not affect the clock.
//
// Validation: the clock must exist ([ErrUnknownClock], looked up by trimmed
// name), segments must be at least 1, reason must not be empty, and the
// advancement must not over-fill the clock — neither an overshoot nor an
// advancement on an already-full clock is accepted ([ErrClockComplete]);
// over-advancing never mutates state. On any error the clock is unchanged.
func (w *World) AdvanceClock(name string, segments int, reason string) ([]string, error) {
	name = strings.TrimSpace(name)
	c, ok := w.Clocks[name]
	if !ok || c == nil {
		return nil, fmt.Errorf("%w %q", ErrUnknownClock, name)
	}
	if segments < 1 {
		return nil, fmt.Errorf("world: advance clock %q: segments must be at least 1 (got %d)", name, segments)
	}
	if strings.TrimSpace(reason) == "" {
		return nil, fmt.Errorf("world: advance clock %q: reason must not be empty", name)
	}
	if c.Filled >= c.Segments {
		return nil, fmt.Errorf("%w: clock %q is already full (%d/%d)",
			ErrClockComplete, name, c.Filled, c.Segments)
	}
	if c.Filled+segments > c.Segments {
		return nil, fmt.Errorf("%w: advancing clock %q by %d would exceed its %d segments",
			ErrClockComplete, name, segments, c.Segments)
	}
	old := c.Filled
	c.Filled += segments
	c.History = append(c.History, ClockChange{Old: old, New: c.Filled, Reason: reason})
	if c.Filled == c.Segments {
		return slices.Clone(c.OnComplete), nil
	}
	return nil, nil
}

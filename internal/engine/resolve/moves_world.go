package resolve

import (
	"fmt"
	"slices"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/engine/world"
)

// SetWorldFlag writes one world fact (tool: world_set_flag). Namespaced
// keys dispatch to the world package's typed setters — economy, war,
// regional aspects and material scarcity — so their value vocabularies are
// validated; every other key is a free-form fact with append-only history
// (world.SetFlag). Mythras Imperative defines no economy or war rules
// (digest §8): flags are the harness's coarse availability model.
type SetWorldFlag struct {
	// Key is the flag key ("economy", "region:<region>:<aspect>",
	// "<material>_scarce", or any free-form key).
	Key string `json:"key"`
	// Value is the flag's new state.
	Value string `json:"value"`
	// Reason is why the flag now holds that value; recorded in the
	// append-only history.
	Reason string `json:"reason,omitempty"`
}

// Kind implements [Move].
func (m SetWorldFlag) Kind() string { return "world_set_flag" }

// Apply validates the key, records the previous value, dispatches to the
// typed setter when the key is namespaced (unknown vocabulary values error
// wrapping world.ErrInvalidFlagValue before anything changes), then reports
// old → new.
func (m SetWorldFlag) Apply(s *State) (Result, error) {
	if s.World == nil {
		return Result{}, ErrNoWorld
	}
	key := strings.TrimSpace(m.Key)
	if key == "" {
		return Result{}, fmt.Errorf("%w: empty flag key", ErrInvalidArgument)
	}
	w := s.World
	old, had := "", false
	if f, ok := w.Lookup(key); ok {
		old, had = f.Value, true
	}
	before := "(unset)"
	if had {
		before = old
	}

	if err := dispatchFlag(w, key, m.Value, m.Reason); err != nil {
		return Result{}, err
	}
	f, _ := w.Lookup(key)
	line := change(key, before, f.Value)
	facts := []string{line}
	if m.Reason != "" {
		facts = append(facts, "reason: "+m.Reason)
	}
	if len(f.History) > 0 {
		facts = append(facts, fmt.Sprintf("transitions recorded: %d", len(f.History)))
	}
	a := Audit{Changes: []string{line}}
	summary := fmt.Sprintf("flag %s: %s", key, line)
	return newResult(summary, facts, a), nil
}

// dispatchFlag routes a flag write to the typed world setter for
// namespaced keys, falling back to the generic SetFlag for free-form keys.
func dispatchFlag(w *world.World, key, value, reason string) error {
	switch {
	case key == world.FlagEconomy:
		return w.SetEconomyState(value, reason)
	case key == world.FlagWar:
		return w.SetWarState(value, reason)
	case strings.HasPrefix(key, "region:"):
		parts := strings.Split(key, ":")
		if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
			return fmt.Errorf("%w: region flag keys must be region:<region>:<aspect> (got %q)", ErrInvalidArgument, key)
		}
		return w.SetRegionFlag(parts[1], parts[2], value, reason)
	case strings.HasSuffix(key, "_scarce"):
		material := strings.TrimSuffix(key, "_scarce")
		if material == "" {
			return fmt.Errorf("%w: scarcity keys must be <material>_scarce (got %q)", ErrInvalidArgument, key)
		}
		scarce, err := parseBool(value)
		if err != nil {
			return err
		}
		return w.SetMaterialScarcity(material, scarce, reason)
	default:
		return w.SetFlag(key, value, reason)
	}
}

// parseBool maps the flag vocabulary for material-scarcity values onto the
// typed setter's bool; anything outside the vocabulary errors wrapping
// world.ErrInvalidFlagValue (the same classification the typed economy and
// war setters produce).
func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "y", "1", "on", "scarce":
		return true, nil
	case "false", "no", "n", "0", "off", "clear", "none", "abundant", "plentiful":
		return false, nil
	}
	return false, fmt.Errorf("%w: scarcity value %q (use true/false)", world.ErrInvalidFlagValue, value)
}

// AddClock creates a progress clock — the world's tension model — for
// advance_clock to fill over the sessions that follow (tool: add_clock;
// world.AddClock).
type AddClock struct {
	// Name is the clock's display name (required, trimmed).
	Name string `json:"name"`
	// Segments is the clock's capacity (positive), fixed at creation.
	Segments int `json:"segments"`
	// OnComplete are narration hooks that fire once, when a later
	// [AdvanceClock] fills the clock to capacity.
	OnComplete []string `json:"on_complete,omitempty"`
}

// Kind implements [Move].
func (m AddClock) Kind() string { return "add_clock" }

// Apply validates the world, name and capacity before running
// world.AddClock, whose duplicate-name refusal wraps world.ErrClockExists;
// on any error nothing has changed. Facts report the fresh clock's
// after-state.
func (m AddClock) Apply(s *State) (Result, error) {
	if s.World == nil {
		return Result{}, ErrNoWorld
	}
	name := strings.TrimSpace(m.Name)
	if name == "" {
		return Result{}, fmt.Errorf("%w: empty clock name", ErrInvalidArgument)
	}
	if m.Segments < 1 {
		return Result{}, fmt.Errorf("%w: segments must be at least 1 (got %d)", ErrInvalidArgument, m.Segments)
	}
	w := s.World
	if err := w.AddClock(name, m.Segments, m.OnComplete...); err != nil {
		return Result{}, err
	}
	clock := w.Clocks[name]
	line := fmt.Sprintf("clock %s: (none) → 0 of %d segments", name, clock.Segments)
	facts := []string{line}
	for _, ev := range clock.OnComplete {
		facts = append(facts, "on complete: "+ev)
	}
	facts = append(facts, fmt.Sprintf("clocks held: %d", len(w.Clocks)))
	summary := fmt.Sprintf("clock '%s' created (%d segments)", name, m.Segments)
	return newResult(summary, facts, Audit{Changes: []string{line}}), nil
}

// AdvanceClock fills segments of a progress clock — the world's tension
// model — and reports what the fill triggered (tool: advance_clock;
// world.AdvanceClock).
type AdvanceClock struct {
	// Clock is the clock's name.
	Clock string `json:"clock"`
	// Segments is how many segments to fill (positive).
	Segments int `json:"segments"`
	// Reason is why the clock advances; recorded in its history.
	Reason string `json:"reason,omitempty"`
}

// Kind implements [Move].
func (m AdvanceClock) Kind() string { return "advance_clock" }

// Apply validates the world and clock name, snapshots the filled count,
// then runs world.AdvanceClock. Errors wrap world.ErrUnknownClock or
// world.ErrClockComplete; completion events come back as facts.
func (m AdvanceClock) Apply(s *State) (Result, error) {
	if s.World == nil {
		return Result{}, ErrNoWorld
	}
	if strings.TrimSpace(m.Clock) == "" {
		return Result{}, fmt.Errorf("%w: empty clock name", ErrInvalidArgument)
	}
	w := s.World
	clock, ok := w.Clocks[m.Clock]
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", world.ErrUnknownClock, m.Clock)
	}
	before := clock.Filled
	events, err := w.AdvanceClock(m.Clock, m.Segments, m.Reason)
	if err != nil {
		return Result{}, err
	}
	after := clock.Filled
	line := fmt.Sprintf("clock %s: %d → %d of %d segments", m.Clock, before, after, clock.Segments)
	facts := []string{line}
	if m.Reason != "" {
		facts = append(facts, "reason: "+m.Reason)
	}
	for _, ev := range events {
		facts = append(facts, "triggered: "+ev)
	}
	a := Audit{Changes: []string{line}}
	summary := fmt.Sprintf("clock %s: %d → %d of %d", m.Clock, before, after, clock.Segments)
	return newResult(summary, facts, a), nil
}

// AdvanceTime moves the in-game calendar forward (tool: advance_time;
// world.Calendar.Advance). The neutral 12-month, 30-day calendar is a
// harness construct — Mythras Imperative defines no calendar (digest §8) —
// and season changes come back as facts.
type AdvanceTime struct {
	// Days advances the calendar by whole days (>= 0).
	Days int `json:"days"`
	// Hours advances it by hours (>= 0); 24 hours make one day.
	Hours int `json:"hours"`
}

// Kind implements [Move].
func (m AdvanceTime) Kind() string { return "advance_time" }

// Apply refuses a zero duration up front, snapshots the date, then runs
// world.Calendar.Advance, which validates negatives and calendar integrity
// before mutating anything.
func (m AdvanceTime) Apply(s *State) (Result, error) {
	if s.World == nil {
		return Result{}, ErrNoWorld
	}
	if m.Days == 0 && m.Hours == 0 {
		return Result{}, fmt.Errorf("%w: zero duration (days 0, hours 0)", ErrInvalidArgument)
	}
	w := s.World
	before := formatCalendar(w.Calendar)
	events, err := w.Calendar.Advance(world.Duration{Days: m.Days, Hours: m.Hours})
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	after := formatCalendar(w.Calendar)
	line := change("date", before, after)
	facts := []string{line}
	for _, ev := range events {
		facts = append(facts, "event: "+ev)
	}
	a := Audit{Changes: []string{line}}
	summary := "time passes: " + after
	return newResult(summary, facts, a), nil
}

// formatCalendar renders a world date for facts and diffs.
func formatCalendar(c world.Calendar) string {
	return fmt.Sprintf("Year %d, Month %d, Day %d (%s, %s)", c.Year, c.Month, c.Day, c.Season, c.TimeOfDay)
}

// SetScene establishes the narrative frame: where play happens, what it
// looks like and who or what is present (tool: set_scene). Present entries
// are free-text labels, not validated references — scenes routinely name
// people who are not registered characters.
type SetScene struct {
	// Location is where play happens (required).
	Location string `json:"location"`
	// Description is the scene-setting text.
	Description string `json:"description,omitempty"`
	// Present lists who or what is on stage (free text, typically
	// character IDs for combatants).
	Present []string `json:"present,omitempty"`
}

// Kind implements [Move].
func (m SetScene) Kind() string { return "set_scene" }

// Apply validates the location and replaces the scene wholesale, reporting
// the previous location and who is present now.
func (m SetScene) Apply(s *State) (Result, error) {
	if strings.TrimSpace(m.Location) == "" {
		return Result{}, fmt.Errorf("%w: empty location", ErrInvalidArgument)
	}
	before := orNone(s.Scene.Location)
	s.Scene = Scene{
		Location:    m.Location,
		Description: m.Description,
		Present:     slices.Clone(m.Present),
	}
	facts := []string{
		change("location", before, m.Location),
	}
	if m.Description != "" {
		facts = append(facts, "description: "+m.Description)
	}
	if len(m.Present) > 0 {
		facts = append(facts, "present: "+strings.Join(m.Present, ", "))
	} else {
		facts = append(facts, "present: (none)")
	}
	line := change("scene location", before, m.Location)
	summary := "scene: " + m.Location
	return newResult(summary, facts, Audit{Changes: []string{line}}), nil
}

// Note stores a persistent GM annotation on the world (tool: note;
// world.AddNote): append-only, with an engine-generated "note-N" ID.
type Note struct {
	// Topic groups the note ("clues", "NPC motives"); required.
	Topic string `json:"topic"`
	// Text is the note's body; required.
	Text string `json:"text"`
	// Tags are free-form labels for grouping.
	Tags []string `json:"tags,omitempty"`
}

// Kind implements [Move].
func (m Note) Kind() string { return "note" }

// Apply validates the required fields, appends the note through
// world.AddNote and reports its assigned ID.
func (m Note) Apply(s *State) (Result, error) {
	if s.World == nil {
		return Result{}, ErrNoWorld
	}
	if strings.TrimSpace(m.Topic) == "" {
		return Result{}, fmt.Errorf("%w: empty note topic", ErrInvalidArgument)
	}
	if strings.TrimSpace(m.Text) == "" {
		return Result{}, fmt.Errorf("%w: empty note text", ErrInvalidArgument)
	}
	n, err := s.World.AddNote(m.Topic, m.Text, m.Tags...)
	if err != nil {
		return Result{}, err
	}
	facts := []string{
		"note id: " + n.ID,
		"topic: " + n.Topic,
		fmt.Sprintf("notes held: %d", len(s.World.Notes)),
	}
	changes := []string{fmt.Sprintf("notes: +%s (%s)", n.ID, n.Topic)}
	summary := fmt.Sprintf("%s recorded (%s)", n.ID, n.Topic)
	return newResult(summary, facts, Audit{Changes: changes}), nil
}

// EndRecap stores an end-of-session summary for context rebuilds (tool:
// end_session_recap; session-loop.md, context compaction).
type EndRecap struct {
	// Summary is the recap text; required.
	Summary string `json:"summary"`
}

// Kind implements [Move].
func (m EndRecap) Kind() string { return "end_session_recap" }

// Apply appends the recap to State.Recaps (oldest first) and reports the
// stored count. Recaps are read back by the SessionRecap query.
func (m EndRecap) Apply(s *State) (Result, error) {
	if strings.TrimSpace(m.Summary) == "" {
		return Result{}, fmt.Errorf("%w: empty recap summary", ErrInvalidArgument)
	}
	before := len(s.Recaps)
	s.Recaps = append(s.Recaps, m.Summary)
	after := len(s.Recaps)
	line := change("recaps", before, after)
	facts := []string{line, "recap stored: " + m.Summary}
	summary := fmt.Sprintf("recap %d stored", after)
	return newResult(summary, facts, Audit{Changes: []string{line}}), nil
}

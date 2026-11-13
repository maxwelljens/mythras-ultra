// Package world models the persistent state of the game world the Games
// Master consults when adjudicating availability and consequences: world
// flags, progress clocks, the in-game calendar, cultures, factions,
// locations, and GM notes.
//
// Determinism contract: nothing in this package reads the wall clock or any
// source of randomness. Every operation is a pure function of the [World]
// value and its arguments, so the same sequence of operations always yields
// the same state and the same [World.Summary].
//
// Mythras Imperative deliberately defines no price lists and no encumbrance
// rules (digest §8: "Money/equipment: narrative only"; "Encumbrance: no
// rules"), so the economics here are a coarse flag abstraction that exists
// only to answer availability questions deterministically — never to price
// goods. Tension is modelled by progress clocks (see [World.AddClock]);
// there is no separate numeric tension field.
package world

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// World is the complete, JSON-round-trippable state of a campaign's
// setting. Construct one with [New] or by unmarshalling a persisted state.
// The mutator methods initialise their own maps; direct writes to Factions
// and Locations require the non-nil maps that [New] and a normal JSON
// round-trip provide.
type World struct {
	// Name is what the campaign calls this world.
	Name string
	// Setting is a one-line description of the genre or place (e.g.
	// "Iron Age Britain, with ghosts").
	Setting string
	// Cultures maps lower-case culture key to fiction-level guidance
	// (see [DefaultCultures]).
	Cultures map[string]CultureInfo
	// Factions maps faction name to its record. Factions are referenced
	// elsewhere only by name.
	Factions map[string]*Faction
	// Flags maps flag key to the flag and its append-only history.
	Flags map[string]Flag
	// Clocks maps clock name to its progress clock.
	Clocks map[string]*Clock
	// Locations maps location name to its record.
	Locations map[string]*Location
	// Calendar is the current in-game date and time of day.
	Calendar Calendar
	// Notes are append-only GM annotations in creation order.
	Notes []Note
}

// Faction is a group with standing in the campaign — a court, a guild, a
// tribe. Standing beyond the recorded attitude is tracked as world flags.
type Faction struct {
	// Name is the faction's display name and map key.
	Name string
	// Description is a one-line sketch for the GM.
	Description string
	// Attitude is the faction's current standing toward the player
	// characters (e.g. "friendly", "wary", "hostile").
	Attitude string
	// Tags are free-form labels for grouping and filtering.
	Tags []string
}

// Location is a place where scenes happen.
type Location struct {
	// Name is the location's display name and map key.
	Name string
	// Description is a one-line sketch for the GM.
	Description string
	// Region groups locations under a shared region name, which is what
	// regional flags ("region:<region>:<aspect>") are keyed on.
	Region string
	// Tags are free-form labels for grouping and filtering.
	Tags []string
}

// Note is a persistent GM annotation. IDs are assigned in append order by
// [World.AddNote] and are never reused.
type Note struct {
	// ID is the deterministic identifier ("note-1", "note-2", ...).
	ID string
	// Topic groups the note (e.g. "clues", "NPC motives").
	Topic string
	// Text is the note's body.
	Text string
	// Tags are free-form labels for grouping and filtering.
	Tags []string
}

// New returns a World ready for play: all maps non-nil, the four default
// human cultures seeded (see [DefaultCultures]), no flags, no clocks, no
// notes, and the calendar set to Year 1, Month 1, Day 1, Winter, 08:00.
func New(name, setting string) *World {
	return &World{
		Name:      name,
		Setting:   setting,
		Cultures:  DefaultCultures(),
		Factions:  map[string]*Faction{},
		Flags:     map[string]Flag{},
		Clocks:    map[string]*Clock{},
		Locations: map[string]*Location{},
		Calendar: Calendar{
			Year:      1,
			Month:     1,
			Day:       1,
			Season:    seasonForMonth(1),
			TimeOfDay: timeOfDayByHour[8],
		},
	}
}

// Get returns the current value of the flag named key. Keys are trimmed of
// surrounding whitespace and are otherwise matched exactly (case-
// sensitive); typed setters such as [World.SetEconomyState] normalise their
// keys for you. If the flag does not exist the returned error wraps
// [ErrUnknownFlag].
func (w *World) Get(key string) (string, error) {
	f, ok := w.Lookup(key)
	if !ok {
		return "", fmt.Errorf("%w %q", ErrUnknownFlag, strings.TrimSpace(key))
	}
	return f.Value, nil
}

// Lookup returns the full flag record for key, including its append-only
// history, and whether the flag exists. It reports absence as a bool rather
// than an error; use [World.Get] when a missing flag is exceptional.
func (w *World) Lookup(key string) (Flag, bool) {
	f, ok := w.Flags[strings.TrimSpace(key)]
	return f, ok
}

// SetFlag creates or updates the flag named key, recording why. On every
// actual change an entry is appended to the flag's History holding the old
// and new values together with reason; setting a flag to the value it
// already holds is a no-op and leaves the history untouched, so history is
// strictly append-only and a pure function of the operation sequence.
// Flags are never deleted: clearing a fact is a transition to the empty
// value, which still records a history entry. The key is trimmed and must
// be non-empty; reason must not be empty (it is the audit trail). An empty
// key is an error wrapping [ErrUnknownFlag].
//
// Free-form keys are allowed and encouraged for one-off facts
// ("prophecy:fulfilled"); namespaced keys with typed wrappers live in
// flags.go ([World.SetEconomyState], [World.SetWarState],
// [World.SetRegionFlag], [World.SetMaterialScarcity]).
func (w *World) SetFlag(key, value, reason string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: key must not be empty", ErrUnknownFlag)
	}
	if strings.TrimSpace(reason) == "" {
		return errors.New("world: set flag: reason must not be empty")
	}
	if w.Flags == nil {
		w.Flags = make(map[string]Flag)
	}
	f, existed := w.Flags[key]
	if existed && f.Value == value {
		return nil // no transition: history stays append-only and gap-free
	}
	f.History = append(f.History, FlagChange{
		Key:    key,
		Old:    f.Value, // empty for a newly created flag
		New:    value,
		Reason: reason,
	})
	f.Key = key
	f.Value = value
	f.Reason = reason
	w.Flags[key] = f
	return nil
}

// AddNote appends a GM note to the world and returns it with its
// deterministic ID ("note-1", "note-2", ... in append order). Topic and
// text are required; tags may be empty.
func (w *World) AddNote(topic, text string, tags ...string) (Note, error) {
	if strings.TrimSpace(topic) == "" {
		return Note{}, errors.New("world: add note: topic must not be empty")
	}
	if strings.TrimSpace(text) == "" {
		return Note{}, errors.New("world: add note: text must not be empty")
	}
	n := Note{
		ID:    "note-" + strconv.Itoa(len(w.Notes)+1),
		Topic: topic,
		Text:  text,
		Tags:  slices.Clone(tags),
	}
	w.Notes = append(w.Notes, n)
	return n, nil
}

// Summary renders the world as a compact structured text block for the GM:
// identity and date first, then every culture, flag, clock, faction,
// location, and note. Output is deterministic — map keys are sorted, notes
// appear in append order — so identical states always produce byte-
// identical summaries, and empty sections are omitted.
func (w *World) Summary() string {
	lines := []string{
		"World: " + w.Name,
		"Setting: " + w.Setting,
		fmt.Sprintf("Date: Year %d, Month %d, Day %d (%s, %s)",
			w.Calendar.Year, w.Calendar.Month, w.Calendar.Day,
			w.Calendar.Season, w.Calendar.TimeOfDay),
	}
	if n := len(w.Cultures); n > 0 {
		keys := slices.Sorted(maps.Keys(w.Cultures))
		lines = append(lines, fmt.Sprintf("Cultures (%d): %s", n, strings.Join(keys, ", ")))
	}
	if n := len(w.Flags); n > 0 {
		lines = append(lines, fmt.Sprintf("Flags (%d):", n))
		for _, k := range slices.Sorted(maps.Keys(w.Flags)) {
			f := w.Flags[k]
			line := "  " + k + " = " + f.Value
			if f.Reason != "" {
				line += " (reason: " + f.Reason + ")"
			}
			lines = append(lines, line)
		}
	}
	if n := len(w.Clocks); n > 0 {
		lines = append(lines, fmt.Sprintf("Clocks (%d):", n))
		for _, k := range slices.Sorted(maps.Keys(w.Clocks)) {
			c := w.Clocks[k]
			if c == nil {
				lines = append(lines, "  "+k+" (no clock)")
				continue
			}
			line := fmt.Sprintf("  [%d/%d] %s", c.Filled, c.Segments, c.Name)
			if len(c.Tags) > 0 {
				line += " (" + strings.Join(c.Tags, ", ") + ")"
			}
			lines = append(lines, line)
		}
	}
	if n := len(w.Factions); n > 0 {
		lines = append(lines, fmt.Sprintf("Factions (%d):", n))
		for _, k := range slices.Sorted(maps.Keys(w.Factions)) {
			f := w.Factions[k]
			if f == nil {
				lines = append(lines, "  "+k+" (no faction)")
				continue
			}
			line := "  " + k
			if f.Attitude != "" {
				line += " - attitude: " + f.Attitude
			}
			lines = append(lines, line)
		}
	}
	if n := len(w.Locations); n > 0 {
		lines = append(lines, fmt.Sprintf("Locations (%d):", n))
		for _, k := range slices.Sorted(maps.Keys(w.Locations)) {
			l := w.Locations[k]
			if l == nil {
				lines = append(lines, "  "+k+" (no location)")
				continue
			}
			line := "  " + k
			if l.Region != "" {
				line += " (region: " + l.Region + ")"
			}
			lines = append(lines, line)
		}
	}
	if n := len(w.Notes); n > 0 {
		lines = append(lines, fmt.Sprintf("Notes (%d):", n))
		for _, n := range w.Notes {
			line := "  " + n.ID + " " + n.Topic + ": " + n.Text
			if len(n.Tags) > 0 {
				line += " [" + strings.Join(n.Tags, ", ") + "]"
			}
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

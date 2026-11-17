package resolve

import (
	"errors"
	"strings"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/world"
)

// TestSetWorldFlag covers the generic path (old → new with history) and the
// typed dispatchers — economy, region, scarcity — with their vocabularies.
func TestSetWorldFlag(t *testing.T) {
	s := newState(t, 1)

	res, err := Apply(s, SetWorldFlag{Key: "omen", Value: "comet", Reason: "seen over the estuary"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(strings.Join(res.Facts, "\n"), "omen: (unset) → comet") {
		t.Errorf("facts = %v, want the unset → comet delta", res.Facts)
	}
	if res.Audit == nil || len(res.Audit.Changes) != 1 {
		t.Errorf("audit = %+v, want one change", res.Audit)
	}
	if _, err := Apply(s, SetWorldFlag{Key: "omen", Value: "raven", Reason: "seen again"}); err != nil {
		t.Fatalf("second set: %v", err)
	}
	f, ok := s.World.Lookup("omen")
	if !ok || f.Value != "raven" || f.History[0].New != "comet" || f.Reason == "" {
		t.Errorf("flag = %+v, want history comet → raven", f)
	}

	// Typed dispatch: economy uses its printed vocabulary.
	if _, err := Apply(s, SetWorldFlag{Key: world.FlagEconomy, Value: "strained", Reason: "war economy"}); err != nil {
		t.Fatalf("economy: %v", err)
	}
	if v, _ := s.World.Get(world.FlagEconomy); v != world.EconomyStrained {
		t.Errorf("economy = %q, want %q", v, world.EconomyStrained)
	}
	if _, err := Apply(s, SetWorldFlag{Key: world.FlagEconomy, Value: "thriving", Reason: "bad test"}); !errors.Is(err, world.ErrInvalidFlagValue) {
		t.Errorf("bad economy value error = %v, want world.ErrInvalidFlagValue", err)
	}

	// Regional and scarcity keys keep their parsers strict.
	if _, err := Apply(s, SetWorldFlag{Key: "region:essex:blockade", Value: "true", Reason: "the war reaches the coast"}); err != nil {
		t.Fatalf("region: %v", err)
	}
	if _, err := Apply(s, SetWorldFlag{Key: "region:essex", Value: "true", Reason: "malformed on purpose"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("malformed region key error = %v, want ErrInvalidArgument", err)
	}
	if _, err := Apply(s, SetWorldFlag{Key: "iron_scarce", Value: "yes", Reason: "the mines fell silent"}); err != nil {
		t.Fatalf("scarcity: %v", err)
	}
	if v, _ := s.World.Get("iron_scarce"); v != "true" {
		t.Errorf("iron_scarce = %q, want canonical true", v)
	}
	if _, err := Apply(s, SetWorldFlag{Key: "iron_scarce", Value: "sometimes", Reason: "nonsense value"}); !errors.Is(err, world.ErrInvalidFlagValue) {
		t.Errorf("bad scarcity value error = %v, want world.ErrInvalidFlagValue", err)
	}

	if _, err := Apply(s, SetWorldFlag{Key: " ", Value: "x", Reason: "empty key"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty key error = %v, want ErrInvalidArgument", err)
	}
	bare := &State{}
	if _, err := Apply(bare, SetWorldFlag{Key: "a", Value: "b"}); !errors.Is(err, ErrNoWorld) {
		t.Errorf("no-world error = %v, want ErrNoWorld", err)
	}
}

// TestAddClock covers clock creation: after-state facts and audit, the
// argument guards, and the duplicate refusal — every refusal leaves the
// clock set untouched.
func TestAddClock(t *testing.T) {
	s := newState(t, 1)
	res, err := Apply(s, AddClock{
		Name: "  Unrest  ", Segments: 6, OnComplete: []string{"the mob storms the granary"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Summary != "clock 'Unrest' created (6 segments)" {
		t.Errorf("summary = %q, want the creation line", res.Summary)
	}
	joined := strings.Join(res.Facts, "\n")
	for _, want := range []string{
		"clock Unrest: (none) → 0 of 6 segments",
		"on complete: the mob storms the granary",
		"clocks held: 1",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("facts = %v, want a %q line", res.Facts, want)
		}
	}
	if res.Audit == nil || len(res.Audit.Changes) != 1 {
		t.Errorf("audit = %+v, want one change", res.Audit)
	}
	clock := s.World.Clocks["Unrest"]
	if clock == nil || clock.Segments != 6 || clock.Filled != 0 || len(clock.OnComplete) != 1 {
		t.Fatalf("clock = %+v, want a fresh 6-segment clock", clock)
	}

	// The created clock fills through the normal move.
	if _, err := Apply(s, AdvanceClock{Clock: "Unrest", Segments: 2, Reason: "blockade rumours"}); err != nil {
		t.Fatalf("advance after create: %v", err)
	}

	// Argument guards: no mutation, existing clocks survive.
	for _, tc := range []struct {
		name string
		move AddClock
	}{
		{"empty name", AddClock{Name: " ", Segments: 4}},
		{"zero segments", AddClock{Name: "doom", Segments: 0}},
		{"negative segments", AddClock{Name: "doom", Segments: -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Apply(s, tc.move); !errors.Is(err, ErrInvalidArgument) {
				t.Errorf("error = %v, want ErrInvalidArgument", err)
			}
			if len(s.World.Clocks) != 1 || s.World.Clocks["Unrest"].Filled != 2 {
				t.Errorf("refused add mutated clocks: %+v", s.World.Clocks)
			}
		})
	}

	// A duplicate name refuses through world's sentinel, original intact.
	if _, err := Apply(s, AddClock{Name: "Unrest", Segments: 4}); !errors.Is(err, world.ErrClockExists) {
		t.Errorf("duplicate error = %v, want world.ErrClockExists", err)
	}
	if len(s.World.Clocks) != 1 || s.World.Clocks["Unrest"].Segments != 6 {
		t.Errorf("duplicate add mutated clocks: %+v", s.World.Clocks)
	}

	bare := &State{}
	if _, err := Apply(bare, AddClock{Name: "doom", Segments: 1}); !errors.Is(err, ErrNoWorld) {
		t.Errorf("no-world error = %v, want ErrNoWorld", err)
	}
}

// TestAdvanceClock covers segment fills, completion events and the clock
// sentinels.
func TestAdvanceClock(t *testing.T) {
	s := newState(t, 1)
	if err := s.World.AddClock("Unrest", 4, "the mob storms the gate"); err != nil {
		t.Fatalf("AddClock: %v", err)
	}
	res, err := Apply(s, AdvanceClock{Clock: "Unrest", Segments: 2, Reason: "grain ships burn"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "clock Unrest: 0 → 2 of 4 segments") {
		t.Errorf("facts = %v", res.Facts)
	}
	if s.World.Clocks["Unrest"].Filled != 2 {
		t.Errorf("Filled = %d, want 2", s.World.Clocks["Unrest"].Filled)
	}

	// Fill to the brim: completion events surface as facts.
	full, err := Apply(s, AdvanceClock{Clock: "Unrest", Segments: 2, Reason: "the last loaf is eaten"})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if !strings.Contains(strings.Join(full.Facts, "\n"), "triggered: the mob storms the gate") {
		t.Errorf("facts = %v, want the completion event", full.Facts)
	}
	if _, err := Apply(s, AdvanceClock{Clock: "Unrest", Segments: 1, Reason: "one more push"}); !errors.Is(err, world.ErrClockComplete) {
		t.Errorf("overfill error = %v, want world.ErrClockComplete", err)
	}
	if _, err := Apply(s, AdvanceClock{Clock: "Doom", Segments: 1, Reason: "doom"}); !errors.Is(err, world.ErrUnknownClock) {
		t.Errorf("unknown clock error = %v, want world.ErrUnknownClock", err)
	}
	if _, err := Apply(s, AdvanceClock{Clock: " ", Segments: 1, Reason: "empty name"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty clock error = %v, want ErrInvalidArgument", err)
	}
}

// TestAdvanceTime covers calendar movement, season/date facts and the
// refusal paths.
func TestAdvanceTime(t *testing.T) {
	s := newState(t, 1)
	res, err := Apply(s, AdvanceTime{Days: 32})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "date: Year 1, Month 1, Day 1") || !strings.Contains(joined, "Month 2") {
		t.Errorf("facts = %v, want the before → after date", res.Facts)
	}
	c := s.World.Calendar
	if c.Month != 2 || c.Day != 3 || c.Year != 1 {
		t.Errorf("calendar = %+v, want Year 1 Month 2 Day 3", c)
	}

	if _, err := Apply(s, AdvanceTime{}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("zero duration error = %v, want ErrInvalidArgument", err)
	}
	if _, err := Apply(s, AdvanceTime{Days: -1}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("negative duration error = %v, want ErrInvalidArgument", err)
	}
	bare := &State{}
	if _, err := Apply(bare, AdvanceTime{Hours: 1}); !errors.Is(err, ErrNoWorld) {
		t.Errorf("no-world error = %v, want ErrNoWorld", err)
	}
}

// TestSetScene covers the scene frame replacement and its guard.
func TestSetScene(t *testing.T) {
	s := newState(t, 1)
	res, err := Apply(s, SetScene{
		Location: "The Broken Causeway", Description: "Rain on black stone.",
		Present: []string{"pc-1", "the ferryman"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if s.Scene.Location != "The Broken Causeway" || len(s.Scene.Present) != 2 {
		t.Errorf("scene = %+v", s.Scene)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "location: (none) → The Broken Causeway") ||
		!strings.Contains(joined, "present: pc-1, the ferryman") ||
		!strings.Contains(joined, "Rain on black stone.") {
		t.Errorf("facts = %v", res.Facts)
	}
	// Present is free text: unregistered labels are legal.
	if _, err := Apply(s, SetScene{Location: "Ferry", Present: []string{"a dog"}}); err != nil {
		t.Fatalf("free-text present: %v", err)
	}
	if _, err := Apply(s, SetScene{Location: ""}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty location error = %v, want ErrInvalidArgument", err)
	}
}

// TestNote covers world annotations: assigned ID, count facts, guards.
func TestNote(t *testing.T) {
	s := newState(t, 1)
	res, err := Apply(s, Note{Topic: "clues", Text: "The ferryman lies.", Tags: []string{"lies"}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(s.World.Notes) != 1 || s.World.Notes[0].ID != "note-1" {
		t.Fatalf("notes = %+v, want note-1", s.World.Notes)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "note id: note-1") || !strings.Contains(joined, "notes held: 1") {
		t.Errorf("facts = %v", res.Facts)
	}
	if _, err := Apply(s, Note{Topic: " ", Text: "x"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty topic error = %v, want ErrInvalidArgument", err)
	}
	if _, err := Apply(s, Note{Topic: "x", Text: " "}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty text error = %v, want ErrInvalidArgument", err)
	}
	bare := &State{}
	if _, err := Apply(bare, Note{Topic: "x", Text: "y"}); !errors.Is(err, ErrNoWorld) {
		t.Errorf("no-world error = %v, want ErrNoWorld", err)
	}
}

// TestEndRecap covers recap storage and its guard, reading back through the
// SessionRecap query.
func TestEndRecap(t *testing.T) {
	s := newState(t, 1)
	res, err := Apply(s, EndRecap{Summary: "The party exposed the reeve."})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(SessionRecap(s)) != 1 {
		t.Errorf("recaps = %v, want one", SessionRecap(s))
	}
	if !strings.Contains(strings.Join(res.Facts, "\n"), "recaps: 0 → 1") {
		t.Errorf("facts = %v", res.Facts)
	}
	if _, err := Apply(s, EndRecap{Summary: "  "}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty recap error = %v, want ErrInvalidArgument", err)
	}
}

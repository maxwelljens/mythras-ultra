package world

import (
	"errors"
	"testing"
)

func TestAddClock(t *testing.T) {
	w := New("t", "t")
	events := []string{"the mob storms the granary"}
	if err := w.AddClock("  Unrest  ", 6, events...); err != nil {
		t.Fatalf("AddClock: %v", err)
	}
	c, ok := w.Clocks["Unrest"]
	if !ok {
		t.Fatal("clock stored under untrimmed name")
	}
	if c.Name != "Unrest" || c.Segments != 6 || c.Filled != 0 {
		t.Errorf("clock = %+v, want Unrest 6/0", c)
	}
	// OnComplete is copied: the caller's slice may be reused.
	events[0] = "mutated"
	if c.OnComplete[0] != "the mob storms the granary" {
		t.Errorf("OnComplete aliases caller slice: %v", c.OnComplete)
	}
}

func TestAddClockValidation(t *testing.T) {
	tests := []struct {
		name     string
		clock    string
		segments int
	}{
		{"empty name", "", 4},
		{"blank name", "   ", 4},
		{"zero segments", "unrest", 0},
		{"negative segments", "unrest", -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := New("t", "t")
			if err := w.AddClock(tc.clock, tc.segments); err == nil {
				t.Fatal("AddClock succeeded, want error")
			}
			if len(w.Clocks) != 0 {
				t.Errorf("failed AddClock mutated state: %+v", w.Clocks)
			}
		})
	}
	t.Run("duplicate", func(t *testing.T) {
		w := New("t", "t")
		if err := w.AddClock("unrest", 4); err != nil {
			t.Fatalf("first AddClock: %v", err)
		}
		if err := w.AddClock("unrest", 6); err == nil {
			t.Fatal("duplicate AddClock succeeded, want error")
		}
		if w.Clocks["unrest"].Segments != 4 {
			t.Errorf("duplicate AddClock mutated the clock: %+v", w.Clocks["unrest"])
		}
	})
}

func TestAdvanceClock(t *testing.T) {
	w := New("t", "t")
	if err := w.AddClock("unrest", 6, "the mob storms the granary"); err != nil {
		t.Fatalf("AddClock: %v", err)
	}

	// Partial advancement: no events, history grows.
	events, err := w.AdvanceClock("unrest", 2, "bread prices rise")
	if err != nil {
		t.Fatalf("advance 1: %v", err)
	}
	if events != nil {
		t.Errorf("partial advance events = %v, want nil", events)
	}
	if events, err = w.AdvanceClock("unrest", 3, "a preacher rails at the gate"); err != nil {
		t.Fatalf("advance 2: %v", err)
	}
	if events != nil {
		t.Errorf("partial advance events = %v, want nil", events)
	}

	// Exact completion fires OnComplete once.
	events, err = w.AdvanceClock("unrest", 1, "the watch mutinies")
	if err != nil {
		t.Fatalf("completion advance: %v", err)
	}
	if len(events) != 1 || events[0] != "the mob storms the granary" {
		t.Errorf("completion events = %v, want the OnComplete hook", events)
	}
	// Mutating the returned slice must not affect the clock.
	events[0] = "tampered"
	c := w.Clocks["unrest"]
	if c.OnComplete[0] != "the mob storms the granary" {
		t.Errorf("returned events alias the clock: %v", c.OnComplete)
	}
	if c.Filled != 6 {
		t.Errorf("Filled = %d, want 6", c.Filled)
	}
	wantHistory := []ClockChange{
		{Old: 0, New: 2, Reason: "bread prices rise"},
		{Old: 2, New: 5, Reason: "a preacher rails at the gate"},
		{Old: 5, New: 6, Reason: "the watch mutinies"},
	}
	if len(c.History) != len(wantHistory) {
		t.Fatalf("history = %+v, want %+v", c.History, wantHistory)
	}
	for i, change := range c.History {
		if change != wantHistory[i] {
			t.Errorf("history[%d] = %+v, want %+v", i, change, wantHistory[i])
		}
	}

	// Advancing a full clock is refused and leaves it untouched.
	if _, err := w.AdvanceClock("unrest", 1, "more"); !errors.Is(err, ErrClockComplete) {
		t.Errorf("advance full clock error = %v, want ErrClockComplete", err)
	}
	if w.Clocks["unrest"].Filled != 6 || len(w.Clocks["unrest"].History) != 3 {
		t.Errorf("refused advance mutated state: %+v", w.Clocks["unrest"])
	}
}

func TestAdvanceClockBoundaries(t *testing.T) {
	t.Run("overshoot refused", func(t *testing.T) {
		w := New("t", "t")
		if err := w.AddClock("doom", 3); err != nil {
			t.Fatalf("AddClock: %v", err)
		}
		if _, err := w.AdvanceClock("doom", 4, "too much"); !errors.Is(err, ErrClockComplete) {
			t.Errorf("overshoot error = %v, want ErrClockComplete", err)
		}
		if w.Clocks["doom"].Filled != 0 {
			t.Errorf("overshoot mutated clock: %+v", w.Clocks["doom"])
		}
		// The exact remainder is accepted afterwards.
		events, err := w.AdvanceClock("doom", 3, "exact")
		if err != nil {
			t.Fatalf("exact fill: %v", err)
		}
		if events != nil {
			t.Errorf("events = %v, want nil (no OnComplete set)", events)
		}
	})
	t.Run("advancing to capacity is exact only", func(t *testing.T) {
		w := New("t", "t")
		if err := w.AddClock("doom", 3, "done"); err != nil {
			t.Fatalf("AddClock: %v", err)
		}
		if _, err := w.AdvanceClock("doom", 2, "half"); err != nil {
			t.Fatalf("advance: %v", err)
		}
		if _, err := w.AdvanceClock("doom", 2, "over"); !errors.Is(err, ErrClockComplete) {
			t.Errorf("error = %v, want ErrClockComplete", err)
		}
		if w.Clocks["doom"].Filled != 2 {
			t.Errorf("Filled = %d, want 2", w.Clocks["doom"].Filled)
		}
	})
}

func TestAdvanceClockErrors(t *testing.T) {
	w := New("t", "t")
	if err := w.AddClock("unrest", 4); err != nil {
		t.Fatalf("AddClock: %v", err)
	}
	tests := []struct {
		name     string
		clock    string
		segments int
		reason   string
	}{
		{"unknown clock", "nope", 1, "why"},
		{"zero segments", "unrest", 0, "why"},
		{"negative segments", "unrest", -1, "why"},
		{"empty reason", "unrest", 1, ""},
		{"blank reason", "unrest", 1, "   "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := w.AdvanceClock(tc.clock, tc.segments, tc.reason); err == nil {
				t.Fatal("AdvanceClock succeeded, want error")
			}
		})
	}
	if w.Clocks["unrest"].Filled != 0 {
		t.Errorf("failed advances mutated clock: %+v", w.Clocks["unrest"])
	}
	// ErrUnknownClock specifically, and nil maps must not panic.
	if _, err := w.AdvanceClock("nope", 1, "why"); !errors.Is(err, ErrUnknownClock) {
		t.Errorf("unknown clock error = %v, want ErrUnknownClock", err)
	}
	bare := &World{}
	if _, err := bare.AdvanceClock("nope", 1, "why"); !errors.Is(err, ErrUnknownClock) {
		t.Errorf("zero-value World error = %v, want ErrUnknownClock", err)
	}
}

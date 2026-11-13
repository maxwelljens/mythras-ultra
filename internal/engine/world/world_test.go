package world

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	w := New("Arcadia", "Iron Age Britain")
	if w.Name != "Arcadia" || w.Setting != "Iron Age Britain" {
		t.Fatalf("New identity = %q/%q, want Arcadia/Iron Age Britain", w.Name, w.Setting)
	}
	if len(w.Cultures) != 4 {
		t.Errorf("New seeded %d cultures, want the 4 defaults", len(w.Cultures))
	}
	if len(w.Factions) != 0 || len(w.Flags) != 0 || len(w.Clocks) != 0 || len(w.Locations) != 0 || len(w.Notes) != 0 {
		t.Errorf("New must start empty: %+v", w)
	}
	want := Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "08:00"}
	if w.Calendar != want {
		t.Errorf("New calendar = %+v, want %+v", w.Calendar, want)
	}
	// Mutators must tolerate nil maps (hand-built or partial states).
	bare := &World{}
	if err := bare.SetFlag("economy", EconomyStable, "setup"); err != nil {
		t.Fatalf("SetFlag on zero-value World: %v", err)
	}
	if err := bare.AddClock("unrest", 4); err != nil {
		t.Fatalf("AddClock on zero-value World: %v", err)
	}
}

func TestAddNote(t *testing.T) {
	w := New("t", "t")
	first, err := w.AddNote("clues", "The broken seal matches the vicar's ring.", "mystery")
	if err != nil {
		t.Fatalf("AddNote 1: %v", err)
	}
	second, err := w.AddNote("NPC motives", "The smith wants the old mine reopened.")
	if err != nil {
		t.Fatalf("AddNote 2: %v", err)
	}
	if first.ID != "note-1" || second.ID != "note-2" {
		t.Errorf("note IDs = %q, %q, want note-1, note-2", first.ID, second.ID)
	}
	if len(w.Notes) != 2 || !reflect.DeepEqual(w.Notes[0], first) || w.Notes[1].Topic != "NPC motives" {
		t.Errorf("notes not stored in append order: %+v", w.Notes)
	}
	if len(first.Tags) != 1 || first.Tags[0] != "mystery" {
		t.Errorf("first note tags = %v, want [mystery]", first.Tags)
	}
	if len(second.Tags) != 0 {
		t.Errorf("second note tags = %v, want none", second.Tags)
	}
}

func TestAddNoteValidation(t *testing.T) {
	w := New("t", "t")
	tests := []struct {
		name  string
		topic string
		text  string
	}{
		{"empty topic", "", "body"},
		{"blank topic", "   ", "body"},
		{"empty text", "topic", ""},
		{"blank text", "topic", "   "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := w.AddNote(tc.topic, tc.text); err == nil {
				t.Fatal("AddNote succeeded, want error")
			}
		})
	}
	if len(w.Notes) != 0 {
		t.Errorf("failed AddNote mutated state: %+v", w.Notes)
	}
}

// buildSampleWorld returns a world exercised across every subsystem. The
// order variant inserts free-form flags in reverse to prove Summary (and
// JSON round-trip) does not depend on map insertion order.
func buildSampleWorld(t *testing.T, reverse bool) *World {
	t.Helper()
	w := New("Arcadia", "Iron Age Britain with ghosts")
	ops := []struct {
		key, value, reason string
	}{
		{"economy", EconomyCollapsed, "the war took the metal"},
		{"war", WarActive, "legions on the march"},
		{"region:altea:blockade", "true", "the harbour is shut"},
		{"prophecy", "fulfilled", "the oracle spoke"},
	}
	if reverse {
		for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
			ops[i], ops[j] = ops[j], ops[i]
		}
	}
	for _, op := range ops {
		if err := w.SetFlag(op.key, op.value, op.reason); err != nil {
			t.Fatalf("SetFlag(%q): %v", op.key, err)
		}
	}
	// A second transition, so history has old → new entries.
	if err := w.SetFlag("economy", EconomyStrained, "the harvest recovered"); err != nil {
		t.Fatalf("SetFlag transition: %v", err)
	}
	if err := w.AddClock("unrest", 6, "the mob storms the granary"); err != nil {
		t.Fatalf("AddClock: %v", err)
	}
	w.Clocks["unrest"].Tags = []string{"tension"}
	if _, err := w.AdvanceClock("unrest", 2, "bread prices rise"); err != nil {
		t.Fatalf("AdvanceClock: %v", err)
	}
	w.Factions["stonewright"] = &Faction{Name: "stonewright", Attitude: "wary"}
	w.Locations["altea"] = &Location{Name: "altea", Region: "coast"}
	if _, err := w.AddNote("clues", "Seal matches the vicar's ring.", "mystery"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if _, err := w.Calendar.Advance(Duration{Days: 35}); err != nil {
		t.Fatalf("Calendar.Advance: %v", err)
	}
	return w
}

func TestSummaryDeterminism(t *testing.T) {
	a := buildSampleWorld(t, false)
	b := buildSampleWorld(t, true) // same ops, different insertion order
	sa, sb := a.Summary(), b.Summary()
	if sa != sb {
		t.Errorf("Summary depends on insertion order:\n--- a ---\n%s\n--- b ---\n%s", sa, sb)
	}
	// Same ops → same Summary: summaries are stable across calls.
	if a.Summary() != sa {
		t.Error("Summary is not stable across calls on the same world")
	}
	if b.Summary() != sb {
		t.Error("Summary is not stable across calls on the same world")
	}
}

func TestSummaryContent(t *testing.T) {
	s := buildSampleWorld(t, false).Summary()
	want := []string{
		"World: Arcadia",
		"Setting: Iron Age Britain with ghosts",
		"Date: Year 1, Month 2, Day 6 (Winter, 08:00)",
		"Cultures (4): barbarian, civilised, nomadic, primitive",
		"Flags (4):",
		"  economy = strained (reason: the harvest recovered)",
		"  war = active (reason: legions on the march)",
		"  region:altea:blockade = true",
		"  prophecy = fulfilled",
		"Clocks (1):",
		"  [2/6] unrest (tension)",
		"Factions (1):",
		"  stonewright - attitude: wary",
		"Locations (1):",
		"  altea (region: coast)",
		"Notes (1):",
		"  note-1 clues: Seal matches the vicar's ring. [mystery]",
	}
	for _, wantLine := range want {
		if !strings.Contains(s, wantLine) {
			t.Errorf("Summary missing line %q:\n%s", wantLine, s)
		}
	}
	// Sorted key order: economy < prophecy < region:altea:blockade < war.
	econ := strings.Index(s, "  economy =")
	prop := strings.Index(s, "  prophecy =")
	region := strings.Index(s, "  region:altea:blockade =")
	war := strings.Index(s, "  war =")
	if econ < prop && prop < region && region < war {
		return
	}
	t.Errorf("flags not in sorted key order:\n%s", s)
}

func TestJSONRoundTrip(t *testing.T) {
	orig := buildSampleWorld(t, false)
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded World
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(orig, &decoded) {
		t.Errorf("round-trip mismatch:\n--- original ---\n%+v\n--- decoded ---\n%+v", orig, &decoded)
	}
	if orig.Summary() != decoded.Summary() {
		t.Errorf("Summary changed across round-trip:\n--- original ---\n%s--- decoded ---\n%s",
			orig.Summary(), decoded.Summary())
	}
	// The decoded world keeps working with append-only semantics intact.
	if err := decoded.SetFlag("economy", EconomyStable, "peace talks"); err != nil {
		t.Fatalf("SetFlag after round-trip: %v", err)
	}
	f, ok := decoded.Lookup("economy")
	if !ok {
		t.Fatal("economy flag lost in round-trip")
	}
	if got := len(f.History); got != 3 {
		t.Errorf("history after round-trip + set = %d entries, want 3", got)
	}
	last := f.History[len(f.History)-1]
	if last.Old != EconomyStrained || last.New != EconomyStable || last.Reason != "peace talks" {
		t.Errorf("last change = %+v, want strained → stable (peace talks)", last)
	}
}

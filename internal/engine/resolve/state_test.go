package resolve

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
)

// TestNewState checks the constructor contract: recorded seed (including
// the entropy fallback for seed 0), schema version, ready-to-play maps,
// world, and no wall-clock reads.
func TestNewState(t *testing.T) {
	s := NewState("The Ash Road", 42)
	if s.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", s.SchemaVersion, SchemaVersion)
	}
	if s.Seed != 42 {
		t.Errorf("Seed = %d, want 42", s.Seed)
	}
	if s.Name != "The Ash Road" {
		t.Errorf("Name = %q", s.Name)
	}
	if s.Characters == nil {
		t.Error("Characters map is nil; CreateCharacter would panic")
	}
	if s.World == nil {
		t.Error("World is nil; world moves would refuse from turn one")
	}
	if !s.CreatedAt.IsZero() || !s.UpdatedAt.IsZero() {
		t.Error("engine must not stamp wall time in NewState")
	}
	if s.EventSeq != 0 || s.RNGState != nil {
		t.Errorf("fresh state has EventSeq %d / RNGState %x", s.EventSeq, s.RNGState)
	}
	if s.ID == "" || strings.ContainsAny(s.ID, `/\`) {
		t.Errorf("ID %q is not filesystem-safe", s.ID)
	}

	seeded := NewState("entropy", 0)
	if seeded.Seed == 0 {
		t.Error("zero seed must be replaced by an entropy seed that is recorded")
	}
	if seeded.Rand() == nil || seeded.Rand().Seed() != seeded.Seed {
		t.Error("Rand must build from the recorded seed")
	}
}

// TestNormalizeRebindsParticipantIdentity is the load-critical identity
// fix: after a JSON round trip the encounter's participants must be the
// SAME instances as State.Characters — combat owns participants by pointer.
func TestNormalizeRebindsParticipantIdentity(t *testing.T) {
	s, _, _ := duelState(t, 42)
	if s.Encounter.Participants[0] != s.Characters["pc-1"] {
		t.Fatal("fixture: participants must alias Characters before the round trip")
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.Encounter.Participants[0] == loaded.Characters["pc-1"] {
		t.Fatal("JSON must decode participants as distinct copies (precondition for the fix)")
	}
	if err := loaded.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	for i, p := range loaded.Encounter.Participants {
		want, ok := loaded.Characters[p.ID]
		if !ok {
			t.Fatalf("participant %d (%q) missing from Characters", i, p.ID)
		}
		if p != want {
			t.Errorf("participant %d (%q) is a copy, not the canonical instance", i, p.ID)
		}
	}
	// Operational consequence: combat lookup by pointer works again.
	if participantIndex(loaded.Encounter, loaded.Characters["pc-2"]) != 1 {
		t.Error("pointer identity not restored: participantIndex cannot find pc-2")
	}
	// A second round trip stays stable (idempotent).
	again, err := json.Marshal(&loaded)
	if err != nil {
		t.Fatalf("marshal2: %v", err)
	}
	if string(again) != string(data) {
		t.Error("Normalize + round trip changed the state bytes")
	}
}

// TestNormalizeErrors covers corrupt encounter references: dangling
// participant IDs, ID-less participants, nil entries, out-of-range indices
// and a mismatched AP slice must all be refused with sentinels — never
// loaded into play, never panic later.
func TestNormalizeErrors(t *testing.T) {
	cases := []struct {
		name    string
		state   func() *State
		wantErr error
	}{
		{"dangling participant", func() *State {
			s := newState(t, 1)
			addCharacter(t, s, "pc-1", true)
			if _, err := Apply(s, StartEncounter{Participants: []string{"pc-1"}}); err != nil {
				t.Fatalf("StartEncounter: %v", err)
			}
			delete(s.Characters, "pc-1")
			return s
		}, ErrDanglingParticipant},
		{"participant without ID", func() *State {
			s := newState(t, 1)
			c := addCharacter(t, s, "pc-1", true)
			s.Encounter = encounterWith(t, c)
			c.ID = "" // identity lost before save
			delete(s.Characters, "pc-1")
			s.Encounter.Participants[0] = &char.Character{Name: "nameless"}
			return s
		}, ErrDanglingParticipant},
		{"nil participant", func() *State {
			s := newState(t, 1)
			c := addCharacter(t, s, "pc-1", true)
			s.Encounter = encounterWith(t, c)
			s.Encounter.Participants[0] = nil
			return s
		}, ErrDanglingParticipant},
		{"order index out of range", func() *State {
			s := newState(t, 1)
			c := addCharacter(t, s, "pc-1", true)
			s.Encounter = encounterWith(t, c)
			s.Encounter.Order = append(s.Encounter.Order, orderEntryAt(7))
			return s
		}, ErrInvalidState},
		{"counter index out of range", func() *State {
			s := newState(t, 1)
			c := addCharacter(t, s, "pc-1", true)
			s.Encounter = encounterWith(t, c)
			s.Encounter.Counters = append(s.Encounter.Counters, counterEntryAt(9))
			return s
		}, ErrInvalidState},
		{"AP slice mismatch", func() *State {
			s := newState(t, 1)
			c := addCharacter(t, s, "pc-1", true)
			s.Encounter = encounterWith(t, c)
			s.Encounter.AP = nil
			return s
		}, ErrInvalidState},
		{"corrupt RNG stream", func() *State {
			s := newState(t, 1)
			s.RNGState = []byte("not a PCG stream")
			return s
		}, ErrInvalidState},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.state().Normalize()
			if err == nil {
				t.Fatal("Normalize = nil, want error")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Normalize error = %v, want wrapping %v", err, tc.wantErr)
			}
		})
	}
	if err := (*State)(nil).Normalize(); !errors.Is(err, ErrInvalidState) {
		t.Errorf("nil state Normalize = %v, want ErrInvalidState", err)
	}
}

// TestHandBuiltStateGuards exercises the zero-value *State path: no panic,
// structured errors from world-dependent moves, and CreateCharacter
// initialising the characters map it inserts into.
func TestHandBuiltStateGuards(t *testing.T) {
	s := &State{}
	if _, err := Apply(s, CreateCharacter{
		Name: "First", PC: true, Culture: "Barbarian", Career: "Agent",
		Characteristics: &char.Characteristics{STR: 10, CON: 10, SIZ: 10, DEX: 10, INT: 10, POW: 10, CHA: 10},
	}); err != nil {
		t.Fatalf("CreateCharacter on zero state: %v", err)
	}
	if _, err := GetCharacter(s, "pc-1"); err != nil {
		t.Errorf("created character unreadable: %v", err)
	}
	if _, err := Apply(s, Note{Topic: "x", Text: "y"}); !errors.Is(err, ErrNoWorld) {
		t.Errorf("world move on zero state error = %v, want ErrNoWorld", err)
	}
	if _, err := Apply(s, SetScene{Location: "nowhere"}); err != nil {
		t.Errorf("scene move on zero state: %v", err)
	}
}

// TestNormalizeFreshState documents that a state without an encounter only
// needs the map safety net.
func TestNormalizeFreshState(t *testing.T) {
	s := &State{}
	if err := s.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if s.Characters == nil {
		t.Error("Normalize must initialise a nil Characters map")
	}
}

// TestStateJSONRoundTrip pins lossless persistence: bytes in, identical
// bytes out (all ints, no float drift), with RNG stream included.
func TestStateJSONRoundTrip(t *testing.T) {
	s, att, def := duelState(t, 42)
	if _, err := Apply(s, RollDice{Expr: "2d6+3", Reason: "test"}); err != nil {
		t.Fatalf("RollDice: %v", err)
	}
	if _, err := Apply(s, ResolveAttack{Attacker: att, Weapon: "Broadsword", Defender: def, Reaction: "parry"}); err != nil {
		t.Fatalf("ResolveAttack: %v", err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := loaded.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	again, err := json.Marshal(&loaded)
	if err != nil {
		t.Fatalf("marshal2: %v", err)
	}
	if string(again) != string(data) {
		t.Errorf("state did not round-trip losslessly:\nfirst:  %s\nsecond: %s", data, again)
	}
	if loaded.EventSeq != s.EventSeq {
		t.Errorf("EventSeq = %d, want %d", loaded.EventSeq, s.EventSeq)
	}
	if string(loaded.RNGState) != string(s.RNGState) {
		t.Error("RNGState did not survive the round trip")
	}
}

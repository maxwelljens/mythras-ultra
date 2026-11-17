package resolve

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
)

// TestApplyIncrementsEventSeq checks the monotonic move counter: accepted
// moves count, refused moves do not.
func TestApplyIncrementsEventSeq(t *testing.T) {
	s := newState(t, 42)
	addCharacter(t, s, "pc-1", true)
	if _, err := Apply(s, SetScene{Location: "the gate"}); err != nil {
		t.Fatalf("SetScene: %v", err)
	}
	if _, err := Apply(s, RollDice{Expr: "1d6"}); err != nil {
		t.Fatalf("RollDice: %v", err)
	}
	if s.EventSeq != 2 {
		t.Errorf("EventSeq = %d after two accepted moves, want 2", s.EventSeq)
	}
	if _, err := Apply(s, SkillTest{Actor: "nobody", Skill: "Brawn"}); !errors.Is(err, ErrUnknownCharacter) {
		t.Fatalf("SkillTest error = %v, want ErrUnknownCharacter", err)
	}
	if s.EventSeq != 2 {
		t.Errorf("EventSeq = %d after a refused move, want 2", s.EventSeq)
	}
}

// TestApplyAllOrNothingOnValidationFailure asserts a refused move leaves
// byte-identical state (data, counters and RNG stream all untouched).
func TestApplyAllOrNothingOnValidationFailure(t *testing.T) {
	s := newState(t, 42)
	addCharacter(t, s, "pc-1", true)
	if _, err := Apply(s, RollDice{Expr: "1d100"}); err != nil {
		t.Fatalf("warm the stream: %v", err)
	}
	before, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cases := []Move{
		SkillTest{Actor: "ghost", Skill: "Brawn"},
		OpposedTest{ActorA: "pc-1", SkillA: "Brawn", ActorB: "ghost", SkillB: "Brawn"},
		SpendAP{Actor: "pc-1", Action: "attack"},           // no encounter
		ResolveAttack{Attacker: "pc-1", Defender: "ghost"}, // no encounter first anyway
		InventoryRemove{Actor: "pc-1", Item: "nope"},
		Heal{Actor: "pc-1", Method: "centuries"},
		SetScene{Location: ""},
		EndRecap{Summary: "   "},
	}
	for _, m := range cases {
		if _, err := Apply(s, m); err == nil {
			t.Errorf("%s: expected an error", m.Kind())
			continue
		}
		after, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("%s: state changed on failure:\nbefore: %s\nafter:  %s", m.Kind(), before, after)
		}
	}
}

// TestApplyRollsBackDiceOnMidMoveFailure covers the hard case: a move that
// consumed randomness before failing (CreateCharacter rolls
// characteristics, then the career name is unknown). The state AND the RNG
// stream must return to their pre-move position — the next roll matches an
// uninterrupted run.
func TestApplyRollsBackDiceOnMidMoveFailure(t *testing.T) {
	const seed = 1234
	s := newState(t, seed)
	_, err := Apply(s, CreateCharacter{
		Name: "Half-built", Culture: "Barbarian", Career: "No Such Career",
	})
	if !errors.Is(err, char.ErrUnknownCareer) {
		t.Fatalf("error = %v, want wrapping char.ErrUnknownCareer", err)
	}
	if len(s.Characters) != 0 {
		t.Fatal("failed creation left a character behind")
	}
	if s.EventSeq != 0 {
		t.Errorf("EventSeq = %d, want 0", s.EventSeq)
	}

	// The stream must be as untouched as the state.
	got, err := Apply(s, RollDice{Expr: "1d100"})
	if err != nil {
		t.Fatalf("RollDice: %v", err)
	}
	wantRoll, _ := dice.NewRand(seed).Roll("1d100")
	if got.Audit == nil || len(got.Audit.Rolls) != 1 || got.Audit.Rolls[0].Total != wantRoll.Total {
		t.Errorf("post-failure roll = %+v, want the first draw of seed %d (%d)", got.Audit, seed, wantRoll.Total)
	}
}

// TestApplyPersistsRNGState checks the mechanism: an accepted move that
// rolled dice stores the live stream in State.RNGState, and a state that
// never rolled keeps it empty.
func TestApplyPersistsRNGState(t *testing.T) {
	s := newState(t, 42)
	if _, err := Apply(s, SetScene{Location: "docks"}); err != nil {
		t.Fatalf("SetScene: %v", err)
	}
	if len(s.RNGState) != 0 {
		t.Errorf("RNGState = %x after a dice-free move, want unchanged", s.RNGState)
	}
	if _, err := Apply(s, RollDice{Expr: "2d6"}); err != nil {
		t.Fatalf("RollDice: %v", err)
	}
	if len(s.RNGState) == 0 {
		t.Fatal("RNGState empty after a move that rolled dice")
	}
	restored, err := dice.UnmarshalRandBinary(s.RNGState)
	if err != nil {
		t.Fatalf("RNGState must decode: %v", err)
	}
	if restored.Seed() != s.Seed {
		t.Errorf("stream seed = %d, want session seed %d", restored.Seed(), s.Seed)
	}
}

// TestApplyGuards checks the entry-point argument guards.
func TestApplyGuards(t *testing.T) {
	if _, err := Apply(nil, RollDice{Expr: "1d6"}); !errors.Is(err, ErrInvalidState) {
		t.Errorf("nil state: %v", err)
	}
	if _, err := Apply(newState(t, 1), nil); !errors.Is(err, ErrInvalidState) {
		t.Errorf("nil move: %v", err)
	}
}

// TestApplyAtomicityAcrossMoves is the property test for the rollback
// machinery: interleaving refusals with accepted moves must never shift the
// dice stream or the counters away from an all-accepted reference run.
func TestApplyAtomicityAcrossMoves(t *testing.T) {
	const seed = 99
	rollAll := func(s *State, moves []Move) []int {
		t.Helper()
		var totals []int
		for _, m := range moves {
			res, err := Apply(s, m)
			if err != nil {
				t.Fatalf("%s: %v", m.Kind(), err)
			}
			if res.Audit != nil && len(res.Audit.Rolls) == 1 {
				totals = append(totals, res.Audit.Rolls[0].Total)
			}
		}
		return totals
	}
	rolls := []Move{
		RollDice{Expr: "1d100"},
		RollDice{Expr: "1d100"},
		RollDice{Expr: "1d100"},
	}
	ref := newState(t, seed)
	want := rollAll(ref, rolls)

	noise := newState(t, seed)
	var got []int
	got = append(got, rollAll(noise, rolls[:1])...)
	// Three distinct refusals, each consuming nothing.
	for _, bad := range []Move{
		SkillTest{Actor: "ghost", Skill: "Brawn"},
		InventoryRemove{Actor: "ghost", Item: "rope"},
		Heal{Actor: "ghost", Method: "day"},
	} {
		if _, err := Apply(noise, bad); err == nil {
			t.Fatalf("%s: expected refusal", bad.Kind())
		}
	}
	got = append(got, rollAll(noise, rolls[1:])...)
	if !slices.Equal(got, want) {
		t.Errorf("dice sequence drifted across refused moves: got %v, want %v", got, want)
	}
	if noise.EventSeq != ref.EventSeq {
		t.Errorf("EventSeq = %d, want %d (refusals must not count)", noise.EventSeq, ref.EventSeq)
	}
}

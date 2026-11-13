package rules

import (
	"errors"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/dice"
)

// fatigueCharacter returns a character whose trained Endurance equals the
// requested value (Endurance base is CON×2, and char floors a skill at its
// base, so CON is derived from the value).
func fatigueCharacter(t *testing.T, endurance int) *char.Character {
	t.Helper()
	if endurance < 2 {
		t.Fatalf("test endurance %d too small", endurance)
	}
	c := &char.Character{
		Name: "Diver",
		Chars: char.Characteristics{
			STR: 10, CON: endurance / 2, SIZ: 11, DEX: 13, INT: 14, POW: 15, CHA: 12,
		},
	}
	if err := c.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if err := c.SetSkill("Endurance", endurance); err != nil {
		t.Fatalf("SetSkill: %v", err)
	}
	return c
}

// TestAsphyxiateHoldBreath checks the breath-hold period of digest L550-551
// (Mythras-Imperative.md L937): full Endurance seconds when prepared — the
// reductions apply only when unprepared — halved when unprepared and
// passive, one-fifth when unprepared and strenuous, fractions rounding up
// (L10-12).
func TestAsphyxiateHoldBreath(t *testing.T) {
	c := fatigueCharacter(t, 47) // odd on purpose: rounding must show
	cases := []struct {
		name                string
		prepared, strenuous bool
		want                int
	}{
		{"prepared gets the full Endurance in seconds", true, false, 47},
		{"prepared in strenuous activity is not reduced (L937: 'if not' prepared)", true, true, 47},
		{"unprepared and passive halved, rounding up (47/2 = 23.5 -> 24)", false, false, 24},
		{"unprepared and strenuous cut to one-fifth, rounding up (47/5 = 9.4 -> 10)", false, true, 10},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c.RemoveCondition(FatigueCondition) // isolate the cases
			res, err := Asphyxiate(dice.NewRand(int64(i+1)), c, tc.prepared, tc.strenuous)
			if err != nil {
				t.Fatalf("Asphyxiate: %v", err)
			}
			if res.HoldSeconds != tc.want {
				t.Errorf("HoldSeconds = %d, want %d", res.HoldSeconds, tc.want)
			}
			if res.Endurance != 47 || res.Prepared != tc.prepared || res.Strenuous != tc.strenuous {
				t.Errorf("result echo wrong: %+v", res)
			}
			if res.RuleRef != "Mythras Imperative, Game System: Asphyxiation" {
				t.Errorf("RuleRef = %q", res.RuleRef)
			}
		})
	}

	// An even Endurance divides without rounding: 46/2 = 23.
	c2 := fatigueCharacter(t, 46)
	res, err := Asphyxiate(dice.NewRand(1), c2, false, false)
	if err != nil {
		t.Fatalf("Asphyxiate: %v", err)
	}
	if res.HoldSeconds != 23 {
		t.Errorf("HoldSeconds = %d, want 23", res.HoldSeconds)
	}
}

// TestAsphyxiateConsequences checks the Endurance consequences of digest
// L551-554 (Critical: none; Success: 1; Failure: 1d2; Fumble: 1d3, that
// Round) against a fixed seed range: every grade's levels are verified per
// roll, and the distribution must cover all four grades and every die face.
func TestAsphyxiateConsequences(t *testing.T) {
	c := fatigueCharacter(t, 47)
	seen := map[Grade]bool{}
	failDice := map[int]bool{}
	fumbleDice := map[int]bool{}

	for seed := int64(1); seed <= 3000; seed++ {
		c.RemoveCondition(FatigueCondition)
		res, err := Asphyxiate(dice.NewRand(seed), c, true, false)
		if err != nil {
			t.Fatalf("seed %d: Asphyxiate: %v", seed, err)
		}
		// The roll must classify to the reported grade at Standard
		// difficulty (modified skill = Endurance).
		want, cerr := Classify(res.Roll, res.Endurance)
		if cerr != nil {
			t.Fatalf("Classify: %v", cerr)
		}
		if res.Grade != want {
			t.Fatalf("seed %d: grade %v for roll %d vs %d, want %v", seed, res.Grade, res.Roll, res.Endurance, want)
		}
		if res.Levels != res.Gained {
			t.Fatalf("seed %d: Levels %d != Gained %d on a fresh character", seed, res.Levels, res.Gained)
		}
		switch res.Grade {
		case GradeCritical:
			if res.Gained != 0 {
				t.Fatalf("seed %d: critical gained %d, want 0 (L941)", seed, res.Gained)
			}
		case GradeSuccess:
			if res.Gained != 1 {
				t.Fatalf("seed %d: success gained %d, want 1 (L942)", seed, res.Gained)
			}
		case GradeFailure:
			if res.Gained < 1 || res.Gained > 2 {
				t.Fatalf("seed %d: failure gained %d, want 1d2 (L943)", seed, res.Gained)
			}
			failDice[res.Gained] = true
		case GradeFumble:
			if res.Gained < 1 || res.Gained > 3 {
				t.Fatalf("seed %d: fumble gained %d, want 1d3 (L944)", seed, res.Gained)
			}
			fumbleDice[res.Gained] = true
		}
		seen[res.Grade] = true

		// The condition carries exactly the accumulated total — and is
		// absent when nothing is left to track (a critical gained 0).
		levels, present, lerr := fatigueLevels(c)
		if lerr != nil || levels != res.Levels || present != (res.Levels > 0) {
			t.Fatalf("seed %d: stored levels (%d, %v, %v) != %d", seed, levels, present, lerr, res.Levels)
		}
	}

	for _, g := range []Grade{GradeCritical, GradeSuccess, GradeFailure, GradeFumble} {
		if !seen[g] {
			t.Errorf("grade %v never observed over 3000 fixed-seed checks", g)
		}
	}
	if len(failDice) != 2 {
		t.Errorf("failure 1d2 faces observed = %v, want both 1 and 2", failDice)
	}
	if len(fumbleDice) != 3 {
		t.Errorf("fumble 1d3 faces observed = %v, want 1, 2 and 3", fumbleDice)
	}
}

// TestAsphyxiateEnduranceFailures checks the error paths: nil inputs and a
// malformed fatigue condition (loud refusal rather than silent state loss).
func TestAsphyxiateEnduranceFailures(t *testing.T) {
	c := fatigueCharacter(t, 47)
	if _, err := Asphyxiate(nil, c, true, false); err == nil {
		t.Error("Asphyxiate(nil Rand) should error")
	}
	if _, err := Asphyxiate(dice.NewRand(1), nil, true, false); err == nil {
		t.Error("Asphyxiate(nil character) should error")
	}
	c.AddCondition(char.Condition{Name: FatigueCondition, Notes: "rather tired"})
	if _, err := Asphyxiate(dice.NewRand(1), c, true, false); !errors.Is(err, ErrInvalidFatigue) {
		t.Errorf("malformed fatigue error = %v, want %v", err, ErrInvalidFatigue)
	}
}

// TestRecoverFatigue checks the Asphyxiation recovery rate of one level per
// minute (L946, digest L553-554), its clamping, and that nothing is
// destroyed by a missing or malformed condition.
func TestRecoverFatigue(t *testing.T) {
	c := fatigueCharacter(t, 47)
	c.AddCondition(char.Condition{Name: FatigueCondition, Notes: "levels: 5"})

	if got := RecoverFatigue(c, 3); got != 3 {
		t.Errorf("RecoverFatigue(3) = %d, want 3", got)
	}
	if got := RecoverFatigue(c, 100); got != 2 { // capped at what is left
		t.Errorf("RecoverFatigue(100) = %d, want 2", got)
	}
	if c.HasCondition(FatigueCondition) {
		t.Error("Fatigue condition should be gone at zero levels")
	}
	if got := RecoverFatigue(c, 5); got != 0 {
		t.Errorf("RecoverFatigue with no condition = %d, want 0", got)
	}

	// Non-positive minutes recover nothing and keep the condition.
	c.AddCondition(char.Condition{Name: FatigueCondition, Notes: "levels: 4"})
	if got := RecoverFatigue(c, 0); got != 0 {
		t.Errorf("RecoverFatigue(0) = %d, want 0", got)
	}
	if !c.HasCondition(FatigueCondition) {
		t.Error("condition removed without recovering")
	}

	// A malformed level count recovers nothing and is left untouched.
	c.AddCondition(char.Condition{Name: FatigueCondition, Notes: "rather tired"})
	if got := RecoverFatigue(c, 5); got != 0 {
		t.Errorf("RecoverFatigue(malformed) = %d, want 0", got)
	}
	for _, cond := range c.Conditions {
		if cond.Name == FatigueCondition && cond.Notes != "rather tired" {
			t.Errorf("malformed condition mutated: %q", cond.Notes)
		}
	}
	if got := RecoverFatigue(nil, 5); got != 0 {
		t.Errorf("RecoverFatigue(nil) = %d, want 0", got)
	}
}

// TestAsphyxiateThenRecover closes the loop: levels an Asphyxiation check
// accrues are fully recovered at one per minute.
func TestAsphyxiateThenRecover(t *testing.T) {
	c := fatigueCharacter(t, 47)
	var res FatigueResult
	found := false
	for seed := int64(1); seed <= 50; seed++ {
		c.RemoveCondition(FatigueCondition)
		out, err := Asphyxiate(dice.NewRand(seed), c, true, false)
		if err != nil {
			t.Fatalf("Asphyxiate: %v", err)
		}
		if out.Gained > 0 {
			res, found = out, true
			break
		}
	}
	if !found {
		t.Fatal("no non-critical Asphyxiation result over 50 fixed seeds")
	}
	if got := RecoverFatigue(c, res.Levels); got != res.Levels {
		t.Errorf("RecoverFatigue(%d) = %d, want all levels", res.Levels, got)
	}
	if c.HasCondition(FatigueCondition) {
		t.Error("fatigue should be fully recovered")
	}
}

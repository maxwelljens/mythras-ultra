package rules

import (
	"errors"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/dice"
)

// TestExperienceRollGainPaths checks both improvement branches of digest
// L959-962 (1d100+INT ≥ skill → +1d4+1%, below → +1%) across a fixed seed
// range, plus the audit fields and the sheet mutation.
func TestExperienceRollGainPaths(t *testing.T) {
	sawImproved, sawFlat := false, false
	for seed := int64(1); seed <= 400; seed++ {
		c := testCharacter(t)
		if err := c.SetSkill("Stealth", 40); err != nil {
			t.Fatalf("SetSkill: %v", err)
		}
		g, err := ExperienceRoll(dice.NewRand(seed), c, "Stealth")
		if err != nil {
			t.Fatalf("seed %d: ExperienceRoll: %v", seed, err)
		}
		if g.Skill != "Stealth" || g.Old != 40 || g.INT != c.Chars.INT {
			t.Fatalf("seed %d: gain = %+v, want Stealth from 40 with INT %d", seed, g, c.Chars.INT)
		}
		if g.Roll < 1 || g.Roll > 100 {
			t.Fatalf("seed %d: roll %d outside 1-100", seed, g.Roll)
		}
		// Branch rule: the compared number is Roll+INT against the skill
		// at roll time (Free adds 1% first, L975).
		target := g.Old
		if g.Free {
			target++
		}
		if g.Roll+g.INT >= target {
			if g.Dice < 2 || g.Dice > 5 {
				t.Fatalf("seed %d: improved by %d, want 1d4+1 (2-5)", seed, g.Dice)
			}
			sawImproved = true
		} else if g.Dice != 1 {
			t.Fatalf("seed %d: missed improvement rolled %d, want 1", seed, g.Dice)
		}
		if g.Dice == 1 {
			sawFlat = true // 1d4+1 is never 1, so Dice==1 means the flat branch
		}
		sk, ok := c.Skill("Stealth")
		if !ok {
			t.Fatal("skill vanished from the sheet")
		}
		if sk.Value != g.New || g.New != g.Old+g.Dice {
			t.Fatalf("seed %d: sheet %d, gain %+v", seed, sk.Value, g)
		}
		if !sk.Advanced {
			t.Fatalf("seed %d: Advanced flag not set", seed)
		}
	}
	if !sawImproved {
		t.Error("no +1d4+1% improvement observed over 400 seeded rolls")
	}
	if !sawFlat {
		t.Error("no flat +1% improvement observed over 400 seeded rolls")
	}
}

// TestExperienceRollOncePerSession checks the double-award guard of L977
// ("The same skill may not benefit from more than one Experience Roll per
// session") and that a refusal mutates nothing and consumes no dice.
func TestExperienceRollOncePerSession(t *testing.T) {
	c := testCharacter(t)
	if err := c.SetSkill("Stealth", 40); err != nil {
		t.Fatalf("SetSkill: %v", err)
	}
	first, err := ExperienceRoll(dice.NewRand(1), c, "Stealth")
	if err != nil {
		t.Fatalf("first ExperienceRoll: %v", err)
	}

	r := dice.NewRand(11)
	_, err = ExperienceRoll(r, c, "Stealth")
	if !errors.Is(err, ErrAlreadyExperienced) {
		t.Fatalf("second ExperienceRoll error = %v, want %v", err, ErrAlreadyExperienced)
	}
	sk, _ := c.Skill("Stealth")
	if sk.Value != first.New {
		t.Errorf("refused roll still mutated the skill: %d, want %d", sk.Value, first.New)
	}
	// The refusal must not consume randomness: a fresh Rand with the same
	// seed still yields the same next value.
	if got, want := r.D100(), dice.NewRand(11).D100(); got != want {
		t.Errorf("refused ExperienceRoll consumed dice: next roll %d, want %d", got, want)
	}
}

// TestExperienceRollFumbleFreeIncrease checks the free fumble increase of
// L975: applied before the roll is made, reported through Gain.Free, and
// added on top of the improvement die.
func TestExperienceRollFumbleFreeIncrease(t *testing.T) {
	c := testCharacter(t)
	if err := c.SetSkill("Stealth", 40); err != nil {
		t.Fatalf("SetSkill: %v", err)
	}
	sk, _ := c.Skill("Stealth")
	sk.Fumbled = true

	g, err := ExperienceRoll(dice.NewRand(3), c, "Stealth")
	if err != nil {
		t.Fatalf("ExperienceRoll: %v", err)
	}
	if !g.Free {
		t.Error("Gain.Free = false, want the pending fumble increase applied")
	}
	if sk.Fumbled {
		t.Error("Fumbled flag not cleared — multiple fumbles would stack (L975)")
	}
	if g.New != g.Old+1+g.Dice {
		t.Errorf("gain %+v: New must be Old + free 1%% + Dice", g)
	}
	// The free 1% was applied before the roll, so the comparison target
	// included it (L975).
	target := g.Old + 1
	if g.Roll+g.INT >= target {
		if g.Dice < 2 || g.Dice > 5 {
			t.Errorf("improved by %d at target %d, want 1d4+1 (2-5)", g.Dice, target)
		}
	} else if g.Dice != 1 {
		t.Errorf("missed improvement at target %d rolled %d, want 1", target, g.Dice)
	}
}

// TestApplyFumbleIncrease checks the free +1% standing on its own (L975):
// grantable without spending an Experience Roll, never stacking.
func TestApplyFumbleIncrease(t *testing.T) {
	c := testCharacter(t)
	if err := c.SetSkill("Stealth", 40); err != nil {
		t.Fatalf("SetSkill: %v", err)
	}
	sk, _ := c.Skill("Stealth")
	sk.Fumbled = true

	applied, err := ApplyFumbleIncrease(c, "Stealth")
	if err != nil || !applied {
		t.Fatalf("ApplyFumbleIncrease = (%v, %v), want (true, nil)", applied, err)
	}
	if sk.Value != 41 || sk.Fumbled {
		t.Errorf("after grant: value %d fumbled %v, want 41 false", sk.Value, sk.Fumbled)
	}

	// "Multiple Fumbles of the same skill do not stack" (L975): the flag is
	// cleared, so a second claim grants nothing.
	applied, err = ApplyFumbleIncrease(c, "Stealth")
	if err != nil || applied {
		t.Errorf("second ApplyFumbleIncrease = (%v, %v), want (false, nil)", applied, err)
	}
	if sk.Value != 41 {
		t.Errorf("value drifted to %d, want 41", sk.Value)
	}

	// A skill on the sheet with no pending fumble grants nothing.
	if applied, err = ApplyFumbleIncrease(c, "Perception"); err != nil || applied {
		t.Errorf("no-fumble ApplyFumbleIncrease = (%v, %v), want (false, nil)", applied, err)
	}
}

// TestExperienceRollPassion checks that passions improve like skills
// ("Passions: improved like skills", Mythras-Imperative.md L563; digest
// L563): the same roll mechanics, fumble increase and once-per-session
// guard apply.
func TestExperienceRollPassion(t *testing.T) {
	c := testCharacter(t)
	if err := c.AddPassion("Loyalty to the Crown", "organization", 1); err != nil {
		t.Fatalf("AddPassion: %v", err)
	}
	p := c.Passions["Loyalty to the Crown"]
	if p == nil {
		t.Fatal("passion missing from sheet")
	}

	gain, err := ExperienceRoll(dice.NewRand(2), c, "Loyalty to the Crown")
	if err != nil {
		t.Fatalf("ExperienceRoll on passion: %v", err)
	}
	if gain.Old != p.Value-gain.Dice || gain.New != p.Value || !p.Advanced {
		t.Errorf("gain %+v does not reflect passion value %d (advanced %v)", gain, p.Value, p.Advanced)
	}
	if _, err := ExperienceRoll(dice.NewRand(2), c, "Loyalty to the Crown"); !errors.Is(err, ErrAlreadyExperienced) {
		t.Errorf("second passion roll error = %v, want %v", err, ErrAlreadyExperienced)
	}

	// Fumble free-increase works on passions too.
	p.Fumbled = true
	if applied, err := ApplyFumbleIncrease(c, "Loyalty to the Crown"); err != nil || !applied {
		t.Errorf("passion ApplyFumbleIncrease = (%v, %v), want (true, nil)", applied, err)
	}
	if p.Fumbled {
		t.Error("Fumbled flag should be cleared after the free increase")
	}
}

// TestExperienceRollRefusals checks the sentinel paths: names that are not
// on the sheet and nil inputs.
func TestExperienceRollRefusals(t *testing.T) {
	c := testCharacter(t)
	if err := c.AddPassion("Loyalty to the Crown", "organization", 1); err != nil {
		t.Fatalf("AddPassion: %v", err)
	}

	if _, err := ExperienceRoll(dice.NewRand(1), c, "Healing"); !errors.Is(err, ErrUnknownSkill) {
		t.Errorf("untrained skill error = %v, want %v (not on the sheet — learning costs L987)", err, ErrUnknownSkill)
	}
	if _, err := ExperienceRoll(dice.NewRand(1), c, "Kissing a cactus"); !errors.Is(err, ErrUnknownSkill) {
		t.Errorf("unknown skill error = %v, want %v", err, ErrUnknownSkill)
	}
	if _, err := ExperienceRoll(nil, c, "Stealth"); err == nil {
		t.Error("ExperienceRoll(nil Rand) should error")
	}
	if _, err := ExperienceRoll(dice.NewRand(1), nil, "Stealth"); err == nil {
		t.Error("ExperienceRoll(nil character) should error")
	}
	if _, err := ApplyFumbleIncrease(nil, "Stealth"); err == nil {
		t.Error("ApplyFumbleIncrease(nil character) should error")
	}
}

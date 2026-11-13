package rules

import (
	"errors"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/dice"
)

// testCharacter returns a refreshed character with deterministic
// characteristics (INT 14 feeds Experience Rolls, POW 15 feeds Luck Points).
// Refresh grants every Standard skill at its base and caches Derived.
func testCharacter(t *testing.T) *char.Character {
	t.Helper()
	c := &char.Character{
		Name: "Test Subject",
		Chars: char.Characteristics{
			STR: 10, CON: 12, SIZ: 11, DEX: 13, INT: 14, POW: 15, CHA: 12,
		},
	}
	if err := c.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return c
}

// TestSkillValue checks the three lookup paths of digest L415 (trained
// sheet value), L383-388 (Passions rated and rolled like skills) and the
// untrained characteristic base.
func TestSkillValue(t *testing.T) {
	c := testCharacter(t)
	if err := c.SetSkill("Stealth", 75); err != nil {
		t.Fatalf("SetSkill: %v", err)
	}
	healingBase, known := char.BaseSkill("Healing", c.Chars)
	if !known {
		t.Fatal("Healing must be in the catalogue")
	}
	if err := c.AddPassion("Loyalty to the Crown", "organization", 1); err != nil {
		t.Fatalf("AddPassion: %v", err)
	}
	passion := c.Passions["Loyalty to the Crown"].Value
	if err := c.SetSkill("Kissing", 40); err != nil { // custom skill on the sheet
		t.Fatalf("SetSkill custom: %v", err)
	}

	cases := []struct {
		name    string
		skill   string
		want    int
		wantErr error
	}{
		{"trained sheet skill", "Stealth", 75, nil},
		{"custom skill on the sheet", "Kissing", 40, nil},
		{"passion usable as skill", "Loyalty to the Crown", passion, nil},
		{"untrained professional uses characteristic base", "Healing", healingBase, nil},
		{"standard skill at its base", "Willpower", 2 * c.Chars.POW, nil},
		{"unknown name has no testable value", "Kissing a cactus", 0, ErrUnknownSkill},
		{"empty name", "", 0, ErrUnknownSkill},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SkillValue(c, tc.skill)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("SkillValue(%q) error = %v, want %v", tc.skill, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SkillValue(%q): %v", tc.skill, err)
			}
			if got != tc.want {
				t.Errorf("SkillValue(%q) = %d, want %d", tc.skill, got, tc.want)
			}
		})
	}

	if _, err := SkillValue(nil, "Stealth"); err == nil {
		t.Error("SkillValue(nil character) should error")
	}
}

// TestSkillTestModifiers covers both Difficulty Grade Table systems
// (digest L442-451) and the audit trail SkillTest must produce.
func TestSkillTestModifiers(t *testing.T) {
	cases := []struct {
		name         string
		skill        int
		grade        DifficultyGrade
		simplified   bool
		wantModified int
	}{
		{"multiplicative standard is a no-op", 50, DifficultyStandard, false, 50},
		{"simplified standard is a no-op", 50, DifficultyStandard, true, 50},
		{"very easy doubles, crossing 100%", 60, DifficultyVeryEasy, false, 120},
		{"simplified very easy adds 40", 60, DifficultyVeryEasy, true, 100},
		{"easy adds half again", 50, DifficultyEasy, false, 75},
		{"simplified easy adds 20", 50, DifficultyEasy, true, 70},
		{"hard removes one third", 50, DifficultyHard, false, 33}, // 50 − 17, fractions round up (Intro L10-12)
		{"simplified hard subtracts 20", 50, DifficultyHard, true, 30},
		{"formidable halves", 50, DifficultyFormidable, false, 25},
		{"simplified formidable subtracts 40", 50, DifficultyFormidable, true, 10},
		{"herculean reduces to one fifth", 50, DifficultyHerculean, false, 10},
		{"simplified herculean clamps at 0", 70, DifficultyHerculean, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := dice.NewRand(1)
			o, audit, err := SkillTest(r, tc.skill, tc.grade, tc.simplified)
			if err != nil {
				t.Fatalf("SkillTest: %v", err)
			}
			if audit.BaseSkill != tc.skill || audit.Grade != tc.grade || audit.ModifiedSkill != tc.wantModified {
				t.Errorf("audit = {roll %d base %d grade %v modified %d}, want base %d grade %v modified %d",
					audit.Roll, audit.BaseSkill, audit.Grade, audit.ModifiedSkill,
					tc.skill, tc.grade, tc.wantModified)
			}
			if o.Skill != audit.ModifiedSkill || o.Roll != audit.Roll {
				t.Errorf("Outcome %+v does not match audit", o)
			}
			// Classification is always Classify(roll, modified skill)
			// (digest L418-433).
			want, cerr := Classify(audit.Roll, audit.ModifiedSkill)
			if cerr != nil {
				t.Fatalf("Classify: %v", cerr)
			}
			if o.Grade != want || audit.Outcome != want {
				t.Errorf("grade = %v (outcome %v), want %v", o.Grade, audit.Outcome, want)
			}
			if audit.RuleRef != "Mythras Imperative, Skills: How Skills Work" {
				t.Errorf("RuleRef = %q", audit.RuleRef)
			}
		})
	}
}

// TestSkillTestSentinels checks that Automatic and Hopeless propagate from
// both modifier systems before any dice are rolled — the replayable RNG
// sequence must not advance on a refusal (digest L444, L451).
func TestSkillTestSentinels(t *testing.T) {
	cases := []struct {
		name       string
		grade      DifficultyGrade
		simplified bool
		want       error
	}{
		{"automatic propagates (multiplicative)", DifficultyAutomatic, false, ErrAutomatic},
		{"automatic propagates (simplified)", DifficultyAutomatic, true, ErrAutomatic},
		{"hopeless propagates (multiplicative)", DifficultyHopeless, false, ErrHopeless},
		{"hopeless propagates (simplified)", DifficultyHopeless, true, ErrHopeless},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := dice.NewRand(7)
			_, _, err := SkillTest(r, 50, tc.grade, tc.simplified)
			if !errors.Is(err, tc.want) {
				t.Fatalf("SkillTest error = %v, want %v", err, tc.want)
			}
			// A fresh Rand with the same seed must still yield the same
			// next value: the refusal consumed no randomness.
			if got, want := r.D100(), dice.NewRand(7).D100(); got != want {
				t.Errorf("refused SkillTest consumed dice: next roll %d, want %d", got, want)
			}
		})
	}
	if _, _, err := SkillTest(dice.NewRand(1), -1, DifficultyStandard, false); !errors.Is(err, ErrInvalidSkill) {
		t.Errorf("negative skill error = %v, want %v", err, ErrInvalidSkill)
	}
	if _, _, err := SkillTest(nil, 50, DifficultyStandard, false); err == nil {
		t.Error("SkillTest(nil Rand) should error")
	}
}

// TestSkillTestModifiedOver100 checks the classification interaction when a
// grade pushes the skill past 100%: Very Easy doubles 60 to 120, so Fumbles
// happen only on 00 and the Critical range widens to 12 (digest L426-429).
func TestSkillTestModifiedOver100(t *testing.T) {
	r := dice.NewRand(99)
	sawCritical, sawFumble := false, false
	for i := 0; i < 2000; i++ {
		o, audit, err := SkillTest(r, 60, DifficultyVeryEasy, false)
		if err != nil {
			t.Fatalf("SkillTest: %v", err)
		}
		if audit.ModifiedSkill != 120 {
			t.Fatalf("modified = %d, want 120", audit.ModifiedSkill)
		}
		if o.Grade == GradeFumble && o.Roll != 100 {
			t.Fatalf("roll %d fumbles at modified 120 — over-100 skills fumble only on 00 (digest L429)", o.Roll)
		}
		if o.Grade == GradeCritical {
			sawCritical = true
			if o.Roll > 12 {
				t.Fatalf("critical on roll %d at modified 120 — critical range is 12 (digest L426)", o.Roll)
			}
		}
		if o.Grade == GradeFumble {
			sawFumble = true
		}
	}
	if !sawCritical {
		t.Error("no critical observed over 2000 rolls at modified 120 (expected range 01-12)")
	}
	if !sawFumble {
		t.Error("no fumble observed over 2000 rolls (expected 00)")
	}
}

// TestSkillTestCharacter checks SkillTestCharacter composing SkillValue with
// SkillTest across the trained, passion and unknown paths.
func TestSkillTestCharacter(t *testing.T) {
	c := testCharacter(t)
	if err := c.SetSkill("Stealth", 75); err != nil {
		t.Fatalf("SetSkill: %v", err)
	}
	if err := c.AddPassion("Loyalty to the Crown", "organization", 1); err != nil {
		t.Fatalf("AddPassion: %v", err)
	}
	passion := c.Passions["Loyalty to the Crown"].Value

	_, audit, err := SkillTestCharacter(dice.NewRand(3), c, "Stealth", DifficultyHard, false)
	if err != nil {
		t.Fatalf("SkillTestCharacter(Stealth): %v", err)
	}
	if audit.BaseSkill != 75 || audit.ModifiedSkill != 50 { // hard: reduce by one third
		t.Errorf("audit base %d modified %d, want 75 / 50", audit.BaseSkill, audit.ModifiedSkill)
	}

	_, audit, err = SkillTestCharacter(dice.NewRand(3), c, "Loyalty to the Crown", DifficultyStandard, true)
	if err != nil {
		t.Fatalf("SkillTestCharacter(passion): %v", err)
	}
	if audit.BaseSkill != passion {
		t.Errorf("passion base %d, want %d", audit.BaseSkill, passion)
	}

	if _, _, err := SkillTestCharacter(dice.NewRand(3), c, "Kissing a cactus", DifficultyStandard, false); !errors.Is(err, ErrUnknownSkill) {
		t.Errorf("unknown skill error = %v, want %v", err, ErrUnknownSkill)
	}
}

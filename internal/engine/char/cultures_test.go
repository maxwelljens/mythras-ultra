package char

import (
	"errors"
	"testing"
)

// sampleChars is a fixed characteristic set for table lookups in tests.
var sampleChars = Characteristics{STR: 12, CON: 14, SIZ: 10, DEX: 11, INT: 13, POW: 15, CHA: 9}

// TestCultureAndCareerIntegrity transcribes-checks the packages: 4
// cultures, 31 careers (tables.md §1.11–§1.12), unique canonical names,
// and every listed skill — standard, choice and professional — resolving in
// the skill catalogue (this catches any transcription typo against
// tables.md §1.8/§1.9).
func TestCultureAndCareerIntegrity(t *testing.T) {
	t.Parallel()
	cultures := Cultures()
	if len(cultures) != 4 {
		t.Fatalf("Cultures() = %d, want 4", len(cultures))
	}
	wantCultures := map[string]bool{"Barbarian": true, "Civilised": true, "Nomadic": true, "Primitive": true}
	seen := map[string]bool{}
	for _, ci := range cultures {
		if !wantCultures[ci.Name] {
			t.Errorf("unexpected culture %q", ci.Name)
		}
		if seen[ci.Name] {
			t.Errorf("duplicate culture %q", ci.Name)
		}
		seen[ci.Name] = true
		checkSkillPackage(t, "culture "+ci.Name, ci.StandardSkills, ci.Choices, ci.ProfessionalSkills)
	}

	careers := Careers()
	if len(careers) != 31 {
		t.Fatalf("Careers() = %d, want 31 (digest Step 6)", len(careers))
	}
	careerNames := map[string]bool{}
	for _, ki := range careers {
		if ki.Name == "" || ki.Description == "" {
			t.Errorf("career %+v needs name and description", ki)
		}
		if careerNames[ki.Name] {
			t.Errorf("duplicate career %q", ki.Name)
		}
		careerNames[ki.Name] = true
		checkSkillPackage(t, "career "+ki.Name, ki.StandardSkills, ki.Choices, ki.ProfessionalSkills)
	}
	// The AMBIGUOUS "Linguistics" rows must have been mapped to Language.
	for _, ki := range careers {
		if ki.Name == "Bounty Hunter" || ki.Name == "Detective" {
			for _, s := range ki.ProfessionalSkills {
				if s == "Linguistics" {
					t.Errorf("%s still lists Linguistics (AMBIGUOUS: presumed Language, tables.md §1.12)", ki.Name)
				}
			}
		}
	}
}

// checkSkillPackage asserts every skill in a culture/career package exists
// in the catalogue and choice groups are well-formed.
func checkSkillPackage(t *testing.T, pkg string, std []string, groups []ChoiceGroup, pros []string) {
	t.Helper()
	check := func(list []string) {
		for _, s := range list {
			if _, ok := BaseSkill(s, sampleChars); !ok {
				t.Errorf("%s lists unknown skill %q", pkg, s)
			}
		}
	}
	check(std)
	check(pros)
	for _, g := range groups {
		if g.Pick < 1 || len(g.Options) < g.Pick {
			t.Errorf("%s choice %q: pick %d of %d options", pkg, g.Description, g.Pick, len(g.Options))
		}
		check(g.Options)
	}
}

// TestApplyCultureCultureAndCareer checks adoption: case-insensitive
// lookup, pools initialised, Standard skills present at base, sentinel
// errors for unknown names.
func TestApplyCultureCultureAndCareer(t *testing.T) {
	t.Parallel()

	t.Run("culture", func(t *testing.T) {
		c := &Character{Chars: sampleChars}
		if err := ApplyCulture(c, "BARBARIAN"); err != nil {
			t.Fatalf("ApplyCulture: %v", err)
		}
		if c.Culture != "Barbarian" {
			t.Errorf("Culture = %q, want Barbarian", c.Culture)
		}
		if c.Pools == nil || c.Pools.Culture != 100 || c.Pools.Career != 100 || c.Pools.Bonus != 150 {
			t.Fatalf("pools = %+v, want 100/100/150", c.Pools)
		}
		if sk, ok := c.Skill("Athletics"); !ok || sk.Value != sk.Base {
			t.Errorf("granted standard skill missing or unbased: %+v", sk)
		}
		if _, ok := c.Skill("Magic"); ok {
			t.Error("professional skill granted without a pick")
		}
		if err := ApplyCulture(c, "owlbear"); !errors.Is(err, ErrUnknownCulture) {
			t.Errorf("unknown culture: err = %v, want ErrUnknownCulture", err)
		}
		if err := ApplyCulture(nil, "Barbarian"); err == nil {
			t.Error("nil character should fail")
		}
	})

	t.Run("career", func(t *testing.T) {
		c := &Character{Chars: sampleChars}
		if err := ApplyCareer(c, "warrior"); err != nil {
			t.Fatalf("ApplyCareer: %v", err)
		}
		if c.Career != "Warrior" {
			t.Errorf("Career = %q, want Warrior", c.Career)
		}
		if _, ok := c.Skill("Brawn"); !ok {
			t.Error("Warrior standard skill Brawn missing")
		}
		if err := ApplyCareer(c, "Astronaut"); !errors.Is(err, ErrUnknownCareer) {
			t.Errorf("unknown career: err = %v, want ErrUnknownCareer", err)
		}
	})
}

// TestSpendPool exercises the point-pool accounting (tables.md §1.10):
// eligibility, pick caps, per-skill Bonus cap, exhaustion and sentinels.
func TestSpendPool(t *testing.T) {
	t.Parallel()

	newBarbarian := func(t *testing.T) *Character {
		t.Helper()
		c := &Character{Chars: sampleChars}
		if err := ApplyCulture(c, "Barbarian"); err != nil {
			t.Fatalf("ApplyCulture: %v", err)
		}
		return c
	}

	t.Run("culture spend bookkeeping", func(t *testing.T) {
		c := newBarbarian(t)
		base, _ := BaseSkill("Athletics", c.Chars)
		if err := c.SpendPool(PoolCulture, "Athletics", 20); err != nil {
			t.Fatalf("SpendPool: %v", err)
		}
		sk, _ := c.Skill("Athletics")
		if sk.Value != base+20 {
			t.Errorf("Athletics = %d, want base+20 = %d", sk.Value, base+20)
		}
		if c.Pools.Culture != 80 {
			t.Errorf("Culture pool = %d, want 80", c.Pools.Culture)
		}
		if got := c.Pools.Spent[PoolCulture]["Athletics"]; got != 20 {
			t.Errorf("Spent[culture][Athletics] = %d, want 20", got)
		}
	})

	t.Run("ineligible for culture pool", func(t *testing.T) {
		c := newBarbarian(t)
		if err := c.SpendPool(PoolCulture, "Magic", 1); !errors.Is(err, ErrSkillNotInPool) {
			t.Errorf("err = %v, want ErrSkillNotInPool", err)
		}
	})

	t.Run("culture pool needs a culture", func(t *testing.T) {
		c := &Character{Chars: sampleChars}
		if err := c.SpendPool(PoolCulture, "Athletics", 1); !errors.Is(err, ErrUnknownCulture) {
			t.Errorf("err = %v, want ErrUnknownCulture", err)
		}
	})

	t.Run("career pool needs a career", func(t *testing.T) {
		c := newBarbarian(t)
		if err := c.SpendPool(PoolCareer, "Brawn", 1); !errors.Is(err, ErrUnknownCareer) {
			t.Errorf("err = %v, want ErrUnknownCareer", err)
		}
	})

	t.Run("three professional picks then the fourth is refused", func(t *testing.T) {
		c := newBarbarian(t)
		for _, pro := range []string{"Craft", "Healing", "Lore"} {
			if err := c.SpendPool(PoolCulture, pro, 1); err != nil {
				t.Fatalf("SpendPool(%q): %v", pro, err)
			}
		}
		if err := c.SpendPool(PoolCulture, "Musicianship", 1); !errors.Is(err, ErrTooManyPicks) {
			t.Errorf("4th professional: err = %v, want ErrTooManyPicks", err)
		}
	})

	t.Run("choice group pick cap", func(t *testing.T) {
		c := newBarbarian(t)
		if err := c.SpendPool(PoolCulture, "Boating", 1); err != nil {
			t.Fatalf("SpendPool(Boating): %v", err)
		}
		// Barbarian: "either Boating or Ride" — funding both breaks the pick.
		if err := c.SpendPool(PoolCulture, "Ride", 1); !errors.Is(err, ErrTooManyPicks) {
			t.Errorf("second choice option: err = %v, want ErrTooManyPicks", err)
		}
	})

	t.Run("career pool spend", func(t *testing.T) {
		c := &Character{Chars: sampleChars}
		if err := ApplyCulture(c, "Civilised"); err != nil {
			t.Fatalf("ApplyCulture: %v", err)
		}
		if err := ApplyCareer(c, "Warrior"); err != nil {
			t.Fatalf("ApplyCareer: %v", err)
		}
		base, _ := BaseSkill("Brawn", c.Chars)
		if err := c.SpendPool(PoolCareer, "Brawn", 10); err != nil {
			t.Fatalf("SpendPool: %v", err)
		}
		sk, _ := c.Skill("Brawn")
		if sk.Value != base+10 {
			t.Errorf("Brawn = %d, want %d", sk.Value, base+10)
		}
		if err := c.SpendPool(PoolCareer, "Magic", 1); !errors.Is(err, ErrSkillNotInPool) {
			t.Errorf("Magic not in Warrior package: err = %v, want ErrSkillNotInPool", err)
		}
	})

	t.Run("bonus pool rules", func(t *testing.T) {
		c := newBarbarian(t)
		// Any Standard skill, even outside the culture package.
		if err := c.SpendPool(PoolBonus, "Willpower", 5); err != nil {
			t.Fatalf("bonus on standard skill: %v", err)
		}
		// A Professional skill granted by the culture.
		if err := c.SpendPool(PoolBonus, "Craft", 5); err != nil {
			t.Fatalf("bonus on granted professional: %v", err)
		}
		// Not standard, not granted: refused.
		if err := c.SpendPool(PoolBonus, "Magic", 5); !errors.Is(err, ErrSkillNotInPool) {
			t.Errorf("bonus on ungranted professional: err = %v, want ErrSkillNotInPool", err)
		}
	})

	t.Run("age band caps bonus spend", func(t *testing.T) {
		c := newBarbarian(t)
		if err := c.ApplyAge(20); err != nil { // Adult: 150 points, +15 per skill
			t.Fatalf("ApplyAge: %v", err)
		}
		if err := c.SpendPool(PoolBonus, "Athletics", 15); err != nil {
			t.Fatalf("spend at cap: %v", err)
		}
		if err := c.SpendPool(PoolBonus, "Athletics", 1); !errors.Is(err, ErrIncreaseCap) {
			t.Errorf("over cap: err = %v, want ErrIncreaseCap", err)
		}
	})

	t.Run("exhaustion and argument validation", func(t *testing.T) {
		c := newBarbarian(t)
		if err := c.SpendPool(PoolCulture, "Athletics", 100); err != nil {
			t.Fatalf("spend full pool: %v", err)
		}
		if c.Pools.Culture != 0 {
			t.Errorf("Culture pool = %d, want 0", c.Pools.Culture)
		}
		if err := c.SpendPool(PoolCulture, "Brawn", 1); !errors.Is(err, ErrPoolExhausted) {
			t.Errorf("exhausted: err = %v, want ErrPoolExhausted", err)
		}
		if err := c.SpendPool("loot", "Athletics", 1); !errors.Is(err, ErrUnknownPool) {
			t.Errorf("unknown pool: err = %v, want ErrUnknownPool", err)
		}
		if err := c.SpendPool(PoolBonus, "Athletics", 0); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("zero points: err = %v, want ErrInvalidAmount", err)
		}
		if err := c.SpendPool(PoolBonus, "", 1); !errors.Is(err, ErrUnknownSkill) {
			t.Errorf("empty skill: err = %v, want ErrUnknownSkill", err)
		}
	})
}

// TestExperienceForAge checks the Experience Table band selection
// (tables.md §1.13), including the AMBIGUOUS overlap rule: the first row
// reaching the age wins, so overlap ages keep the younger category.
func TestExperienceForAge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		age        int
		wantCat    string
		wantBonus  int
		wantMaxInc int
	}{
		{3, "Young", 100, 10}, // below the table clamps to Young
		{14, "Young", 100, 10},
		{15, "Young", 100, 10}, // AMBIGUOUS overlap: first row wins
		{16, "Adult", 150, 15},
		{27, "Adult", 150, 15},
		{30, "Middle Aged", 200, 20},
		{40, "Middle Aged", 200, 20}, // AMBIGUOUS overlap: first row wins
		{44, "Senior", 250, 25},
		{60, "Senior", 250, 25}, // AMBIGUOUS overlap: first row wins
		{65, "Old", 300, 30},
		{120, "Old", 300, 30}, // beyond the table clamps to Old
	}
	for _, tt := range tests {
		band, err := ExperienceForAge(tt.age)
		if err != nil {
			t.Fatalf("ExperienceForAge(%d): %v", tt.age, err)
		}
		if band.Category != tt.wantCat || band.BonusPoints != tt.wantBonus || band.MaxIncrease != tt.wantMaxInc {
			t.Errorf("age %d → %+v, want %s/%d/%d", tt.age, band, tt.wantCat, tt.wantBonus, tt.wantMaxInc)
		}
	}
	if _, err := ExperienceForAge(-1); !errors.Is(err, ErrInvalidAge) {
		t.Errorf("negative age: err = %v, want ErrInvalidAge", err)
	}
	if bands := ExperienceBands(); len(bands) != 5 {
		t.Errorf("ExperienceBands() = %d rows, want 5", len(bands))
	}
}

// TestApplyAge checks that ApplyAge sizes the Bonus pool from the age band
// while keeping Culture/Career (AMBIGUOUS §9.16: the natural replacement
// reading).
func TestApplyAge(t *testing.T) {
	t.Parallel()
	c := &Character{Chars: sampleChars}
	if err := ApplyCulture(c, "Civilised"); err != nil {
		t.Fatalf("ApplyCulture: %v", err)
	}
	if err := c.ApplyAge(70); err != nil { // Old: 300 points, +30 cap
		t.Fatalf("ApplyAge: %v", err)
	}
	if c.Age != 70 {
		t.Errorf("Age = %d, want 70", c.Age)
	}
	if c.Pools.Bonus != 300 || c.Pools.MaxIncrease != 30 {
		t.Errorf("Bonus/MaxIncrease = %d/%d, want 300/30", c.Pools.Bonus, c.Pools.MaxIncrease)
	}
	if c.Pools.Culture != 100 || c.Pools.Career != 100 {
		t.Errorf("Culture/Career pools disturbed: %d/%d, want 100/100", c.Pools.Culture, c.Pools.Career)
	}
	if err := c.ApplyAge(-5); !errors.Is(err, ErrInvalidAge) {
		t.Errorf("negative age: err = %v, want ErrInvalidAge", err)
	}
}

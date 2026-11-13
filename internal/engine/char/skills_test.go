package char

import (
	"errors"
	"testing"
)

// TestBaseSkill_SpotChecks checks base-percentage formulas against the
// Standard Skills table (tables.md §1.8) and the Professional Skills table
// (§1.9) with hand-computed values.
func TestBaseSkill_SpotChecks(t *testing.T) {
	t.Parallel()
	ch := Characteristics{STR: 12, CON: 14, SIZ: 10, DEX: 11, INT: 13, POW: 15, CHA: 9}
	tests := []struct {
		skill string
		want  int
	}{
		{"Athletics", 12 + 11},         // STR+DEX
		{"Boating", 12 + 14},           // STR+CON
		{"Brawn", 12 + 10},             // STR+SIZ
		{"Endurance", 2 * 14},          // CON x2
		{"Evade", 2 * 11},              // DEX x2
		{"Customs", 2*13 + 40},         // INT x2, +40
		{"Native Tongue", 13 + 9 + 40}, // INT+CHA, +40
		{"Willpower", 2 * 15},          // POW x2
		{"Perception", 13 + 15},        // INT+POW
		{"Streetwise", 15 + 9},         // POW+CHA (professional)
		{"Seamanship", 13 + 14},        // INT+CON (professional)
		{"Lockpicking", 2 * 11},        // DEX x2 (professional)
		{"Fly", 12 + 11},               // STR+DEX (creature skill, §1.9 footnote)
	}
	for _, tt := range tests {
		got, ok := BaseSkill(tt.skill, ch)
		if !ok {
			t.Fatalf("BaseSkill(%q) unknown", tt.skill)
		}
		if got != tt.want {
			t.Errorf("BaseSkill(%q) = %d, want %d", tt.skill, got, tt.want)
		}
	}
	if _, ok := BaseSkill("Kissing", ch); ok {
		t.Error("unknown skill should not resolve")
	}
	if _, ok := BaseSkill("", ch); ok {
		t.Error("empty skill name should not resolve")
	}
}

// TestBaseSkillFormula checks the printed formula text round-trips exactly
// for representative rows of tables.md §1.8/§1.9.
func TestBaseSkillFormula(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"Athletics":     "STR+DEX",
		"Customs":       "INT x2, +40",
		"Native Tongue": "INT+CHA, +40",
		"Endurance":     "CON x2",
		"Willpower":     "POW x2",
		"Streetwise":    "POW+CHA",
		"Craft":         "DEX+INT",
	}
	for skill, formula := range want {
		got, ok := BaseSkillFormula(skill)
		if !ok || got != formula {
			t.Errorf("BaseSkillFormula(%q) = %q, %v; want %q", skill, got, ok, formula)
		}
	}
	if _, ok := BaseSkillFormula("Kissing"); ok {
		t.Error("unknown skill should have no formula")
	}
}

// TestSkillLists checks the shape of the catalogue: 22 Standard skills and
// 39+1 Professional skills (tables.md §1.8/§1.9), with every entry
// resolving and classes assigned correctly.
func TestSkillLists(t *testing.T) {
	t.Parallel()
	standard := StandardSkills()
	professional := ProfessionalSkills()
	if len(standard) != 22 {
		t.Errorf("StandardSkills() = %d skills, want 22 (digest Step 4)", len(standard))
	}
	if len(professional) != 40 {
		t.Errorf("ProfessionalSkills() = %d skills, want 40 (39 + Fly)", len(professional))
	}
	if len(AllSkills()) != 62 {
		t.Errorf("AllSkills() = %d entries, want 62", len(AllSkills()))
	}
	if !IsStandard("Athletics") || IsStandard("Lore") {
		t.Error("IsStandard misclassifies Athletics/Lore")
	}
	if !IsProfessional("Lore") || IsProfessional("Athletics") {
		t.Error("IsProfessional misclassifies Lore/Athletics")
	}
	// AllSkills returns a copy.
	m := AllSkills()
	delete(m, "Athletics")
	if _, ok := AllSkills()["Athletics"]; !ok {
		t.Error("AllSkills leaked the internal catalogue")
	}
}

// TestSetSkill covers custom-skill acceptance, base flooring and the
// sentinel errors.
func TestSetSkill(t *testing.T) {
	t.Parallel()
	c := &Character{Chars: Characteristics{STR: 12, DEX: 10, INT: 13}}

	// Known skill: Custom stays false, Base computed, value floored at base.
	if err := c.SetSkill("Athletics", 1); err != nil {
		t.Fatalf("SetSkill(Athletics, 1): %v", err)
	}
	sk, _ := c.Skill("Athletics")
	if sk.Custom {
		t.Error("catalogue skill flagged Custom")
	}
	if sk.Base != 22 || sk.Value != 22 {
		t.Errorf("Athletics base/value = %d/%d, want 22/22 (floored at base)", sk.Base, sk.Value)
	}
	if err := c.SetSkill("Athletics", 45); err != nil {
		t.Fatalf("SetSkill(Athletics, 45): %v", err)
	}
	sk, _ = c.Skill("Athletics")
	if sk.Value != 45 {
		t.Errorf("Athletics value = %d, want 45", sk.Value)
	}

	// Custom skill: allowed, flagged, no base.
	if err := c.SetSkill("Kissing", 37); err != nil {
		t.Fatalf("SetSkill(Kissing, 37): %v", err)
	}
	sk, ok := c.Skill("Kissing")
	if !ok || !sk.Custom || sk.Base != 0 || sk.Value != 37 {
		t.Errorf("custom skill = %+v, want Custom with value 37", sk)
	}

	if err := c.SetSkill("Athletics", -1); !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("negative value: err = %v, want ErrInvalidAmount", err)
	}
	if err := c.SetSkill("", 5); !errors.Is(err, ErrUnknownSkill) {
		t.Errorf("empty name: err = %v, want ErrUnknownSkill", err)
	}
}

// TestPassions checks the Passion Table (tables.md §1.17): bases from
// characteristics, the +40/+30/+20 importance bonuses, and sentinel errors.
func TestPassions(t *testing.T) {
	t.Parallel()
	ch := Characteristics{STR: 1, CON: 1, SIZ: 1, DEX: 1, INT: 13, POW: 15, CHA: 9}
	if len(PassionKinds()) != 6 {
		t.Fatalf("PassionKinds() = %d rows, want 6", len(PassionKinds()))
	}
	tests := []struct {
		kind string
		want int
	}{
		{"person", 15 + 9},        // POW+CHA
		{"organization", 15 + 13}, // POW+INT
		{"race", 2 * 15},          // POW x2
		{"place", 15 + 13},        // POW+INT
		{"object", 2 * 15},        // POW x2
		{"trait", 15 + 9},         // POW+CHA
	}
	for _, tt := range tests {
		got, ok := PassionBase(tt.kind, ch)
		if !ok || got != tt.want {
			t.Errorf("PassionBase(%q) = %d, %v; want %d", tt.kind, got, ok, tt.want)
		}
	}
	if _, ok := PassionBase("vibes", ch); ok {
		t.Error("unknown passion kind should not resolve")
	}
	for order, want := range map[int]int{1: 40, 2: 30, 3: 20} {
		if got, ok := PassionBonus(order); !ok || got != want {
			t.Errorf("PassionBonus(%d) = %d, %v; want %d", order, got, ok, want)
		}
	}
	if _, ok := PassionBonus(4); ok {
		t.Error("importance 4 should not resolve")
	}

	c := &Character{Chars: ch}
	if err := c.AddPassion("Revenge", "race", 1); err != nil {
		t.Fatalf("AddPassion: %v", err)
	}
	if p := c.Passions["Revenge"]; p == nil || p.Value != 30+40 || p.Basis != "race" {
		t.Errorf("Revenge passion = %+v, want value 70 basis race", p)
	}
	if err := c.AddPassion("X", "vibes", 1); !errors.Is(err, ErrInvalidPassion) {
		t.Errorf("unknown kind: err = %v, want ErrInvalidPassion", err)
	}
	if err := c.AddPassion("X", "race", 0); !errors.Is(err, ErrInvalidPassion) {
		t.Errorf("bad order: err = %v, want ErrInvalidPassion", err)
	}
}

// TestCombatStyle checks the STR+DEX base of combat styles (Step 4).
func TestCombatStyle(t *testing.T) {
	t.Parallel()
	c := &Character{Chars: Characteristics{STR: 12, DEX: 11}}
	if err := c.SetCombatStyle("Broadsword and Shield", 34); err != nil {
		t.Fatalf("SetCombatStyle: %v", err)
	}
	cs := c.CombatStyles["Broadsword and Shield"]
	if cs == nil || cs.Base != 23 || cs.Value != 34 {
		t.Errorf("combat style = %+v, want base 23 value 34", cs)
	}
	if err := c.SetCombatStyle("", 10); err == nil {
		t.Error("empty style name should fail")
	}
	if err := c.SetCombatStyle("X", -1); !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("negative value: err = %v, want ErrInvalidAmount", err)
	}
}

// TestNextID checks the "prefix-N" identifier rule: IDs are never reused,
// junk is ignored, and an empty prefix defaults to "id".
func TestNextID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		prefix   string
		existing []string
		want     string
	}{
		{"fresh", "pc", nil, "pc-1"},
		{"gap-free numbering picks max+1", "pc", []string{"pc-1", "pc-3", "pc-10"}, "pc-11"},
		{"other prefixes ignored", "npc", []string{"pc-7", "npc-2"}, "npc-3"},
		{"junk ignored", "pc", []string{"pc-x", "pc-", "pc-2y"}, "pc-1"},
		{"empty prefix defaults to id", "", nil, "id-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextID(tt.prefix, tt.existing); got != tt.want {
				t.Errorf("NextID(%q, %v) = %q, want %q", tt.prefix, tt.existing, got, tt.want)
			}
		})
	}
}

// TestConditions checks condition bookkeeping (add replaces same
// name+location, remove clears a name, unconsciousness query).
func TestConditions(t *testing.T) {
	t.Parallel()
	c := &Character{}
	c.AddCondition(Condition{Name: CondProne})
	c.AddCondition(Condition{Name: CondStunned, Rounds: 3})
	c.AddCondition(Condition{Name: CondStunned, Rounds: 2}) // replaces
	if len(c.Conditions) != 2 {
		t.Fatalf("conditions = %d, want 2 (same name+location replaced)", len(c.Conditions))
	}
	if !c.HasCondition(CondProne) || c.IsUnconscious() {
		t.Error("HasCondition/IsUnconscious misreport")
	}
	c.AddCondition(Condition{Name: CondUnconscious})
	if !c.IsUnconscious() {
		t.Error("IsUnconscious should see the Unconscious condition")
	}
	c.RemoveCondition(CondStunned)
	if c.HasCondition(CondStunned) || len(c.Conditions) != 2 {
		t.Error("RemoveCondition did not clear Stunned")
	}
}

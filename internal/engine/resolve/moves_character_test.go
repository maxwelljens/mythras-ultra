package resolve

import (
	"errors"
	"strings"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
	"github.com/maxwelljens/mythras-ultra/internal/engine/rules"
)

// TestCreateCharacterRolled covers the standard creation path: characteristics
// are the exact sequence char.Generate draws from the session seed, culture
// and career grant their packages, the derived snapshot is fresh, and the ID
// comes from the pc- prefix series.
func TestCreateCharacterRolled(t *testing.T) {
	const seed = 555
	s := newState(t, seed)
	wantChars, err := char.Generate(dice.NewRand(seed), char.GenRoll)
	if err != nil {
		t.Fatalf("reference generate: %v", err)
	}

	res, err := Apply(s, CreateCharacter{
		Name: "Ynara", PC: true, Culture: "barbarian", Career: "Agent",
		Age: 20, Skills: map[string]int{"Brawn": 55},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	c, err := GetCharacter(s, "pc-1")
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	if c.Chars != wantChars {
		t.Errorf("characteristics = %+v, want the seed's reference roll %+v", c.Chars, wantChars)
	}
	if !c.PC || c.Culture != "Barbarian" || c.Career != "Agent" || c.Age != 20 {
		t.Errorf("identity wrong: pc=%v culture=%q career=%q age=%d", c.PC, c.Culture, c.Career, c.Age)
	}
	if c.Derived.HitPoints == 0 || c.CurrentHP == nil {
		t.Error("derived snapshot or current hit locations not initialised")
	}
	if sk, ok := c.Skill("Brawn"); !ok || sk.Value != 55 {
		t.Errorf("skill override: Brawl = %+v, want 55", sk)
	}
	if c.Pools == nil || c.Pools.Culture != char.CulturePoolPoints || c.Pools.Career != char.CareerPoolPoints {
		t.Error("creation pools not initialised")
	}
	joined := strings.Join(res.Facts, "\n")
	for _, want := range []string{"id: pc-1", "characteristics (rolled)", "derived:", "creation pools left"} {
		if !strings.Contains(joined, want) {
			t.Errorf("facts missing %q: %v", want, res.Facts)
		}
	}
	if res.Audit == nil || len(res.Audit.Citations) == 0 {
		t.Error("creation must cite the Characters steps")
	}

	// The ID series: next PC, then the NPC prefix series.
	if _, err := Apply(s, CreateCharacter{Name: "Corren", PC: true, Culture: "Civilised", Career: "Detective"}); err != nil {
		t.Fatalf("second create: %v", err)
	}
	if _, err := Apply(s, CreateCharacter{Name: "Gate Guard", Culture: "Barbarian", Career: "Bounty Hunter"}); err != nil {
		t.Fatalf("npc create: %v", err)
	}
	for _, id := range []string{"pc-2", "npc-1"} {
		if _, err := GetCharacter(s, id); err != nil {
			t.Errorf("%s missing after creation: %v", id, err)
		}
	}
	if len(s.Characters) != 3 {
		t.Errorf("Characters = %d entries, want 3", len(s.Characters))
	}
}

// TestCreateCharacterAssigned covers supplied characteristics: no dice are
// consumed, and package validation failures leave no trace.
func TestCreateCharacterAssigned(t *testing.T) {
	s := newState(t, 9)
	chars := char.Characteristics{STR: 10, CON: 10, SIZ: 10, DEX: 10, INT: 10, POW: 10, CHA: 10}
	if _, err := Apply(s, CreateCharacter{
		Name: "Assigned", PC: true, Culture: "Primitive", Career: "Crafter",
		Characteristics: &chars,
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	c, err := GetCharacter(s, "pc-1")
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	if c.Chars != chars {
		t.Errorf("characteristics = %+v, want supplied %+v", c.Chars, chars)
	}
	if len(s.RNGState) != 0 {
		t.Errorf("RNGState = %x, want empty (no dice were rolled)", s.RNGState)
	}

	cases := []struct {
		name    string
		move    CreateCharacter
		wantErr error
	}{
		{"empty name", CreateCharacter{Name: "", Culture: "Barbarian", Career: "Agent"}, ErrInvalidArgument},
		{"missing culture", CreateCharacter{Name: "X", Career: "Agent"}, ErrInvalidArgument},
		{"missing career", CreateCharacter{Name: "X", Culture: "Barbarian"}, ErrInvalidArgument},
		{"unknown culture", CreateCharacter{Name: "X", Culture: "Klingon", Career: "Agent"}, char.ErrUnknownCulture},
		{"unknown career", CreateCharacter{Name: "X", Culture: "Barbarian", Career: "Jedi"}, char.ErrUnknownCareer},
		{"negative age", CreateCharacter{Name: "X", Culture: "Barbarian", Career: "Agent", Age: -3}, char.ErrInvalidAge},
		{"negative skill", CreateCharacter{Name: "X", Culture: "Barbarian", Career: "Agent", Skills: map[string]int{"Brawn": -1}}, char.ErrInvalidAmount},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := len(s.Characters)
			if _, err := Apply(s, tc.move); !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want wrapping %v", err, tc.wantErr)
			}
			if len(s.Characters) != before {
				t.Error("failed creation inserted a character")
			}
		})
	}
}

// TestFinalizeCharacter covers equipment and Passions with their totals
// facts, plus the spec refusals.
func TestFinalizeCharacter(t *testing.T) {
	s := newState(t, 3)
	addCharacter(t, s, "pc-1", true)

	res, err := Apply(s, FinalizeCharacter{
		Actor: "pc-1",
		Equipment: []EquipmentSpec{
			{Name: "Broadsword"},
			{Name: "Bedroll", Qty: 2},
		},
		Passions: []PassionSpec{
			{Name: "Hate (Orcs)", Kind: "race", Order: 3},
			{Name: "Loyalty to the clan", Kind: "organization"}, // order defaults to 1
		},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	c, _ := GetCharacter(s, "pc-1")
	if len(c.Inventory) != 2 {
		t.Fatalf("inventory = %d items, want 2", len(c.Inventory))
	}
	sword := c.Inventory[0]
	if sword.Weapon == nil || sword.ID == "" || sword.Qty != 1 {
		t.Errorf("Broadsword = %+v, want a statistics-carrying item with an ID", sword)
	}
	if roll := c.Inventory[1]; roll.Weapon != nil || roll.Qty != 2 {
		t.Errorf("Bedroll = %+v, want plain item ×2", roll)
	}
	// race base = POW doubled (28) + importance-3 bonus (20) = 48.
	if p := c.Passions["Hate (Orcs)"]; p == nil || p.Value != 48 || p.Basis != "race" {
		t.Errorf("Hate (Orcs) = %+v, want value 48 basis race", p)
	}
	if p := c.Passions["Loyalty to the clan"]; p == nil || p.Value == 0 {
		t.Errorf("second passion missing: %+v", c.Passions)
	}
	joined := strings.Join(res.Facts, "\n")
	for _, want := range []string{"added: Broadsword", "no game statistics", "passion: Hate (Orcs) = 48%", "totals: hit points", "carried:"} {
		if !strings.Contains(joined, want) {
			t.Errorf("facts missing %q: %v", want, res.Facts)
		}
	}
	if res.Audit == nil || len(res.Audit.Changes) == 0 {
		t.Error("finalize must record audit changes")
	}

	cases := []struct {
		name    string
		move    FinalizeCharacter
		wantErr error
	}{
		{"unknown actor", FinalizeCharacter{Actor: "ghost"}, ErrUnknownCharacter},
		{"empty equipment", FinalizeCharacter{Actor: "pc-1", Equipment: []EquipmentSpec{{Name: " "}}}, ErrInvalidArgument},
		{"passion without kind", FinalizeCharacter{Actor: "pc-1", Passions: []PassionSpec{{Name: "X"}}}, ErrInvalidArgument},
		{"unknown passion kind", FinalizeCharacter{Actor: "pc-1", Passions: []PassionSpec{{Name: "X", Kind: "nonsense"}}}, char.ErrInvalidPassion},
		{"bad importance", FinalizeCharacter{Actor: "pc-1", Passions: []PassionSpec{{Name: "X", Kind: "race", Order: 9}}}, char.ErrInvalidPassion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cur := s.Characters["pc-1"] // refusals rebuild the aggregate
			before := len(cur.Inventory)
			if _, err := Apply(s, tc.move); !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want wrapping %v", err, tc.wantErr)
			}
			if len(s.Characters["pc-1"].Inventory) != before {
				t.Error("refused finalize changed the inventory")
			}
		})
	}
}

// TestUpdateCharacter covers the free-text-only diff contract.
func TestUpdateCharacter(t *testing.T) {
	s := newState(t, 3)
	c := addCharacter(t, s, "pc-1", true)

	notes := "Owes the ferryman a favour."
	concept := "Disgraced scout."
	res, err := Apply(s, UpdateCharacter{Actor: "pc-1", Concept: &concept, Notes: &notes})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if c.Concept != concept || c.Notes != notes || c.Appearance != "" {
		t.Errorf("fields = concept %q appearance %q notes %q", c.Concept, c.Appearance, c.Notes)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "concept: (none) → "+concept) {
		t.Errorf("facts = %v, want the before → after diff", res.Facts)
	}
	if res.Audit == nil || len(res.Audit.Changes) == 0 {
		t.Error("diff must be audited")
	}

	// Partial update leaves the other fields alone.
	look := "Weather-beaten."
	if _, err := Apply(s, UpdateCharacter{Actor: "pc-1", Appearance: &look}); err != nil {
		t.Fatalf("second update: %v", err)
	}
	if c.Notes != notes || c.Concept != concept || c.Appearance != look {
		t.Errorf("partial update disturbed other fields: %+v", c)
	}
	// Clearing a field is an explicit empty string.
	empty := ""
	if _, err := Apply(s, UpdateCharacter{Actor: "pc-1", Concept: &empty}); err != nil {
		t.Fatalf("clear update: %v", err)
	}
	if c.Concept != "" {
		t.Errorf("concept = %q, want cleared", c.Concept)
	}

	if _, err := Apply(s, UpdateCharacter{Actor: "pc-1"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("no-changes error = %v, want ErrInvalidArgument", err)
	}
	if _, err := Apply(s, UpdateCharacter{Actor: "ghost", Notes: &notes}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown actor error = %v, want ErrUnknownCharacter", err)
	}
}

// TestSkillAdvanceExperience pins the experience path against the engine's
// own roll: same seed, same Gain, audit carries both dice and the
// one-roll-per-session guard holds afterwards.
func TestSkillAdvanceExperience(t *testing.T) {
	const seed = 2024
	s := newState(t, seed)
	c := addCharacter(t, s, "pc-1", true)
	if err := c.SetSkill("Brawn", 50); err != nil {
		t.Fatalf("SetSkill: %v", err)
	}

	// Reference: the same roll against an identical character.
	ref := addCharacter(t, newState(t, seed), "pc-1", true)
	if err := ref.SetSkill("Brawn", 50); err != nil {
		t.Fatalf("ref SetSkill: %v", err)
	}
	want, err := rules.ExperienceRoll(dice.NewRand(seed), ref, "Brawn")
	if err != nil {
		t.Fatalf("reference ExperienceRoll: %v", err)
	}

	res, err := Apply(s, SkillAdvance{Actor: "pc-1", Skill: "Brawn"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	sk, _ := c.Skill("Brawn")
	if sk.Value != want.New || !sk.Advanced {
		t.Errorf("Brawl = %d (advanced %v), want reference %d", sk.Value, sk.Advanced, want.New)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "experience roll:") {
		t.Errorf("facts = %v, want the experience roll line", res.Facts)
	}
	if res.Audit == nil || len(res.Audit.Rolls) == 0 || res.Audit.Rolls[0].Total != want.Roll {
		t.Errorf("audit rolls = %+v, want the reference d100 %d", res.Audit, want.Roll)
	}
	if want.Dice > 1 {
		found := false
		for _, r := range res.Audit.Rolls {
			if r.Expr == "1d4+1" && r.Total == want.Dice {
				found = true
			}
		}
		if !found {
			t.Errorf("improvement die missing from audit: %+v", res.Audit.Rolls)
		}
	}
	if res.Audit == nil || len(res.Audit.Citations) == 0 {
		t.Error("advancement must cite Character Improvement")
	}

	// One Experience Roll per session per skill.
	if _, err := Apply(s, SkillAdvance{Actor: "pc-1", Skill: "Brawn"}); !errors.Is(err, rules.ErrAlreadyExperienced) {
		t.Errorf("second advance error = %v, want rules.ErrAlreadyExperienced", err)
	}
	if _, err := Apply(s, SkillAdvance{Actor: "pc-1", Skill: "Nope"}); !errors.Is(err, rules.ErrUnknownSkill) {
		t.Errorf("unknown skill error = %v, want rules.ErrUnknownSkill", err)
	}
	if _, err := Apply(s, SkillAdvance{Actor: "ghost", Skill: "Brawn"}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown actor error = %v, want ErrUnknownCharacter", err)
	}
}

// TestSkillAdvanceFumble covers the free +1% path: pending flag grants it,
// a second call reports no pending increase instead of inventing points.
func TestSkillAdvanceFumble(t *testing.T) {
	s := newState(t, 1)
	c := addCharacter(t, s, "pc-1", true)
	if err := c.SetSkill("Stealth", 40); err != nil {
		t.Fatalf("SetSkill: %v", err)
	}
	c.Skills["Stealth"].Fumbled = true

	res, err := Apply(s, SkillAdvance{Actor: "pc-1", Skill: "Stealth", Free: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	sk, _ := c.Skill("Stealth")
	if sk.Value != 41 || sk.Fumbled {
		t.Errorf("Stealth = %d fumbled=%v, want 41 with flag cleared", sk.Value, sk.Fumbled)
	}
	if !strings.Contains(strings.Join(res.Facts, "\n"), "free fumble increase") {
		t.Errorf("facts = %v", res.Facts)
	}

	again, err := Apply(s, SkillAdvance{Actor: "pc-1", Skill: "Stealth", Free: true})
	if err != nil {
		t.Fatalf("second free advance: %v", err)
	}
	if sk, _ := c.Skill("Stealth"); sk.Value != 41 {
		t.Errorf("value rose without a pending fumble: %d", sk.Value)
	}
	if !strings.Contains(strings.Join(again.Facts, "\n"), "no free") {
		t.Errorf("facts = %v, want a no-pending report", again.Facts)
	}
	if again.Audit != nil && len(again.Audit.Rolls) != 0 {
		t.Error("free increase must not roll dice")
	}
}

// TestSkillAdvancePassion pins that Passions advance like skills (digest:
// "Passions are improved like skills").
func TestSkillAdvancePassion(t *testing.T) {
	const seed = 31
	s := newState(t, seed)
	c := addCharacter(t, s, "pc-1", true)
	if err := c.AddPassion("Hate (Orcs)", "race", 2); err != nil {
		t.Fatalf("AddPassion: %v", err)
	}
	before := c.Passions["Hate (Orcs)"].Value
	if _, err := Apply(s, SkillAdvance{Actor: "pc-1", Skill: "Hate (Orcs)"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	after := c.Passions["Hate (Orcs)"].Value
	if after <= before {
		t.Errorf("passion %d → %d, want an increase", before, after)
	}
	if !c.Passions["Hate (Orcs)"].Advanced {
		t.Error("Advanced flag not set on the passion")
	}
}

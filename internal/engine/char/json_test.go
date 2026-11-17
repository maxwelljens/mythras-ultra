package char

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
)

// buildRoundTripCharacter assembles a character exercising every exported
// state family — generation, culture/career/age pools, spends, skills,
// passions, combat styles, inventory with equipment, ammo, wounds,
// conditions and life flags — so the round trip covers the real shape of
// state.json.
func buildRoundTripCharacter(t *testing.T) *Character {
	t.Helper()
	ch, err := Generate(dice.NewRand(2026), GenRoll)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	c := &Character{
		ID:                 NextID("pc", nil),
		Name:               "Mira Vane",
		PC:                 true,
		Concept:            "Disgraced courier with a debt",
		Appearance:         "Ink-stained fingers, watchful eyes",
		Notes:              "Owes the harbour master a favour",
		Chars:              ch,
		HitLocationCatalog: "",
	}
	if err := ApplyCulture(c, "Civilised"); err != nil {
		t.Fatalf("ApplyCulture: %v", err)
	}
	if err := ApplyCareer(c, "Thief"); err != nil {
		t.Fatalf("ApplyCareer: %v", err)
	}
	if err := c.ApplyAge(22); err != nil {
		t.Fatalf("ApplyAge: %v", err)
	}
	if err := c.SpendPool(PoolCulture, "Streetwise", 40); err != nil {
		t.Fatalf("SpendPool: %v", err)
	}
	if err := c.SpendPool(PoolCareer, "Lockpicking", 35); err != nil {
		t.Fatalf("SpendPool: %v", err)
	}
	if err := c.SpendPool(PoolBonus, "Perception", 15); err != nil { // Adult cap is +15
		t.Fatalf("SpendPool: %v", err)
	}
	if err := c.SetSkill("Kissing", 40); err != nil { // custom skill
		t.Fatalf("SetSkill: %v", err)
	}
	if err := c.AddPassion("Revenge", "person", 1); err != nil {
		t.Fatalf("AddPassion: %v", err)
	}
	if err := c.SetCombatStyle("Dagger and Buckler", 47); err != nil {
		t.Fatalf("SetCombatStyle: %v", err)
	}

	sword, _ := NewWeapon("Dagger")
	shield, _ := NewShield("Buckler")
	leather, err := ArmorByName("Bike Leathers")
	if err != nil {
		t.Fatalf("ArmorByName: %v", err)
	}
	armor := Item{Name: leather.Name, Qty: 1, Tags: []string{TagArmor}, Armor: &ArmorStats{Points: map[string]int{"*": leather.AP}}}
	arrows := &Item{Name: "Arrows", Qty: 20, Tags: []string{TagAmmunition}}
	for _, it := range []*Item{&sword, &shield, &armor, arrows} {
		if err := c.AddItem(it); err != nil {
			t.Fatalf("AddItem: %v", err)
		}
	}
	if err := c.Wield("main", sword.ID); err != nil {
		t.Fatalf("Wield: %v", err)
	}
	if err := c.Wear(armor.ID); err != nil {
		t.Fatalf("Wear: %v", err)
	}
	if err := c.ConsumeAmmo(3); err != nil {
		t.Fatalf("ConsumeAmmo: %v", err)
	}
	if _, _, err := c.ApplyWound("Chest", 5); err != nil {
		t.Fatalf("ApplyWound: %v", err)
	}
	c.AddCondition(Condition{Name: CondBleeding, Rounds: 12, Notes: "treated after the fight"})
	c.Dead = false
	return c
}

// TestCharacterJSONRoundTrip marshals a fully built character, unmarshals
// it, and requires a deep equality — every exported field must survive
// state.json persistence.
func TestCharacterJSONRoundTrip(t *testing.T) {
	t.Parallel()
	c := buildRoundTripCharacter(t)

	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back Character
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(c, &back) {
		t.Errorf("round trip changed the character:\n got %+v\nwant %+v", &back, c)
	}

	// The restored character must still work mechanically.
	if err := back.Refresh(); err != nil {
		t.Fatalf("Refresh after round trip: %v", err)
	}
	sev, err := back.SeverityAt("Chest")
	if err != nil || sev != SeverityMinor {
		t.Errorf("SeverityAt after round trip = %v, %v", sev, err)
	}
	if left, err := back.ArrowsLeft(); err != nil || left != 17 {
		t.Errorf("ArrowsLeft after round trip = %d, %v; want 17", left, err)
	}
	if back.Skills["Lockpicking"].Value <= back.Skills["Lockpicking"].Base {
		t.Error("spent career points lost in round trip")
	}
}

// TestCharacterJSONMinimal ensures the zero-value Character also round-trips
// (fresh NPCs before Refresh).
func TestCharacterJSONMinimal(t *testing.T) {
	t.Parallel()
	c := &Character{ID: "npc-1", Name: "Guard", Chars: Characteristics{STR: 10, CON: 10, SIZ: 10, DEX: 10, INT: 10, POW: 10, CHA: 10}}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back Character
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(c, &back) {
		t.Errorf("round trip changed the character:\n got %+v\nwant %+v", &back, c)
	}
}

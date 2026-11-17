package resolve

import (
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/combat"
)

// newState returns a session with a fixed seed for deterministic rolls.
func newState(t *testing.T, seed int64) *State {
	t.Helper()
	return NewState("test session", seed)
}

// addCharacter registers a refreshed character under the given ID (PC by
// default) with deterministic characteristics: INT 14 feeds Experience
// Rolls, POW 15 feeds Luck Points, STR+DEX 25 feeds combat bases.
func addCharacter(t *testing.T, s *State, id string, pc bool) *char.Character {
	t.Helper()
	c := &char.Character{
		ID:   id,
		Name: id,
		PC:   pc,
		Chars: char.Characteristics{
			STR: 14, CON: 13, SIZ: 12, DEX: 15, INT: 12, POW: 14, CHA: 10,
		},
	}
	if err := c.Refresh(); err != nil {
		t.Fatalf("refresh %s: %v", id, err)
	}
	s.Characters[id] = c
	return c
}

// learnStyle teaches a combat style covering the listed weapons.
func learnStyle(t *testing.T, c *char.Character, name string, value int, weapons ...string) {
	t.Helper()
	if err := c.SetCombatStyle(name, value); err != nil {
		t.Fatalf("SetCombatStyle %q: %v", name, err)
	}
	cs := c.CombatStyles[name]
	cs.Weapons = append(cs.Weapons, weapons...)
}

// giveWeapon adds a sample weapon to the inventory and wields it in the
// main hand.
func giveWeapon(t *testing.T, c *char.Character, weapon string) {
	t.Helper()
	item, err := char.NewWeapon(weapon)
	if err != nil {
		t.Fatalf("NewWeapon %q: %v", weapon, err)
	}
	if err := c.AddItem(&item); err != nil {
		t.Fatalf("AddItem %q: %v", weapon, err)
	}
	if err := c.Wield("main", item.ID); err != nil {
		t.Fatalf("Wield %q: %v", weapon, err)
	}
}

// duelState builds a running two-combatant encounter inside a state: an
// armed attacker and an armed defender, both ready for resolve moves.
func duelState(t *testing.T, seed int64) (*State, string, string) {
	t.Helper()
	s := newState(t, seed)
	att := addCharacter(t, s, "pc-1", true)
	def := addCharacter(t, s, "pc-2", true)
	learnStyle(t, att, "Broadsword Art", 70, "Broadsword")
	learnStyle(t, def, "Broadsword Art", 65, "Broadsword")
	giveWeapon(t, att, "Broadsword")
	giveWeapon(t, def, "Broadsword")
	if _, err := Apply(s, StartEncounter{Participants: []string{"pc-1", "pc-2"}}); err != nil {
		t.Fatalf("StartEncounter: %v", err)
	}
	return s, "pc-1", "pc-2"
}

// encounterWith builds a minimal started encounter around one character —
// the fixture for Normalize's structural checks.
func encounterWith(t *testing.T, c *char.Character) *combat.Encounter {
	t.Helper()
	e, err := combat.Start([]*char.Character{c})
	if err != nil {
		t.Fatalf("combat.Start: %v", err)
	}
	return e
}

// orderEntryAt builds a strike-order row pointing at a participant index.
func orderEntryAt(idx int) combat.OrderEntry {
	return combat.OrderEntry{Index: idx, Score: 5, DEX: 10}
}

// counterEntryAt builds a secret-counter row pointing at a participant
// index.
func counterEntryAt(idx int) combat.CounterEntry {
	return combat.CounterEntry{Index: idx, Against: "Impale"}
}

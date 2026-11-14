package combat

import (
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/dice"
)

// fighter returns a refreshed human combatant with deterministic
// characteristics (STR 14 / DEX 15 feeds combat bases, POW 14 feeds
// Endurance-adjacent skills) and a stable ID for encounter identity.
func fighter(t *testing.T, id, name string) *char.Character {
	t.Helper()
	c := &char.Character{
		ID:   id,
		Name: name,
		Chars: char.Characteristics{
			STR: 14, CON: 13, SIZ: 12, DEX: 15, INT: 12, POW: 14, CHA: 10,
		},
	}
	if err := c.Refresh(); err != nil {
		t.Fatalf("refresh %s: %v", name, err)
	}
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

// wearMail dresses the character in the sample Mail armour (AP 6 at
// every location).
func wearMail(t *testing.T, c *char.Character) {
	t.Helper()
	item, err := char.NewArmor("Mail")
	if err != nil {
		t.Fatalf("NewArmor: %v", err)
	}
	if err := c.AddItem(&item); err != nil {
		t.Fatalf("AddItem armor: %v", err)
	}
	if err := c.Wear(item.ID); err != nil {
		t.Fatalf("Wear: %v", err)
	}
}

// giveAmmo adds an ammunition stack of the given quantity.
func giveAmmo(t *testing.T, c *char.Character, name string, qty int) {
	t.Helper()
	item := &char.Item{Name: name, Qty: qty, Tags: []string{char.TagAmmunition}}
	if err := c.AddItem(item); err != nil {
		t.Fatalf("AddItem ammo: %v", err)
	}
}

// duel builds a running two-character encounter with a melee attacker
// (Broadsword) and a defender parrying with their own Broadsword.
func duel(t *testing.T, seed int64) (*dice.Rand, *Encounter, *char.Character, *char.Character) {
	t.Helper()
	att := fighter(t, "att-1", "Attacker")
	def := fighter(t, "def-1", "Defender")
	learnStyle(t, att, "Broadsword Art", 70, "Broadsword")
	learnStyle(t, def, "Broadsword Art", 65, "Broadsword")
	giveWeapon(t, att, "Broadsword")
	giveWeapon(t, def, "Broadsword")
	return duelBetween(t, seed, att, def)
}

// duelBetween starts an encounter over the given combatants.
func duelBetween(t *testing.T, seed int64, att, def *char.Character) (*dice.Rand, *Encounter, *char.Character, *char.Character) {
	t.Helper()
	e, err := Start([]*char.Character{att, def})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return dice.NewRand(seed), e, att, def
}

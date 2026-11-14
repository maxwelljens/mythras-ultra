package resolve

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/char"
)

// TestInventoryAdd covers sample-table items (statistics carried), plain
// narrative items, tags, and the carried-count delta facts.
func TestInventoryAdd(t *testing.T) {
	s := newState(t, 5)
	c := addCharacter(t, s, "pc-1", true)

	res, err := Apply(s, InventoryAdd{Actor: "pc-1", Item: "Broadsword"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	sword, err := c.FindItem("item-1")
	if err != nil {
		t.Fatalf("FindItem: %v", err)
	}
	if sword.Weapon == nil || sword.Qty != 1 || !slices.Contains(sword.Tags, char.TagWeapon) {
		t.Errorf("Broadsword = %+v, want a tagged weapon item with statistics", sword)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "added: Broadsword ×1 (item-1)") || !strings.Contains(joined, "carried: 0 → 1") {
		t.Errorf("facts = %v", res.Facts)
	}

	// Ammunition tags make the item count as ammo.
	if _, err := Apply(s, InventoryAdd{Actor: "pc-1", Item: "Arrows", Qty: 7, Tags: []string{char.TagAmmunition}}); err != nil {
		t.Fatalf("ammo add: %v", err)
	}
	if left, err := c.ArrowsLeft(); err != nil || left != 7 {
		t.Errorf("ArrowsLeft = %d, %v; want 7", left, err)
	}

	// A name the tables do not know still carries (plain, flagged).
	plain, err := Apply(s, InventoryAdd{Actor: "pc-1", Item: "Bedroll"})
	if err != nil {
		t.Fatalf("plain add: %v", err)
	}
	if !strings.Contains(strings.Join(plain.Facts, "\n"), "no game statistics") {
		t.Errorf("facts = %v, want the statless note", plain.Facts)
	}

	if _, err := Apply(s, InventoryAdd{Actor: "pc-1", Item: " "}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty item error = %v, want ErrInvalidArgument", err)
	}
	if _, err := Apply(s, InventoryAdd{Actor: "ghost", Item: "Rope"}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown actor error = %v, want ErrUnknownCharacter", err)
	}
}

// TestInventoryRemove covers partial and whole-stack removal with
// before → after facts, and the quantity refusal.
func TestInventoryRemove(t *testing.T) {
	s := newState(t, 5)
	c := addCharacter(t, s, "pc-1", true)
	if _, err := Apply(s, InventoryAdd{Actor: "pc-1", Item: "Arrows", Qty: 7, Tags: []string{char.TagAmmunition}}); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	res, err := Apply(s, InventoryRemove{Actor: "pc-1", Item: "item-1", Qty: 6})
	if err != nil {
		t.Fatalf("partial remove: %v", err)
	}
	if c.Inventory[0].Qty != 1 {
		t.Errorf("Qty = %d, want 1", c.Inventory[0].Qty)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "Arrows: 7 → 1") || !strings.Contains(joined, "carried: 7 → 1") {
		t.Errorf("facts = %v, want 7 → 1 deltas", res.Facts)
	}

	// Whole stack: the entry leaves the inventory; by name works too.
	if _, err := Apply(s, InventoryRemove{Actor: "pc-1", Item: "Arrows"}); err != nil {
		t.Fatalf("full remove: %v", err)
	}
	if len(c.Inventory) != 0 {
		t.Errorf("inventory = %+v, want empty", c.Inventory)
	}
	if _, err := c.ArrowsLeft(); err == nil {
		t.Error("ammo still reported after full removal")
	}

	if _, err := Apply(s, InventoryRemove{Actor: "pc-1", Item: "item-1"}); !errors.Is(err, char.ErrItemNotFound) {
		t.Errorf("unknown item error = %v, want char.ErrItemNotFound", err)
	}
	if _, err := Apply(s, InventoryRemove{Actor: "pc-1", Item: " "}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty ref error = %v, want ErrInvalidArgument", err)
	}

	// Over-removal refuses with both amounts, mutating nothing.
	if _, err := Apply(s, InventoryAdd{Actor: "pc-1", Item: "Rations", Qty: 2}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := Apply(s, InventoryRemove{Actor: "pc-1", Item: "Rations", Qty: 5}); !errors.Is(err, ErrNotEnoughItems) {
		t.Fatalf("over-remove error = %v, want ErrNotEnoughItems", err)
	}
	// The refusal rolled the aggregate back: re-query before asserting.
	cur := s.Characters["pc-1"]
	if len(cur.Inventory) != 1 || cur.Inventory[0].Qty != 2 {
		t.Errorf("refused removal mutated inventory: %+v", cur.Inventory)
	}
}

// TestEquip covers wielding into slots (with the swap-out fact), wearing,
// and the item refusals.
func TestEquip(t *testing.T) {
	s := newState(t, 5)
	c := addCharacter(t, s, "pc-1", true)
	if _, err := Apply(s, InventoryAdd{Actor: "pc-1", Item: "Broadsword"}); err != nil {
		t.Fatalf("add sword: %v", err)
	}
	if _, err := Apply(s, InventoryAdd{Actor: "pc-1", Item: "Dagger"}); err != nil {
		t.Fatalf("add dagger: %v", err)
	}
	if _, err := Apply(s, InventoryAdd{Actor: "pc-1", Item: "Mail"}); err != nil {
		t.Fatalf("add mail: %v", err)
	}

	if _, err := Apply(s, Equip{Actor: "pc-1", Item: "item-1", Slot: "main"}); err != nil {
		t.Fatalf("wield: %v", err)
	}
	sword, _ := c.FindItem("item-1")
	if !sword.Equipped || sword.Slot != "main" {
		t.Errorf("sword = %+v, want wielded in main", sword)
	}

	// A second main-hand item swaps the first out.
	res, err := Apply(s, Equip{Actor: "pc-1", Item: "item-2", Slot: "main"})
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	dagger, _ := c.FindItem("item-2")
	if !dagger.Equipped || dagger.Slot != "main" {
		t.Errorf("dagger = %+v, want wielded in main", dagger)
	}
	if sword.Equipped {
		t.Error("previous main-hand item still equipped")
	}
	if !strings.Contains(strings.Join(res.Facts, "\n"), "Broadsword → Dagger") {
		t.Errorf("facts = %v, want the slot swap diff", res.Facts)
	}

	// Worn gear: empty slot.
	if _, err := Apply(s, Equip{Actor: "pc-1", Item: "item-3"}); err != nil {
		t.Fatalf("wear: %v", err)
	}
	mail, _ := c.FindItem("item-3")
	if !mail.Equipped || mail.Slot != "" {
		t.Errorf("mail = %+v, want worn", mail)
	}

	if _, err := Apply(s, Equip{Actor: "pc-1", Item: "item-9"}); !errors.Is(err, char.ErrItemNotFound) {
		t.Errorf("unknown item error = %v, want char.ErrItemNotFound", err)
	}
	if _, err := Apply(s, Equip{Actor: "ghost", Item: "item-1", Slot: "main"}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown actor error = %v, want ErrUnknownCharacter", err)
	}
}

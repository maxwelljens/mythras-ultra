package char

import (
	"errors"
	"fmt"
	"slices"
)

// Item tags. Items carry Tags rather than subclassing so the GM can add
// improvised entries without new types.
const (
	// TagWeapon marks a weapon (melee or ranged).
	TagWeapon = "weapon"
	// TagMelee marks a melee weapon.
	TagMelee = "melee"
	// TagRanged marks a ranged weapon.
	TagRanged = "ranged"
	// TagArmor marks worn protection with Armor Points.
	TagArmor = "armor"
	// TagShield marks a shield.
	TagShield = "shield"
	// TagAmmunition marks consumable ammunition (arrows, bolts, shots).
	TagAmmunition = "ammunition"
	// TagClothing marks ordinary clothing.
	TagClothing = "clothing"
)

// Item is one inventory entry. Mechanical statistics are typed and
// optional: Weapon for weapons, Armor for worn protection, Shield for
// shields; everything else (clothing, tools, trinkets) is name, quantity
// and tags only.
//
// Mythras Imperative has no currency, price list or encumbrance rules
// (digest §9), so Item carries no cost or weight: Carrying returns an
// advisory quantity count only.
type Item struct {
	// ID is the inventory identifier ("item-N", see NextID).
	ID string `json:"id,omitempty"`
	// Name is the item's display name.
	Name string `json:"name"`
	// Qty is the quantity carried; AddItem defaults a non-positive quantity
	// to 1. Ammunition decrements via ConsumeAmmo.
	Qty int `json:"qty,omitempty"`
	// Tags classify the item (see the Tag* constants).
	Tags []string `json:"tags,omitempty"`
	// Equipped reports whether the item is currently worn or wielded.
	Equipped bool `json:"equipped,omitempty"`
	// Slot names the equip slot for wielded items ("main", "off", ...);
	// empty for worn items and stowed gear.
	Slot string `json:"slot,omitempty"`
	// Weapon holds weapon statistics, if this is a weapon.
	Weapon *WeaponStats `json:"weapon,omitempty"`
	// Armor holds armor statistics, if this is worn protection.
	Armor *ArmorStats `json:"armor,omitempty"`
	// Shield holds shield statistics, if this is a shield.
	Shield *ShieldStats `json:"shield,omitempty"`
	// Notes is free text (printed weapon notes, GM annotations).
	Notes string `json:"notes,omitempty"`
}

// hasTag reports whether the item carries the given tag.
func (it *Item) hasTag(tag string) bool {
	if it == nil {
		return false
	}
	return slices.Contains(it.Tags, tag)
}

// itemIDs returns the IDs of the current inventory (for NextID).
func (c *Character) itemIDs() []string {
	ids := make([]string, 0, len(c.Inventory))
	for _, it := range c.Inventory {
		if it != nil {
			ids = append(ids, it.ID)
		}
	}
	return ids
}

// FindItem returns the inventory item with the given ID. Errors wrap
// [ErrItemNotFound].
func (c *Character) FindItem(id string) (*Item, error) {
	if c == nil {
		return nil, errors.New("char: nil character")
	}
	for _, it := range c.Inventory {
		if it != nil && it.ID == id {
			return it, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrItemNotFound, id)
}

// AddItem appends an item to the inventory. An item without an ID gets the
// next "item-N" identifier; a non-positive quantity becomes 1; a duplicate
// ID is rejected.
func (c *Character) AddItem(item *Item) error {
	if c == nil || item == nil {
		return errors.New("char: nil character or item")
	}
	if item.Name == "" {
		return errors.New("char: item needs a name")
	}
	if item.Qty <= 0 {
		item.Qty = 1
	}
	if item.ID == "" {
		item.ID = NextID("item", c.itemIDs())
	} else if _, err := c.FindItem(item.ID); err == nil {
		return fmt.Errorf("char: duplicate item ID %q", item.ID)
	}
	c.Inventory = append(c.Inventory, item)
	return nil
}

// Wield equips the item with the given inventory ID into the named weapon
// slot ("main", "off", or any non-empty name), unequipping whatever
// occupied that slot. The engine does not police which item is wielded —
// improvised weapons are a GM call.
//
// Errors wrap [ErrInvalidSlot] (empty slot) or [ErrItemNotFound].
func (c *Character) Wield(slot, id string) error {
	if slot == "" {
		return fmt.Errorf("%w: empty slot name", ErrInvalidSlot)
	}
	item, err := c.FindItem(id)
	if err != nil {
		return err
	}
	for _, other := range c.Inventory {
		if other != nil && other != item && other.Equipped && other.Slot == slot {
			other.Equipped = false
			other.Slot = ""
		}
	}
	item.Equipped = true
	item.Slot = slot
	return nil
}

// Wear equips the item with the given inventory ID as worn gear (armor,
// clothing). Worn items stack for ArmorAt; use Unequip to remove them.
// Errors wrap [ErrItemNotFound].
func (c *Character) Wear(id string) error {
	item, err := c.FindItem(id)
	if err != nil {
		return err
	}
	item.Equipped = true
	item.Slot = ""
	return nil
}

// Unequip stops equipping the item with the given inventory ID (it stays in
// the inventory). Errors wrap [ErrItemNotFound].
func (c *Character) Unequip(id string) error {
	item, err := c.FindItem(id)
	if err != nil {
		return err
	}
	item.Equipped = false
	item.Slot = ""
	return nil
}

// ArrowsLeft returns the total quantity of ammunition carried (inventory
// items tagged [TagAmmunition]) — the "arrows in the quiver" count for
// ranged fire. When the inventory holds no ammunition item at all the error
// wraps [ErrNoAmmo].
func (c *Character) ArrowsLeft() (int, error) {
	if c == nil {
		return 0, errors.New("char: nil character")
	}
	ammo := false
	total := 0
	for _, it := range c.Inventory {
		if it != nil && it.hasTag(TagAmmunition) {
			ammo = true
			if it.Qty > 0 {
				total += it.Qty
			}
		}
	}
	if !ammo {
		return 0, fmt.Errorf("%w: no item tagged %q", ErrNoAmmo, TagAmmunition)
	}
	return total, nil
}

// ConsumeAmmo removes n rounds of ammunition, drawing from ammunition items
// in inventory order; items emptied by the consumption are removed from the
// inventory. If fewer than n rounds are carried, nothing is consumed and
// the error wraps [ErrNoAmmo]. A negative n wraps [ErrInvalidAmount]; n == 0
// consumes nothing.
func (c *Character) ConsumeAmmo(n int) error {
	if c == nil {
		return errors.New("char: nil character")
	}
	if n < 0 {
		return fmt.Errorf("%w: consume %d rounds", ErrInvalidAmount, n)
	}
	if n == 0 {
		return nil
	}
	// Validate the whole amount before consuming anything: a failed shot
	// must never half-empty the quiver.
	total, err := c.ArrowsLeft()
	if err != nil {
		return err
	}
	if total < n {
		return fmt.Errorf("%w: need %d rounds, have %d", ErrNoAmmo, n, total)
	}
	remaining := n
	kept := c.Inventory[:0]
	for _, it := range c.Inventory {
		if remaining > 0 && it != nil && it.hasTag(TagAmmunition) && it.Qty > 0 {
			take := min(it.Qty, remaining)
			it.Qty -= take
			remaining -= take
			if it.Qty == 0 {
				continue // drop the emptied stack
			}
		}
		kept = append(kept, it)
	}
	c.Inventory = kept
	return nil
}

// Carrying returns the total quantity of items carried — an advisory bulk
// count for the UI and audit only. Mythras Imperative defines no
// encumbrance rules (digest §9), so nothing in the engine penalises this
// value.
func (c *Character) Carrying() int {
	if c == nil {
		return 0
	}
	total := 0
	for _, it := range c.Inventory {
		if it != nil && it.Qty > 0 {
			total += it.Qty
		}
	}
	return total
}

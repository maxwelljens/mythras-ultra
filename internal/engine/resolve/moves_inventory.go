package resolve

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mythras-ultra/myul/internal/engine/char"
)

// InventoryAdd adds an item to a character's inventory (tool:
// inventory_add). Sample-table names — weapons, armour and shields — are
// taken with their printed game statistics; any other name becomes a plain
// statless item so the GM can carry narrative gear. Mythras Imperative has
// no prices or encumbrance rules (digest §9), so only the advisory carried
// count is reported.
type InventoryAdd struct {
	// Actor is the character receiving the item (ID).
	Actor string `json:"actor"`
	// Item is the item's name (sample-table name or free text).
	Item string `json:"item"`
	// Qty is the quantity; <= 0 means 1.
	Qty int `json:"qty,omitempty"`
	// Tags classifies the item (see the char.Tag* constants), e.g.
	// "ammunition".
	Tags []string `json:"tags,omitempty"`
}

// Kind implements [Move].
func (m InventoryAdd) Kind() string { return "inventory_add" }

// Apply validates the actor and name, builds the item, and appends it
// through char.Character.AddItem; the audit reports the new item ID and the
// carried count before → after.
func (m InventoryAdd) Apply(s *State) (Result, error) {
	c, err := s.character(m.Actor)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(m.Item) == "" {
		return Result{}, fmt.Errorf("%w: empty item name", ErrInvalidArgument)
	}
	item, detailed := buildItem(m.Item, m.Qty)
	for _, tag := range m.Tags { // keep the table's tags, add the caller's
		if !slices.Contains(item.Tags, tag) {
			item.Tags = append(item.Tags, tag)
		}
	}
	carriedBefore := c.Carrying()
	if err := c.AddItem(item); err != nil {
		return Result{}, err
	}
	carriedAfter := c.Carrying()
	line := fmt.Sprintf("added: %s ×%d (%s)", item.Name, item.Qty, item.ID)
	if !detailed {
		line += " — no game statistics"
	}
	facts := []string{
		line,
		change("carried", carriedBefore, carriedAfter),
	}
	changes := []string{fmt.Sprintf("inventory: +%s ×%d (%s)", item.Name, item.Qty, item.ID)}
	if carriedBefore != carriedAfter {
		changes = append(changes, change("carried", carriedBefore, carriedAfter))
	}
	summary := fmt.Sprintf("%s received %s ×%d (%s)", c.ID, item.Name, item.Qty, item.ID)
	return newResult(summary, facts, Audit{Changes: changes}), nil
}

// InventoryRemove removes quantity from a stack — or the whole stack — of
// an inventory item (tool: inventory_remove). The item is matched by its
// engine ID first, then by name; removal of an equipped item simply drops
// it from the inventory.
type InventoryRemove struct {
	// Actor is the character losing the item (ID).
	Actor string `json:"actor"`
	// Item is the item's ID ("item-3") or inventory name.
	Item string `json:"item"`
	// Qty is the quantity to remove; <= 0 removes 1.
	Qty int `json:"qty,omitempty"`
}

// Kind implements [Move].
func (m InventoryRemove) Kind() string { return "inventory_remove" }

// Apply resolves the item and validates the quantity against what is
// carried before removing anything (char has no remove API — the exported
// inventory slice is edited here, the same bookkeeping layer char documents
// for engine operations).
func (m InventoryRemove) Apply(s *State) (Result, error) {
	c, err := s.character(m.Actor)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(m.Item) == "" {
		return Result{}, fmt.Errorf("%w: empty item reference", ErrInvalidArgument)
	}
	item, err := findItem(c, m.Item)
	if err != nil {
		return Result{}, err
	}
	qty := m.Qty
	if qty <= 0 {
		qty = 1
	}
	if item.Qty < qty {
		return Result{}, fmt.Errorf("%w: %q holds %d, asked to remove %d", ErrNotEnoughItems, item.Name, item.Qty, qty)
	}
	carriedBefore := c.Carrying()
	removed := item.Qty
	if item.Qty == qty {
		// Whole stack: drop the entry (order preserved).
		kept := make([]*char.Item, 0, len(c.Inventory)-1)
		for _, it := range c.Inventory {
			if it != item {
				kept = append(kept, it)
			}
		}
		c.Inventory = kept
	} else {
		item.Qty -= qty
	}
	carriedAfter := c.Carrying()
	facts := []string{
		change(item.Name, removed, carriedStack(c, item)),
		change("carried", carriedBefore, carriedAfter),
	}
	changes := []string{fmt.Sprintf("inventory: -%s ×%d (%s)", item.Name, qty, item.ID)}
	if carriedBefore != carriedAfter {
		changes = append(changes, change("carried", carriedBefore, carriedAfter))
	}
	summary := fmt.Sprintf("%s lost %s ×%d (%s)", c.ID, item.Name, qty, item.ID)
	return newResult(summary, facts, Audit{Changes: changes}), nil
}

// Equip readies an inventory item: an empty slot wears it (armour,
// clothing), a named slot wields it in that weapon slot, unequipping
// whatever occupied it (tool: equip; char.Wield / char.Wear).
type Equip struct {
	// Actor is the character equipping the item (ID).
	Actor string `json:"actor"`
	// Item is the item's ID ("item-3") or inventory name.
	Item string `json:"item"`
	// Slot is the weapon slot to wield into ("main", "off", ...); empty
	// means worn.
	Slot string `json:"slot,omitempty"`
}

// Kind implements [Move].
func (m Equip) Kind() string { return "equip" }

// Apply resolves the item, records what the slot held before, then equips
// through the char APIs. Errors wrap char.ErrItemNotFound or
// char.ErrInvalidSlot.
func (m Equip) Apply(s *State) (Result, error) {
	c, err := s.character(m.Actor)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(m.Item) == "" {
		return Result{}, fmt.Errorf("%w: empty item reference", ErrInvalidArgument)
	}
	item, err := findItem(c, m.Item)
	if err != nil {
		return Result{}, err
	}
	before := ""
	what := "worn"
	if m.Slot == "" {
		// Worn gear stacks (several items may be worn), so there is no
		// single slot holder to diff against.
		if err := c.Wear(item.ID); err != nil {
			return Result{}, err
		}
	} else {
		what = "wielded in " + m.Slot
		for _, other := range c.Inventory {
			if other != nil && other.Equipped && other.Slot == m.Slot && other != item {
				before = other.Name
			}
		}
		if err := c.Wield(m.Slot, item.ID); err != nil {
			return Result{}, err
		}
	}
	var line string
	if before != "" && before != item.Name {
		line = change(what, before, item.Name)
	} else {
		line = fmt.Sprintf("%s: %s (%s)", what, item.Name, item.ID)
	}
	facts := []string{line}
	changes := []string{"equipment: " + line}
	summary := fmt.Sprintf("%s readies %s (%s)", c.ID, item.Name, what)
	return newResult(summary, facts, Audit{Changes: changes}), nil
}

// buildItem resolves a name into an inventory item: the sample weapon,
// armour and shield tables first (detailed reports whether printed game
// statistics were found), then a plain statless item. Qty <= 0 becomes 1.
func buildItem(name string, qty int) (*char.Item, bool) {
	if qty <= 0 {
		qty = 1
	}
	ctors := []func(string) (char.Item, error){char.NewWeapon, char.NewArmor, char.NewShield}
	for _, ctor := range ctors {
		if item, err := ctor(name); err == nil {
			item.Qty = qty
			return &item, true
		}
	}
	return &char.Item{Name: name, Qty: qty}, false
}

// findItem resolves a reference to an inventory entry: an exact ID match
// first (the engine-generated "item-N" the GM should be passing), then the
// first item with that exact name. Errors wrap char.ErrItemNotFound,
// listing the inventory so the harness can answer IMPOSSIBLE with real
// choices.
func findItem(c *char.Character, ref string) (*char.Item, error) {
	if item, err := c.FindItem(ref); err == nil {
		return item, nil
	}
	for _, it := range c.Inventory {
		if it != nil && it.Name == ref {
			return it, nil
		}
	}
	ids := make([]string, 0, len(c.Inventory))
	for _, it := range c.Inventory {
		if it != nil {
			ids = append(ids, fmt.Sprintf("%s (%s ×%d)", it.ID, it.Name, it.Qty))
		}
	}
	return nil, fmt.Errorf("%w %q (inventory: %s)", char.ErrItemNotFound, ref, strings.Join(ids, ", "))
}

// carriedStack reports an item's remaining stack after removal: 0 once the
// entry is gone (findItem never returns a dropped entry, so 0 means the
// whole stack left the inventory).
func carriedStack(c *char.Character, item *char.Item) int {
	if !slices.Contains(c.Inventory, item) {
		return 0
	}
	return item.Qty
}

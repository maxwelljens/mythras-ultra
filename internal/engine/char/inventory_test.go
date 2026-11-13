package char

import (
	"errors"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/dice"
)

// TestInventoryBasics covers AddItem ID assignment/defaults, duplicate and
// missing-ID rejection, and Carrying (advisory only — no encumbrance rules
// exist, digest §9).
func TestInventoryBasics(t *testing.T) {
	t.Parallel()
	c := newTestCharacter(t)

	sword, err := NewWeapon("Longsword")
	if err != nil {
		t.Fatalf("NewWeapon: %v", err)
	}
	if err := c.AddItem(&sword); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if sword.ID != "item-1" {
		t.Errorf("first item ID = %q, want item-1", sword.ID)
	}
	// Quantity defaults to 1; the item carries its typed stats.
	if sword.Qty != 1 || sword.Weapon == nil || sword.Weapon.Damage != "1d8" {
		t.Errorf("sword = %+v, want qty 1 with weapon stats", sword)
	}

	arrows := &Item{Name: "Arrows", Qty: 24, Tags: []string{TagAmmunition}}
	if err := c.AddItem(arrows); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if arrows.ID != "item-2" {
		t.Errorf("second item ID = %q, want item-2", arrows.ID)
	}
	if got := c.Carrying(); got != 25 {
		t.Errorf("Carrying() = %d, want 25", got)
	}

	// A non-positive quantity is normalised to 1.
	shield, err := NewShield("Buckler")
	if err != nil {
		t.Fatalf("NewShield: %v", err)
	}
	shield.Qty = 0
	if err := c.AddItem(&shield); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if shield.Qty != 1 {
		t.Errorf("shield qty = %d, want 1", shield.Qty)
	}

	// Duplicate IDs and nameless items are refused.
	dupe := &Item{ID: arrows.ID, Name: "Clone"}
	if err := c.AddItem(dupe); err == nil {
		t.Error("duplicate item ID should fail")
	}
	if err := c.AddItem(&Item{Name: ""}); err == nil {
		t.Error("nameless item should fail")
	}
	if _, err := c.FindItem("item-404"); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("FindItem: err = %v, want ErrItemNotFound", err)
	}
	if _, err := c.FindItem(arrows.ID); err != nil {
		t.Errorf("FindItem(arrows): %v", err)
	}
}

// TestWieldWear checks slot mechanics: wielding displaces the slot's
// current occupant, wearing stacks, Unequip clears both.
func TestWieldWear(t *testing.T) {
	t.Parallel()
	c := newTestCharacter(t)
	sword, _ := NewWeapon("Longsword")
	axe, _ := NewWeapon("Axe")
	mail, _ := NewArmor("Mail")
	for _, it := range []*Item{&sword, &axe, &mail} {
		if err := c.AddItem(it); err != nil {
			t.Fatalf("AddItem: %v", err)
		}
	}

	if err := c.Wield("main", sword.ID); err != nil {
		t.Fatalf("Wield: %v", err)
	}
	if !sword.Equipped || sword.Slot != "main" {
		t.Errorf("sword = %+v, want equipped in main", sword)
	}
	// A second "main" weapon displaces the first.
	if err := c.Wield("main", axe.ID); err != nil {
		t.Fatalf("Wield: %v", err)
	}
	if sword.Equipped || sword.Slot != "" {
		t.Errorf("first sword should be stowed: %+v", sword)
	}
	if !axe.Equipped || axe.Slot != "main" {
		t.Errorf("axe = %+v, want equipped in main", axe)
	}
	// Off-hand keeps both armed.
	if err := c.Wield("off", sword.ID); err != nil {
		t.Fatalf("Wield: %v", err)
	}
	if !sword.Equipped || !axe.Equipped {
		t.Error("main and off should both be equipped")
	}

	if err := c.Wear(mail.ID); err != nil {
		t.Fatalf("Wear: %v", err)
	}
	if !mail.Equipped || mail.Slot != "" {
		t.Errorf("mail = %+v, want worn without slot", mail)
	}
	if err := c.Unequip(mail.ID); err != nil {
		t.Fatalf("Unequip: %v", err)
	}
	if mail.Equipped {
		t.Error("mail should be unequipped")
	}

	if err := c.Wield("", sword.ID); !errors.Is(err, ErrInvalidSlot) {
		t.Errorf("empty slot: err = %v, want ErrInvalidSlot", err)
	}
	if err := c.Wield("main", "item-404"); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("missing item: err = %v, want ErrItemNotFound", err)
	}
	if err := c.Wear("item-404"); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("missing item: err = %v, want ErrItemNotFound", err)
	}
}

// TestAmmunition covers ArrowsLeft/ConsumeAmmo: staged draws in inventory
// order, whole-amount validation (a failed shot never half-empties the
// quiver), stack removal at 0, and the sentinels.
func TestAmmunition(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*Character, *Item, *Item) {
		t.Helper()
		c := newTestCharacter(t)
		arrows := &Item{Name: "Arrows", Qty: 10, Tags: []string{TagAmmunition}}
		bolts := &Item{Name: "Bolts", Qty: 5, Tags: []string{TagAmmunition}}
		if err := c.AddItem(arrows); err != nil {
			t.Fatalf("AddItem: %v", err)
		}
		if err := c.AddItem(bolts); err != nil {
			t.Fatalf("AddItem: %v", err)
		}
		return c, arrows, bolts
	}

	t.Run("counts and staged consumption", func(t *testing.T) {
		c, arrows, bolts := setup(t)
		if left, err := c.ArrowsLeft(); err != nil || left != 15 {
			t.Fatalf("ArrowsLeft() = %d, %v; want 15", left, err)
		}
		if err := c.ConsumeAmmo(3); err != nil {
			t.Fatalf("ConsumeAmmo(3): %v", err)
		}
		if arrows.Qty != 7 || bolts.Qty != 5 {
			t.Errorf("qty after 3 rounds = arrows %d, bolts %d; want 7/5 (inventory order)", arrows.Qty, bolts.Qty)
		}
		// Consume 10 more: the 7 remaining arrows, then 3 bolts.
		if err := c.ConsumeAmmo(10); err != nil {
			t.Fatalf("ConsumeAmmo(10): %v", err)
		}
		if arrows.Qty != 0 || bolts.Qty != 2 {
			t.Errorf("qty after 10 more = arrows %d, bolts %d; want 0/2", arrows.Qty, bolts.Qty)
		}
		if left, _ := c.ArrowsLeft(); left != 2 {
			t.Errorf("ArrowsLeft() = %d, want 2", left)
		}
	})

	t.Run("emptied stacks are removed", func(t *testing.T) {
		c, _, _ := setup(t)
		if err := c.ConsumeAmmo(15); err != nil {
			t.Fatalf("ConsumeAmmo(15): %v", err)
		}
		if len(c.Inventory) != 0 {
			t.Errorf("inventory = %d entries, want all ammunition stacks removed", len(c.Inventory))
		}
		if _, err := c.ArrowsLeft(); !errors.Is(err, ErrNoAmmo) {
			t.Errorf("ArrowsLeft after empty: err = %v, want ErrNoAmmo", err)
		}
	})

	t.Run("insufficient ammunition consumes nothing", func(t *testing.T) {
		c, arrows, bolts := setup(t)
		if err := c.ConsumeAmmo(16); !errors.Is(err, ErrNoAmmo) {
			t.Fatalf("err = %v, want ErrNoAmmo", err)
		}
		if arrows.Qty != 10 || bolts.Qty != 5 {
			t.Errorf("failed consumption changed qty: %d/%d", arrows.Qty, bolts.Qty)
		}
	})

	t.Run("argument validation", func(t *testing.T) {
		c := newTestCharacter(t)
		if _, err := c.ArrowsLeft(); !errors.Is(err, ErrNoAmmo) {
			t.Errorf("no ammunition at all: err = %v, want ErrNoAmmo", err)
		}
		if err := c.ConsumeAmmo(-1); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("negative: err = %v, want ErrInvalidAmount", err)
		}
		if err := c.ConsumeAmmo(0); err != nil {
			t.Errorf("zero: %v", err)
		}
	})
}

// TestEquipmentTables spot-checks the sample tables (tables.md §5.1–§5.4)
// and their lookups, including the melee-first "Dagger" resolution.
func TestEquipmentTables(t *testing.T) {
	t.Parallel()
	if len(MeleeWeapons()) != 14 {
		t.Errorf("MeleeWeapons() = %d rows, want 14 (tables.md §5.3)", len(MeleeWeapons()))
	}
	if len(RangedWeapons()) != 12 {
		t.Errorf("RangedWeapons() = %d rows, want 12 (tables.md §5.4)", len(RangedWeapons()))
	}
	if len(Armors()) != 21 {
		t.Errorf("Armors() = %d rows, want 21 (tables.md §5.1)", len(Armors()))
	}
	if len(Shields()) != 5 {
		t.Errorf("Shields() = %d rows, want 5 (tables.md §5.2)", len(Shields()))
	}

	spear, err := WeaponByName("2H Spear")
	if err != nil {
		t.Fatalf("WeaponByName: %v", err)
	}
	if spear.Stats.Damage != "1d10+1" || spear.Stats.Size != "L" || spear.Stats.AP != 4 || spear.Stats.HP != 10 {
		t.Errorf("2H Spear = %+v, want 1d10+1/L/4/10", spear.Stats)
	}
	bow, err := WeaponByName("Bow")
	if err != nil {
		t.Fatalf("WeaponByName: %v", err)
	}
	if bow.Stats.Range != "15/100/200" || bow.Stats.Force != "L" || bow.Stats.Load != 1 || !bow.Stats.DMAdds {
		t.Errorf("Bow = %+v, want range 15/100/200, Force L, Load 1, DM applies", bow.Stats)
	}
	// "Dagger" resolves the melee row first; the ranged row stays reachable.
	dagger, err := WeaponByName("Dagger")
	if err != nil || dagger.Stats.AP != 6 || dagger.Stats.Range != "" {
		t.Errorf("Dagger = %+v, %v; want the melee row (AP 6)", dagger, err)
	}
	ranged := RangedWeapons()
	found := false
	for _, e := range ranged {
		if e.Name == "Dagger" && e.Stats.Damage == "1d4" && e.Stats.Range == "5/10/20" {
			found = true
		}
	}
	if !found {
		t.Error("ranged Dagger row (1d4, 5/10/20) missing")
	}
	if _, err := WeaponByName("Vibrocannon"); !errors.Is(err, ErrUnknownEquipment) {
		t.Errorf("unknown weapon: err = %v, want ErrUnknownEquipment", err)
	}

	mail, err := ArmorByName("Mail")
	if err != nil || mail.AP != 6 {
		t.Errorf("Mail = %+v, %v; want AP 6", mail, err)
	}
	if _, err := ArmorByName("Plot armor"); !errors.Is(err, ErrUnknownEquipment) {
		t.Errorf("unknown armor: err = %v, want ErrUnknownEquipment", err)
	}
	kite, err := ShieldByName("Kite")
	if err != nil || kite.Stats.BlockLocations != 4 || kite.Stats.Size != "H" {
		t.Errorf("Kite = %+v, %v; want 4 blocking locations, size H", kite, err)
	}
	if _, err := ShieldByName("Tower Shield Deluxe"); !errors.Is(err, ErrUnknownEquipment) {
		t.Errorf("unknown shield: err = %v, want ErrUnknownEquipment", err)
	}
}

// TestEquipmentDamageExpressionsParse rolls every printed damage
// expression so the combat layer can never meet an unparseable one.
func TestEquipmentDamageExpressionsParse(t *testing.T) {
	t.Parallel()
	r := dice.NewRand(99)
	check := func(kind, name, expr string) {
		if _, err := r.Roll(expr); err != nil {
			t.Errorf("%s %s damage %q: %v", kind, name, expr, err)
		}
	}
	for _, e := range MeleeWeapons() {
		check("melee", e.Name, e.Stats.Damage)
	}
	for _, e := range RangedWeapons() {
		check("ranged", e.Name, e.Stats.Damage)
	}
	for _, e := range Shields() {
		check("shield", e.Name, e.Stats.Damage)
	}
}

// TestArmorAt checks armor stacking: natural catalog AP + every equipped
// item covering the location (tables.md §5.1 wildcard coverage; Combat
// How Combat Works step 7), and HighestArmorAP for the Initiative penalty.
func TestArmorAt(t *testing.T) {
	t.Parallel()
	t.Run("worn armor stacks on natural", func(t *testing.T) {
		c := &Character{
			Chars:              Characteristics{STR: 18, CON: 16, SIZ: 22, DEX: 12, INT: 12, POW: 12, CHA: 10},
			HitLocationCatalog: "minotaur",
		}
		if err := c.Refresh(); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		// Minotaur head has 3 natural AP; the body has 0.
		if got, err := c.ArmorAt("Head"); err != nil || got != 3 {
			t.Fatalf("ArmorAt(Head) = %d, %v; want 3 natural", got, err)
		}
		mail, _ := NewArmor("Mail")
		if err := c.AddItem(&mail); err != nil {
			t.Fatalf("AddItem: %v", err)
		}
		if err := c.Wear(mail.ID); err != nil {
			t.Fatalf("Wear: %v", err)
		}
		if got, err := c.ArmorAt("Head"); err != nil || got != 9 {
			t.Errorf("ArmorAt(Head) = %d, %v; want 9 (3 natural + 6 mail)", got, err)
		}
		if got, err := c.ArmorAt("Chest"); err != nil || got != 6 {
			t.Errorf("ArmorAt(Chest) = %d, %v; want 6 (mail only)", got, err)
		}
		if got := c.HighestArmorAP(); got != 6 {
			t.Errorf("HighestArmorAP() = %d, want 6", got)
		}
		if _, err := c.ArmorAt("Tail"); !errors.Is(err, ErrInvalidLocation) {
			t.Errorf("unknown location: err = %v, want ErrInvalidLocation", err)
		}
	})

	t.Run("unworn armor counts for nothing", func(t *testing.T) {
		c := newTestCharacter(t)
		gambeson, _ := NewArmor("Aketon")
		if err := c.AddItem(&gambeson); err != nil {
			t.Fatalf("AddItem: %v", err)
		}
		if got, err := c.ArmorAt("Chest"); err != nil || got != 0 {
			t.Errorf("unworn armor counted: %d, %v", got, err)
		}
		if err := c.Wear(gambeson.ID); err != nil {
			t.Fatalf("Wear: %v", err)
		}
		if got, err := c.ArmorAt("Chest"); err != nil || got != 2 {
			t.Errorf("worn armor missing: %d, %v", got, err)
		}
	})
}

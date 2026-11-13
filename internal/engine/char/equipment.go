package char

import (
	"errors"
	"fmt"
	"slices"
)

// WeaponStats are a weapon's mechanical statistics, transcribed from the
// sample tables: melee from tables.md §5.3, ranged from §5.4 (Combat ›
// Arms and Armor). Fields the sample tables do not print (DamageType,
// Reach) are left for the GM or a setting to supply — the engine does not
// invent them.
type WeaponStats struct {
	// Damage is the weapon's damage dice expression (e.g. "1d8+1").
	Damage string `json:"damage,omitempty"`
	// DamageType is a setting/GM label ("bludgeoning", "cutting", ...):
	// the sample tables print notes, not damage types.
	DamageType string `json:"damageType,omitempty"`
	// Reach is the melee reach in metres; 0 means unspecified — the sample
	// tables print no reach column.
	Reach int `json:"reach,omitempty"`
	// Size is the weapon Size (S/M/L/H/E), governing parry capacity
	// (Combat › Damage Reduction).
	Size string `json:"size,omitempty"`
	// AP is the weapon's Armor Points (its protection when parrying, and
	// what a struck weapon must beat to be damaged).
	AP int `json:"ap,omitempty"`
	// HP is the weapon's Hit Points; 0 means the table prints none ("—").
	HP int `json:"hp,omitempty"`
	// Range is a ranged weapon's Close/Effective/Long bands in metres,
	// printed as "15/100/200".
	Range string `json:"range,omitempty"`
	// Force is the ranged weapon's Force (S/M/L/H/E); empty when the table
	// prints "—".
	Force string `json:"force,omitempty"`
	// DMAdds reports whether the attacker's Damage Modifier is added to
	// damage: always for melee, and per the Y/N column for ranged weapons
	// (tables.md §5.4).
	DMAdds bool `json:"dmAdds,omitempty"`
	// Load is the number of Turns to load or reload; 0 means not
	// applicable ("—").
	Load int `json:"load,omitempty"`
	// ImpaleSize is the Size used in the Impale Effects Table when the
	// weapon impales; empty when the table prints "—".
	ImpaleSize string `json:"impaleSize,omitempty"`
	// Notes carries the printed notes ("Two handed", "Can Sunder Armor").
	Notes string `json:"notes,omitempty"`
}

// ArmorStats are worn protection: Armor Points per hit location. The
// sample armor table (tables.md §5.1) prints one AP value per item without
// a location breakdown, so armor built from it uses the key "*" covering
// every location; per-location suits list each location explicitly.
// Natural and worn armor stack (Combat › How Combat Works step 7) — see
// Character.ArmorAt.
type ArmorStats struct {
	// Points maps hit-location name → Armor Points; the key "*" grants its
	// value to every location in the character's catalog.
	Points map[string]int `json:"points,omitempty"`
}

// ShieldStats are a shield's statistics (tables.md §5.2: Damage, Size,
// AP/HP, and the number of locations it can cover with Passive Blocking).
type ShieldStats struct {
	// Damage is the shield's bash damage dice expression.
	Damage string `json:"damage,omitempty"`
	// Size is the shield Size (S/M/L/H/E) for parry-size comparison.
	Size string `json:"size,omitempty"`
	// AP are the shield's Armor Points when used to block.
	AP int `json:"ap,omitempty"`
	// HP are the shield's Hit Points (0 HP = broken).
	HP int `json:"hp,omitempty"`
	// BlockLocations is how many hit locations the shield can cover with
	// Passive Blocking (Combat › Close Combat › Passive Blocking).
	BlockLocations int `json:"blockLocations,omitempty"`
}

// WeaponEntry is one row of the sample weapon tables (melee or ranged).
type WeaponEntry struct {
	// Name is the printed weapon name.
	Name string `json:"name"`
	// Tags classify the entry: TagWeapon plus TagMelee or TagRanged.
	Tags []string `json:"tags"`
	// Stats are the printed weapon statistics.
	Stats WeaponStats `json:"stats"`
}

// ArmorEntry is one row of the sample armor table (tables.md §5.1).
type ArmorEntry struct {
	// Name is the printed armor name (any era column).
	Name string `json:"name"`
	// AP is the printed Armor Points.
	AP int `json:"ap"`
}

// ShieldEntry is one row of the sample shield table (tables.md §5.2).
type ShieldEntry struct {
	// Name is the printed shield name.
	Name string `json:"name"`
	// Stats are the printed shield statistics.
	Stats ShieldStats `json:"stats"`
}

// meleeWeapons transcribes the Sample Melee Weapons table (tables.md §5.3,
// Combat › Arms and Armor). "—" AP/HP (Fist/Kick) is stored as 0; the DM
// always applies to melee damage (Combat › How Combat Works step 5).
var meleeWeapons = []WeaponEntry{
	{Name: "Axe", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d6+1", Size: "M", AP: 4, HP: 8, DMAdds: true, Notes: "Can Sunder Armor"}},
	{Name: "Broadsword", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d8", Size: "M", AP: 6, HP: 10, DMAdds: true}},
	{Name: "Chain", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d4", Size: "M", AP: 8, HP: 6, DMAdds: true, Notes: "Can entangle"}},
	{Name: "Club", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d6", Size: "M", AP: 4, HP: 4, DMAdds: true}},
	{Name: "Dagger", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d4+1", Size: "S", AP: 6, HP: 8, DMAdds: true, Notes: "Can be thrown"}},
	{Name: "Fist/Kick", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d3", Size: "S", DMAdds: true, Notes: "Human Unarmed Combat Damage"}},
	{Name: "Longsword", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d8", Size: "M", AP: 6, HP: 12, DMAdds: true, Notes: "Two handed"}},
	{Name: "Mace", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d8", Size: "M", AP: 6, HP: 6, DMAdds: true}},
	{Name: "Net", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d4", Size: "S", AP: 2, HP: 20, DMAdds: true, Notes: "Entrapping, Thrown"}},
	{Name: "Shortsword", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d6", Size: "M", AP: 6, HP: 8, DMAdds: true}},
	{Name: "1H Spear", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d8+1", Size: "M", AP: 4, HP: 5, DMAdds: true}},
	{Name: "2H Spear", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d10+1", Size: "L", AP: 4, HP: 10, DMAdds: true, Notes: "Can be set to receive a charge"}},
	{Name: "War hammer", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d8+1", Size: "M", AP: 3, HP: 8, DMAdds: true, Notes: "Two handed"}},
	{Name: "Whip", Tags: []string{TagWeapon, TagMelee}, Stats: WeaponStats{Damage: "1d3", Size: "M", AP: 2, HP: 8, DMAdds: true, Notes: "Entangling"}},
}

// rangedWeapons transcribes the Sample Ranged Weapons table (tables.md
// §5.4). Range is printed "Close/Effective/Long" in metres; "—" columns are
// stored as zero values.
var rangedWeapons = []WeaponEntry{
	{Name: "Bolas", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "1d4", Range: "10/25/50", AP: 2, HP: 2}},
	{Name: "Bow", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "1d8", Range: "15/100/200", Force: "L", DMAdds: true, Load: 1, ImpaleSize: "S", AP: 4, HP: 4}},
	{Name: "Dagger", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "1d4", Range: "5/10/20", Force: "S", DMAdds: true, ImpaleSize: "S", AP: 4, HP: 8}},
	{Name: "Javelin", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "1d8+1", Range: "10/20/50", Force: "H", DMAdds: true, ImpaleSize: "M", AP: 3, HP: 8}},
	{Name: "Sling", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "1d8", Range: "10/150/300", Force: "L", Load: 2, AP: 1, HP: 2}},
	{Name: "Stone/Rock", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "1d3", Range: "5/10/20", Force: "S", DMAdds: true}},
	{Name: "Pistol", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "1d6", Range: "50/100/200", Force: "L", Load: 2}},
	{Name: "Rifle", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "2d6", Range: "100/300/2000", Force: "H", Load: 2}},
	{Name: "Shotgun", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "3d6", Range: "20/50/200", Force: "M", Load: 3}},
	{Name: "Laser Rifle", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "1d10+2", Range: "40/120/480", Load: 3}},
	{Name: "Plasma Rifle", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "2d6+4", Range: "30/100/300", Force: "E", Load: 3}},
	{Name: "Gauss Rifle", Tags: []string{TagWeapon, TagRanged}, Stats: WeaponStats{Damage: "2d8+2", Range: "150/500/5000", Force: "E", Load: 3}},
}

// sampleArmor transcribes the Sample Armor table (tables.md §5.1): each
// non-empty name from the Ancient/Medieval, Modern and Futuristic columns
// with its row's Armor Points.
var sampleArmor = []ArmorEntry{
	{Name: "Furs/Hides", AP: 1},
	{Name: "Bike Leathers", AP: 1},
	{Name: "Aketon", AP: 2},
	{Name: "Gambeson", AP: 2},
	{Name: "Sports Gear", AP: 2},
	{Name: "Linothorax", AP: 3},
	{Name: "Ballistic Cloth", AP: 3},
	{Name: "Brigandine", AP: 4},
	{Name: "Flak Jacket", AP: 4},
	{Name: "Liquid Body Armor", AP: 4},
	{Name: "Hoplite Plate", AP: 5},
	{Name: "Riot Gear", AP: 5},
	{Name: "Mail", AP: 6},
	{Name: "Type I Bulletproof Vest", AP: 6},
	{Name: "Adaptive Mesh", AP: 6},
	{Name: "Plated Mail", AP: 7},
	{Name: "Type II Bulletproof Vest", AP: 7},
	{Name: "Type III Bulletproof Vest", AP: 8},
	{Name: "Light Assault Armor", AP: 8},
	{Name: "Type IV Bulletproof Vest", AP: 10},
	{Name: "Full Assault Armor", AP: 12},
}

// sampleShields transcribes the Sample Shield table (tables.md §5.2):
// Damage, Size, AP/HP and Passive Blocking locations.
var sampleShields = []ShieldEntry{
	{Name: "Buckler", Stats: ShieldStats{Damage: "1d3", Size: "M", AP: 6, HP: 9, BlockLocations: 2}},
	{Name: "Heater", Stats: ShieldStats{Damage: "1d4", Size: "L", AP: 6, HP: 12, BlockLocations: 3}},
	{Name: "Kite", Stats: ShieldStats{Damage: "1d4", Size: "H", AP: 4, HP: 15, BlockLocations: 4}},
	{Name: "Scutum", Stats: ShieldStats{Damage: "1d4", Size: "H", AP: 4, HP: 18, BlockLocations: 5}},
	{Name: "Viking/Riot Shield", Stats: ShieldStats{Damage: "1d4", Size: "L", AP: 4, HP: 12, BlockLocations: 4}},
}

// MeleeWeapons returns copies of every sample melee weapon row, in table
// order. The returned entries (and their tag slices) are copies — mutating
// them cannot corrupt the table.
func MeleeWeapons() []WeaponEntry {
	return cloneWeaponEntries(meleeWeapons)
}

// RangedWeapons returns copies of every sample ranged weapon row, in table
// order. Note "Dagger" exists in both weapon tables; WeaponByName resolves
// the melee row first — use this function to reach the ranged row.
func RangedWeapons() []WeaponEntry {
	return cloneWeaponEntries(rangedWeapons)
}

// cloneWeaponEntries deep-copies weapon table rows.
func cloneWeaponEntries(in []WeaponEntry) []WeaponEntry {
	out := make([]WeaponEntry, len(in))
	for i, e := range in {
		e.Tags = slices.Clone(e.Tags)
		out[i] = e
	}
	return out
}

// Armors returns copies of every sample armor row, in table order.
func Armors() []ArmorEntry {
	out := make([]ArmorEntry, len(sampleArmor))
	copy(out, sampleArmor)
	return out
}

// Shields returns copies of every sample shield row, in table order.
func Shields() []ShieldEntry {
	out := make([]ShieldEntry, len(sampleShields))
	copy(out, sampleShields)
	return out
}

// WeaponByName returns the sample weapon row with the given name, searching
// the melee table first and then the ranged table. Errors wrap
// [ErrUnknownEquipment].
func WeaponByName(name string) (WeaponEntry, error) {
	for _, e := range meleeWeapons {
		if e.Name == name {
			return cloneWeaponEntries([]WeaponEntry{e})[0], nil
		}
	}
	for _, e := range rangedWeapons {
		if e.Name == name {
			return cloneWeaponEntries([]WeaponEntry{e})[0], nil
		}
	}
	return WeaponEntry{}, fmt.Errorf("%w: weapon %q", ErrUnknownEquipment, name)
}

// ArmorByName returns the sample armor row with the given name. Errors wrap
// [ErrUnknownEquipment].
func ArmorByName(name string) (ArmorEntry, error) {
	for _, e := range sampleArmor {
		if e.Name == name {
			return e, nil
		}
	}
	return ArmorEntry{}, fmt.Errorf("%w: armor %q", ErrUnknownEquipment, name)
}

// ShieldByName returns the sample shield row with the given name. Errors
// wrap [ErrUnknownEquipment].
func ShieldByName(name string) (ShieldEntry, error) {
	for _, e := range sampleShields {
		if e.Name == name {
			return e, nil
		}
	}
	return ShieldEntry{}, fmt.Errorf("%w: shield %q", ErrUnknownEquipment, name)
}

// NewWeapon returns an inventory item for the named sample weapon (melee or
// ranged, melee first). The item has no ID until added with AddItem.
// Errors wrap [ErrUnknownEquipment].
func NewWeapon(name string) (Item, error) {
	e, err := WeaponByName(name)
	if err != nil {
		return Item{}, err
	}
	stats := e.Stats
	return Item{Name: e.Name, Qty: 1, Tags: slices.Clone(e.Tags), Weapon: &stats}, nil
}

// NewArmor returns an inventory item for the named sample armor, covering
// every hit location ("*") at the table's Armor Points. The item has no ID
// until added with AddItem. Errors wrap [ErrUnknownEquipment].
func NewArmor(name string) (Item, error) {
	e, err := ArmorByName(name)
	if err != nil {
		return Item{}, err
	}
	return Item{
		Name: e.Name,
		Qty:  1,
		Tags: []string{TagArmor},
		Armor: &ArmorStats{
			Points: map[string]int{"*": e.AP},
		},
	}, nil
}

// NewShield returns an inventory item for the named sample shield (tagged
// as both a weapon and a shield so it can be wielded). The item has no ID
// until added with AddItem. Errors wrap [ErrUnknownEquipment].
func NewShield(name string) (Item, error) {
	e, err := ShieldByName(name)
	if err != nil {
		return Item{}, err
	}
	stats := e.Stats
	return Item{
		Name:   e.Name,
		Qty:    1,
		Tags:   []string{TagWeapon, TagShield},
		Shield: &stats,
	}, nil
}

// ArmorAt returns the total Armor Points protecting the named hit location:
// the catalog's natural armor plus every equipped armor item covering that
// location (the "*" key covers all locations). Natural and worn armor stack
// (Combat › How Combat Works step 7). Errors wrap [ErrInvalidLocation].
func (c *Character) ArmorAt(location string) (int, error) {
	if c == nil {
		return 0, errNilCharacter()
	}
	d, err := Derive(c)
	if err != nil {
		return 0, err
	}
	ap := 0
	found := false
	for _, loc := range d.Locations {
		if loc.Name == location {
			ap += loc.AP
			found = true
			break
		}
	}
	if !found {
		return 0, fmt.Errorf("%w: %q", ErrInvalidLocation, location)
	}
	for _, it := range c.Inventory {
		if it == nil || !it.Equipped || it.Armor == nil {
			continue
		}
		if v, ok := it.Armor.Points[location]; ok {
			ap += v
			continue
		}
		if v, ok := it.Armor.Points["*"]; ok {
			ap += v
		}
	}
	return ap, nil
}

// HighestArmorAP returns the Armor Points of the single most protective
// worn item (the highest per-location value it grants). The highest AP worn
// penalises the Initiative roll (Combat › Combat Rounds and Initiative);
// natural catalog armor is not counted here.
func (c *Character) HighestArmorAP() int {
	if c == nil {
		return 0
	}
	best := 0
	for _, it := range c.Inventory {
		if it == nil || !it.Equipped || it.Armor == nil {
			continue
		}
		for _, v := range it.Armor.Points {
			if v > best {
				best = v
			}
		}
	}
	return best
}

// errNilCharacter is the shared nil-character error for inventory/equipment
// helpers.
func errNilCharacter() error {
	return errors.New("char: nil character")
}

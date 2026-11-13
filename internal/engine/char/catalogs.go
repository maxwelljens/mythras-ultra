package char

import (
	"fmt"
	"sort"
	"strings"
)

// DefaultCatalog is the hit-location catalog used when a character's
// HitLocationCatalog field is empty: the standard humanoid table.
const DefaultCatalog = "humanoid"

// LocationDef is one row of a hit-location catalog: the name of the hit
// location, the inclusive RollRange on the catalog's roll (all catalogs in
// the rulebook are read on 1d20), and where its starting Hit Points and
// Armor Points come from:
//
//   - HP > 0: a fixed starting HP printed in a creature stat block
//     (tables.md §8.4); AP is that stat block's natural armor value.
//   - HPRow names the Step 3 Hit Points row (tables.md §1.6) — "Head",
//     "Chest", "Abdomen", "Arm" or "Leg" — used when HP is 0.
//
// A humanoid-style location has AP 0: worn armor is counted by
// Character.ArmorAt, not stored on the location.
type LocationDef struct {
	// Name is the hit location's name, e.g. "Right Leg".
	Name string `json:"name"`
	// RollRange is the inclusive 1d20 range that selects this location.
	RollRange [2]int `json:"rollRange"`
	// HP is the fixed starting Hit Points from a creature stat block;
	// 0 means derive from HPRow.
	HP int `json:"hp,omitempty"`
	// AP is the fixed natural Armor Points from a creature stat block.
	AP int `json:"ap,omitempty"`
	// HPRow is the Step 3 HP table row ("Head", "Chest", "Abdomen", "Arm",
	// "Leg"); ignored when HP > 0.
	HPRow string `json:"hpRow,omitempty"`
}

// Catalog returns a copy of the named hit-location catalog. An empty name
// means DefaultCatalog. Lookup is case-insensitive and accepts the printed
// creature names as aliases (e.g. "Bear (Grizzly)", "Goblin/Orc"). Errors
// wrap [ErrUnknownCatalog].
//
// The returned slice is a copy — mutating it cannot corrupt the table.
func Catalog(name string) ([]LocationDef, error) {
	if name == "" {
		name = DefaultCatalog
	}
	key := strings.ToLower(strings.TrimSpace(name))
	if canonical, ok := catalogAliases[key]; ok {
		key = canonical
	}
	defs, ok := hitLocationCatalogs[key]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownCatalog, name)
	}
	out := make([]LocationDef, len(defs))
	copy(out, defs)
	return out, nil
}

// CatalogNames returns the canonical key of every hit-location catalog, in
// sorted order.
func CatalogNames() []string {
	names := make([]string, 0, len(hitLocationCatalogs))
	for name := range hitLocationCatalogs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

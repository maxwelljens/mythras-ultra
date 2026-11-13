package char

import (
	"errors"
	"reflect"
	"testing"
)

// TestCatalog_Humanoid checks the humanoid catalog against the Humanoid
// Hit Locations table exactly (tables.md §3.8, read on 1d20).
func TestCatalog_Humanoid(t *testing.T) {
	t.Parallel()
	got, err := Catalog(DefaultCatalog)
	if err != nil {
		t.Fatalf("Catalog(humanoid): %v", err)
	}
	want := []LocationDef{
		{Name: "Right Leg", RollRange: [2]int{1, 3}, HPRow: "Leg"},
		{Name: "Left Leg", RollRange: [2]int{4, 6}, HPRow: "Leg"},
		{Name: "Abdomen", RollRange: [2]int{7, 9}, HPRow: "Abdomen"},
		{Name: "Chest", RollRange: [2]int{10, 12}, HPRow: "Chest"},
		{Name: "Right Arm", RollRange: [2]int{13, 15}, HPRow: "Arm"},
		{Name: "Left Arm", RollRange: [2]int{16, 18}, HPRow: "Arm"},
		{Name: "Head", RollRange: [2]int{19, 20}, HPRow: "Head"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("humanoid catalog =\n%+v\nwant\n%+v", got, want)
	}
}

// TestCatalog_RangesCoverAllRolls validates every catalog in the package:
// names are present, ranges lie inside 1..20 (all catalogs are read on
// 1d20), and every roll value is covered exactly once — except the shark,
// whose printed table overlaps by design (AMBIGUOUS, tables.md §8.4 /
// digest §9.10: rolls 17 and 18 appear in both "Left Fin" and "Head").
func TestCatalog_RangesCoverAllRolls(t *testing.T) {
	t.Parallel()
	names := CatalogNames()
	if len(names) != 11 {
		t.Fatalf("CatalogNames() = %d catalogs, want 11 (humanoid + 10 creatures)", len(names))
	}
	const maxRoll = 20
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			defs, err := Catalog(name)
			if err != nil {
				t.Fatalf("Catalog(%q): %v", name, err)
			}
			coverage := map[int]int{}
			for _, def := range defs {
				if def.Name == "" {
					t.Errorf("empty location name in %q", name)
				}
				lo, hi := def.RollRange[0], def.RollRange[1]
				if lo < 1 || hi > maxRoll || lo > hi {
					t.Errorf("location %q range [%d,%d] outside 1..%d", def.Name, lo, hi, maxRoll)
					continue
				}
				for roll := lo; roll <= hi; roll++ {
					coverage[roll]++
				}
				// Every location needs a hit point source.
				if def.HP <= 0 && def.HPRow == "" {
					t.Errorf("location %q has neither fixed HP nor an HP row", def.Name)
				}
			}
			for roll := 1; roll <= maxRoll; roll++ {
				n := coverage[roll]
				if n == 0 {
					t.Errorf("roll %d not covered in catalog %q", roll, name)
					continue
				}
				if name == "shark" && (roll == 17 || roll == 18) {
					// Printed overlap: 17-18 Left Fin + 17-20 Head.
					if n != 2 {
						t.Errorf("shark roll %d covered %d times, want the documented overlap of 2", roll, n)
					}
					continue
				}
				if n != 1 {
					t.Errorf("roll %d covered %d times in catalog %q, want exactly 1", roll, n, name)
				}
			}
		})
	}
}

// TestCatalog_Lookup covers the default, aliases and the unknown-catalog
// sentinel.
func TestCatalog_Lookup(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "humanoid", "Humanoid", " HUMANOID ", "human"} {
		defs, err := Catalog(name)
		if err != nil || len(defs) != 7 {
			t.Fatalf("Catalog(%q) = %d defs, %v; want 7 humanoid locations", name, len(defs), err)
		}
	}
	aliases := map[string]string{
		"Bear (Grizzly)":      "bear",
		"Goblin/Orc":          "goblin",
		"Xenomorphic Alien":   "xenomorph",
		"Crocodile/Alligator": "crocodile",
	}
	for alias, canonical := range aliases {
		got, err := Catalog(alias)
		if err != nil {
			t.Fatalf("Catalog(%q): %v", alias, err)
		}
		want, err := Catalog(canonical)
		if err != nil {
			t.Fatalf("Catalog(%q): %v", canonical, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("alias %q did not resolve to %q", alias, canonical)
		}
	}
	if _, err := Catalog("owlbear"); !errors.Is(err, ErrUnknownCatalog) {
		t.Errorf("unknown catalog: err = %v, want ErrUnknownCatalog", err)
	}
}

// TestCatalog_ReturnsCopy mutates a returned catalog and checks the table
// is unaffected.
func TestCatalog_ReturnsCopy(t *testing.T) {
	t.Parallel()
	defs, err := Catalog("goblin")
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	defs[0].Name = "mutated"
	defs[0].HP = 999
	again, err := Catalog("goblin")
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if again[0].Name == "mutated" || again[0].HP == 999 {
		t.Error("Catalog leaked the internal table")
	}
}

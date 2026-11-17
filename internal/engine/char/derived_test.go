package char

import (
	"errors"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
)

// TestDerive_DamageModifier checks the Damage Modifier table
// (tables.md §1.2, Step 3) against hand-computed STR+SIZ cases, including
// the AMBIGUOUS refusal beyond STR+SIZ 120.
func TestDerive_DamageModifier(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		str, siz  int
		wantExpr  string
		wantBand  string
		wantErrIs error
	}{
		{"first band 5 or less", 4, 1, "-1d8", "5 or less", nil},
		{"band 6-10", 5, 5, "-1d6", "6-10", nil},
		{"band 11-15", 8, 7, "-1d4", "11-15", nil},
		{"band 16-20", 10, 10, "-1d2", "16-20", nil},
		{"band 21-25", 12, 10, "0", "21-25", nil},
		{"band 26-30", 13, 14, "1d2", "26-30", nil},
		{"band 31-35", 17, 15, "1d4", "31-35", nil},
		{"band 36-40", 18, 20, "1d6", "36-40", nil},
		{"band 41-45", 18, 25, "1d8", "41-45", nil},
		{"band 46-50", 18, 30, "1d10", "46-50", nil},
		{"band 51-60", 18, 40, "1d12", "51-60", nil},
		{"band 61-70", 18, 45, "2d6", "61-70", nil},
		{"band 71-80", 18, 55, "1d8+1d6", "71-80", nil},
		{"band 81-90", 18, 65, "2d8", "81-90", nil},
		{"band 91-100", 18, 75, "1d10+1d8", "91-100", nil},
		{"band 101-110", 18, 85, "2d10", "101-110", nil},
		{"band 111-120", 18, 100, "2d10+1d2", "111-120", nil},
		{"beyond 120 refused (AMBIGUOUS Continue Progression)", 18, 103, "", "", ErrDamageModifierRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Character{Chars: Characteristics{STR: tt.str, SIZ: tt.siz}}
			d, err := Derive(c)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("Derive(%d+%d) error = %v, want %v", tt.str, tt.siz, err, tt.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("Derive(%d+%d): %v", tt.str, tt.siz, err)
			}
			if d.DamageModifier != tt.wantExpr {
				t.Errorf("DamageModifier = %q, want %q", d.DamageModifier, tt.wantExpr)
			}
			if d.DamageModifierBand != tt.wantBand {
				t.Errorf("DamageModifierBand = %q, want %q", d.DamageModifierBand, tt.wantBand)
			}
		})
	}
}

// TestDamageModifierExpressionsParse verifies every stored Damage Modifier
// expression rolls with the dice package (the combat layer will roll them
// directly).
func TestDamageModifierExpressionsParse(t *testing.T) {
	t.Parallel()
	r := dice.NewRand(1)
	for _, row := range damageModifierTable {
		if _, err := r.Roll(row.expr); err != nil {
			t.Errorf("expression %q (band %s): %v", row.expr, row.band, err)
		}
	}
}

// TestDerive_HitPoints checks the Hit Points per Location table
// (tables.md §1.6, Step 3) against hand-computed CON+SIZ cases, including
// the "+5 beyond 40" column.
func TestDerive_HitPoints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		con, siz  int
		want      map[string]int // starting HP per location
		wantTotal int
	}{
		{
			name: "CON+SIZ 5 (first column)",
			con:  4, siz: 1,
			want:      map[string]int{"Head": 1, "Chest": 3, "Abdomen": 2, "Right Arm": 1, "Left Arm": 1, "Right Leg": 1, "Left Leg": 1},
			wantTotal: 10,
		},
		{
			name: "CON+SIZ 22 (21-25 column)",
			con:  12, siz: 10,
			want:      map[string]int{"Head": 5, "Chest": 7, "Abdomen": 6, "Right Arm": 4, "Left Arm": 4, "Right Leg": 5, "Left Leg": 5},
			wantTotal: 36,
		},
		{
			name: "CON+SIZ 40 (36-40 column)",
			con:  20, siz: 20,
			want:      map[string]int{"Head": 8, "Chest": 10, "Abdomen": 9, "Right Arm": 7, "Left Arm": 7, "Right Leg": 8, "Left Leg": 8},
			wantTotal: 57,
		},
		{
			name: "CON+SIZ 45 (+5 column: one step beyond 40)",
			con:  25, siz: 20,
			want:      map[string]int{"Head": 9, "Chest": 11, "Abdomen": 10, "Right Arm": 8, "Left Arm": 8, "Right Leg": 9, "Left Leg": 9},
			wantTotal: 64,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Character{Chars: Characteristics{CON: tt.con, SIZ: tt.siz}}
			d, err := Derive(c)
			if err != nil {
				t.Fatalf("Derive: %v", err)
			}
			if d.HitPoints != tt.wantTotal {
				t.Errorf("total HP = %d, want %d", d.HitPoints, tt.wantTotal)
			}
			for loc, want := range tt.want {
				if got := d.LocationHP[loc]; got != want {
					t.Errorf("LocationHP[%s] = %d, want %d", loc, got, want)
				}
			}
			if len(d.LocationHP) != len(tt.want) {
				t.Errorf("LocationHP has %d entries, want %d", len(d.LocationHP), len(tt.want))
			}
		})
	}
}

// TestDerive_Attributes checks the Step 3 step tables (tables.md §1.3–§1.5,
// §1.7): Experience Modifier, Healing Rate, Luck Points (the identical
// 6/12/18/+6 tables), Initiative rounding (AMBIGUOUS: ceil per the global
// round-up rule), Magic/Power Points = POW, AP = 2, movement 6.
func TestDerive_Attributes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		chars      Characteristics
		wantExpMod int // tables.md §1.3 (CHA)
		wantHeal   int // tables.md §1.4 (CON)
		wantLuck   int // tables.md §1.5 (POW)
		wantInit   int // ceil((DEX+INT)/2)
	}{
		{"CHA/CON/POW 6 or less, DEX+INT even", Characteristics{CON: 6, DEX: 11, INT: 13, POW: 6, CHA: 6}, -1, 1, 1, 12},
		{"CHA/CON/POW 7-12", Characteristics{CON: 7, DEX: 10, INT: 13, POW: 12, CHA: 7}, 0, 2, 2, 12},
		{"CHA/CON/POW 13-18, DEX+INT odd (round up)", Characteristics{CON: 13, DEX: 9, INT: 10, POW: 13, CHA: 13}, 1, 3, 3, 10},
		{"upper 13-18 boundary", Characteristics{CON: 18, DEX: 18, INT: 18, POW: 18, CHA: 18}, 1, 3, 3, 18},
		{"first +6 step (19-24)", Characteristics{CON: 19, DEX: 1, INT: 1, POW: 19, CHA: 19}, 2, 4, 4, 1},
		{"second +6 step (25-30)", Characteristics{CON: 25, DEX: 1, INT: 1, POW: 25, CHA: 25}, 3, 5, 5, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d, err := Derive(&Character{Chars: tt.chars})
			if err != nil {
				t.Fatalf("Derive: %v", err)
			}
			if d.ExperienceModifier != tt.wantExpMod {
				t.Errorf("ExperienceModifier = %d, want %d", d.ExperienceModifier, tt.wantExpMod)
			}
			if d.HealingRate != tt.wantHeal {
				t.Errorf("HealingRate = %d, want %d", d.HealingRate, tt.wantHeal)
			}
			if d.LuckPoints != tt.wantLuck {
				t.Errorf("LuckPoints = %d, want %d", d.LuckPoints, tt.wantLuck)
			}
			if d.Initiative != tt.wantInit {
				t.Errorf("Initiative = %d, want %d", d.Initiative, tt.wantInit)
			}
			if d.ActionPoints != 2 {
				t.Errorf("ActionPoints = %d, want 2", d.ActionPoints)
			}
			if d.MagicPoints != tt.chars.POW || d.PowerPoints != tt.chars.POW {
				t.Errorf("Magic/Power Points = %d/%d, want POW %d", d.MagicPoints, d.PowerPoints, tt.chars.POW)
			}
			if d.MovementRate != 6 {
				t.Errorf("MovementRate = %d, want 6 (humans)", d.MovementRate)
			}
		})
	}
}

// TestDerive_CreatureCatalog checks that a creature catalog's fixed
// stat-block HP/AP flow into the snapshot (tables.md §8.4, dragon rows).
func TestDerive_CreatureCatalog(t *testing.T) {
	t.Parallel()
	c := &Character{
		Chars:              Characteristics{STR: 30, CON: 30, SIZ: 50, DEX: 20, INT: 25, POW: 25, CHA: 20},
		HitLocationCatalog: "dragon",
	}
	d, err := Derive(c)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	want := map[string]int{"Tail": 17, "Right Wing": 16, "Forequarters": 19, "Head": 17}
	for loc, hp := range want {
		if got := d.LocationHP[loc]; got != hp {
			t.Errorf("LocationHP[%s] = %d, want %d", loc, got, hp)
		}
	}
	if got := d.HitPoints; got != 17+17+17+18+16+16+19+17+17+17 {
		t.Errorf("total HP = %d, want %d", got, 17+17+17+18+16+16+19+17+17+17)
	}
	if len(d.Locations) != 10 || d.Locations[0].AP != 8 {
		t.Errorf("dragon catalog not embedded: %+v", d.Locations)
	}
}

// TestDerive_Errors covers the sentinel errors Derive can produce.
func TestDerive_Errors(t *testing.T) {
	t.Parallel()
	if _, err := Derive(nil); err == nil {
		t.Error("Derive(nil) should fail")
	}
	_, err := Derive(&Character{Chars: Characteristics{STR: -1}})
	if !errors.Is(err, ErrInvalidCharacteristic) {
		t.Errorf("negative characteristic: err = %v, want ErrInvalidCharacteristic", err)
	}
	_, err = Derive(&Character{HitLocationCatalog: "owlbear"})
	if !errors.Is(err, ErrUnknownCatalog) {
		t.Errorf("unknown catalog: err = %v, want ErrUnknownCatalog", err)
	}
}

// TestDerived_HitLocations returns a copy — mutating it cannot corrupt the
// cached snapshot.
func TestDerived_HitLocations(t *testing.T) {
	t.Parallel()
	d, err := Derive(&Character{})
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	locs := d.HitLocations()
	locs[0].Name = "mutated"
	locs2 := d.HitLocations()
	if locs2[0].Name == "mutated" {
		t.Error("HitLocations leaked the cached slice")
	}
}

// TestRefresh checks the Refresh invariants: standard skills created,
// derived cached, current HP initialised without clobbering damage, and 0
// CON marking the character dead (Step 2: 0 CON = death).
func TestRefresh(t *testing.T) {
	t.Parallel()
	c := &Character{Chars: Characteristics{STR: 12, CON: 12, SIZ: 10, DEX: 11, INT: 13, POW: 14, CHA: 9}}
	if err := c.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if c.Derived.HitPoints == 0 || c.Dead {
		t.Fatalf("derived/dead not initialised: %+v dead=%v", c.Derived, c.Dead)
	}
	for _, name := range StandardSkills() {
		sk, ok := c.Skill(name)
		if !ok {
			t.Fatalf("standard skill %q missing after Refresh", name)
		}
		if sk.Base <= 0 || sk.Value != sk.Base {
			t.Errorf("skill %q base/value = %d/%d, want equal positive", name, sk.Base, sk.Value)
		}
	}
	// Damage must persist across refreshes.
	c.CurrentHP["Chest"] -= 3
	if err := c.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := c.CurrentHP["Chest"]; got != c.Derived.LocationHP["Chest"]-3 {
		t.Errorf("damage lost across Refresh: chest = %d", got)
	}
	// 0 CON = death.
	c.Chars.CON = 0
	if err := c.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !c.Dead || c.IsAlive() {
		t.Error("0 CON should mark the character dead")
	}
}

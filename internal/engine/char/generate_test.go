package char

import (
	"errors"
	"reflect"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
)

// TestGenerate_Determinism checks that identical seed and method yield
// identical characteristics (the replayability contract), and that a
// different seed does not.
func TestGenerate_Determinism(t *testing.T) {
	t.Parallel()
	for _, m := range []GenMethod{GenRoll, GenRollAssign, GenRollHigh, GenPoints, GenPulpRoll, GenPulpPoints, GenParagonRoll, GenParagonPoints} {
		t.Run(m.String(), func(t *testing.T) {
			t.Parallel()
			a, err := Generate(dice.NewRand(42), m)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			b, err := Generate(dice.NewRand(42), m)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if !reflect.DeepEqual(a, b) {
				t.Errorf("same seed diverged: %+v vs %+v", a, b)
			}
			c, err := Generate(dice.NewRand(43), m)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if reflect.DeepEqual(a, c) {
				t.Errorf("different seeds produced identical rolls for %s", m)
			}
		})
	}
}

// TestGenerate_MethodParity documents the methods that share rolls:
// Dice Roll Assign rolls exactly like Dice Roll (the swap is the caller's
// arrangement), and Pulp Hero's printed rolls equal Dice Roll, High
// (tables.md §1.1 vs §1.15).
func TestGenerate_MethodParity(t *testing.T) {
	t.Parallel()
	pairs := [][2]GenMethod{
		{GenRoll, GenRollAssign},
		{GenRollHigh, GenPulpRoll},
	}
	for _, pair := range pairs {
		a, err := Generate(dice.NewRand(7), pair[0])
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		b, err := Generate(dice.NewRand(7), pair[1])
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s and %s should share rolls: %+v vs %+v", pair[0], pair[1], a, b)
		}
	}
}

// TestGenerate_Bounds checks every method against the printed caps and
// minima (tables.md §1.1: 3–18 for the 3d6 group, 8–18 for SIZ/INT, pool
// sums of 80/90/100 for the point builds).
func TestGenerate_Bounds(t *testing.T) {
	t.Parallel()
	rolled := []GenMethod{GenRoll, GenRollAssign, GenRollHigh, GenPulpRoll, GenParagonRoll}
	builds := []struct {
		method GenMethod
		pool   int
	}{
		{GenPoints, 80},
		{GenPulpPoints, 90},
		{GenParagonPoints, 100},
	}
	checkBounds := func(t *testing.T, ch Characteristics) {
		t.Helper()
		for _, sc := range []struct {
			name string
			v    int
			min  int
		}{
			{"STR", ch.STR, 3}, {"CON", ch.CON, 3}, {"DEX", ch.DEX, 3},
			{"POW", ch.POW, 3}, {"CHA", ch.CHA, 3}, {"SIZ", ch.SIZ, 8}, {"INT", ch.INT, 8},
		} {
			if sc.v < sc.min || sc.v > 18 {
				t.Errorf("%s = %d, outside [%d,18]", sc.name, sc.v, sc.min)
			}
		}
	}
	for _, m := range rolled {
		for seed := int64(1); seed <= 25; seed++ {
			ch, err := Generate(dice.NewRand(seed), m)
			if err != nil {
				t.Fatalf("%s seed %d: %v", m, seed, err)
			}
			checkBounds(t, ch)
		}
	}
	for _, b := range builds {
		for seed := int64(1); seed <= 25; seed++ {
			ch, err := Generate(dice.NewRand(seed), b.method)
			if err != nil {
				t.Fatalf("%s seed %d: %v", b.method, seed, err)
			}
			checkBounds(t, ch)
			total := ch.STR + ch.CON + ch.SIZ + ch.DEX + ch.INT + ch.POW + ch.CHA
			if total != b.pool {
				t.Errorf("%s seed %d: total %d, want pool %d", b.method, seed, total, b.pool)
			}
		}
	}
}

// TestGenerate_Errors covers the unknown-method and nil-Rand guards.
func TestGenerate_Errors(t *testing.T) {
	t.Parallel()
	if _, err := Generate(dice.NewRand(1), GenMethod(99)); !errors.Is(err, ErrUnknownGenMethod) {
		t.Errorf("unknown method: err = %v, want ErrUnknownGenMethod", err)
	}
	if _, err := Generate(nil, GenRoll); err == nil {
		t.Error("nil Rand should fail")
	}
}

// TestSkillPicks checks the Skill Pyramid allocations (tables.md §1.14):
// 1×50, 2×40, 3×30, 4×20, 5×10 — 15 allocations summing to 350, with a
// fresh slice per call.
func TestSkillPicks(t *testing.T) {
	t.Parallel()
	want := []int{50, 40, 40, 30, 30, 30, 20, 20, 20, 20, 10, 10, 10, 10, 10}
	got := SkillPicks()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SkillPicks() = %v, want %v", got, want)
	}
	sum := 0
	for _, v := range got {
		sum += v
	}
	if sum != 350 || len(got) != 15 {
		t.Errorf("pyramid totals %d over %d allocations, want 350 over 15", sum, len(got))
	}
	got[0] = 99
	if again := SkillPicks(); again[0] != 50 {
		t.Error("SkillPicks leaked the previous call's slice")
	}
}

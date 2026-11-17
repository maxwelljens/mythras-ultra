package rules

import (
	"errors"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
)

// luckCharacter returns a refreshed character with the given POW so
// Derived.LuckPoints follows tables.md §1.5.
func luckCharacter(t *testing.T, pow int) *char.Character {
	t.Helper()
	c := &char.Character{
		Name: "Lucky",
		Chars: char.Characteristics{
			STR: 10, CON: 12, SIZ: 11, DEX: 13, INT: 14, POW: pow, CHA: 12,
		},
	}
	if err := c.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return c
}

// TestLuckPoints checks the Luck Points table from POW (tables.md §1.5,
// digest L211-212: ≤6 → 1, 7-12 → 2, 13-18 → 3, each 6 points thereafter
// → +1) and the nil/unrefreshed edge cases.
func TestLuckPoints(t *testing.T) {
	cases := []struct{ pow, want int }{
		{6, 1}, {7, 2}, {12, 2}, {13, 3}, {18, 3}, {19, 4}, {24, 4}, {25, 5}, {30, 5},
	}
	for _, tc := range cases {
		c := luckCharacter(t, tc.pow)
		if got := LuckPoints(c); got != tc.want {
			t.Errorf("LuckPoints(POW %d) = %d, want %d", tc.pow, got, tc.want)
		}
	}
	if got := LuckPoints(nil); got != 0 {
		t.Errorf("LuckPoints(nil) = %d, want 0", got)
	}
	if got := LuckPoints(&char.Character{}); got != 0 {
		t.Errorf("LuckPoints(unrefreshed) = %d, want 0", got)
	}
}

// TestSpendLuckUses checks the three printed uses of Using Luck Points
// (L1265-1279): case-insensitive purpose matching, the flat cost of one
// point per action (L1267), and the narration facts each use carries.
func TestSpendLuckUses(t *testing.T) {
	c := luckCharacter(t, 15) // max 3
	purposes := []struct{ in, wantName string }{
		{"Cheat Fate", "Cheat Fate"},
		{"cheat fate", "Cheat Fate"},
		{"DESPERATE EFFORT", "Desperate Effort"},
		{"mitigate damage", "Mitigate Damage"},
	}
	for _, tc := range purposes {
		t.Run(tc.in, func(t *testing.T) {
			use, err := SpendLuck(dice.NewRand(1), c, tc.in)
			if err != nil {
				t.Fatalf("SpendLuck(%q): %v", tc.in, err)
			}
			if use.Purpose != tc.wantName {
				t.Errorf("Purpose = %q, want %q", use.Purpose, tc.wantName)
			}
			if use.Cost != 1 {
				t.Errorf("Cost = %d, want 1 (one Luck Point per action, L1267)", use.Cost)
			}
			if use.Max != 3 {
				t.Errorf("Max = %d, want 3", use.Max)
			}
			if use.Effect == "" || use.RuleRef == "" {
				t.Errorf("narration facts incomplete: %+v", use)
			}
		})
	}
}

// TestSpendLuckCheatFateReroll checks the deterministic re-roll option of
// Cheat Fate (L1271) and that non-reroll uses roll nothing.
func TestSpendLuckCheatFateReroll(t *testing.T) {
	c := luckCharacter(t, 15)
	first, err := SpendLuck(dice.NewRand(42), c, "Cheat Fate")
	if err != nil {
		t.Fatalf("SpendLuck: %v", err)
	}
	second, err := SpendLuck(dice.NewRand(42), c, "Cheat Fate")
	if err != nil {
		t.Fatalf("SpendLuck: %v", err)
	}
	if first.Reroll != second.Reroll {
		t.Errorf("reroll not deterministic for one seed: %d vs %d", first.Reroll, second.Reroll)
	}
	if first.Reroll < 1 || first.Reroll > 100 {
		t.Errorf("reroll = %d, want a d100 result", first.Reroll)
	}
	use, err := SpendLuck(dice.NewRand(42), c, "Desperate Effort")
	if err != nil {
		t.Fatalf("SpendLuck: %v", err)
	}
	if use.Reroll != 0 {
		t.Errorf("Desperate Effort rolled dice (%d) — only Cheat Fate involves dice", use.Reroll)
	}
}

// TestSpendLuckRefusals checks that undefined purposes (including the
// Experience/Training rolls Luck may never buy, L1267) and invalid inputs
// are refused before any dice are rolled.
func TestSpendLuckRefusals(t *testing.T) {
	c := luckCharacter(t, 15)

	for _, purpose := range []string{"experience roll", "training", "heal 1 hp", ""} {
		r := dice.NewRand(5)
		_, err := SpendLuck(r, c, purpose)
		if !errors.Is(err, ErrUnknownLuckUse) {
			t.Errorf("SpendLuck(%q) error = %v, want %v", purpose, err, ErrUnknownLuckUse)
		}
		if got, want := r.D100(), dice.NewRand(5).D100(); got != want {
			t.Errorf("SpendLuck(%q) refused but consumed dice: %d, want %d", purpose, got, want)
		}
	}
	if _, err := SpendLuck(dice.NewRand(1), luckCharacter(t, 15), "cheat fate"); err != nil {
		t.Errorf("valid use errored: %v", err)
	}
	if _, err := SpendLuck(dice.NewRand(1), &char.Character{}, "cheat fate"); !errors.Is(err, ErrNoLuck) {
		t.Errorf("unrefreshed character error = %v, want %v", err, ErrNoLuck)
	}
	if _, err := SpendLuck(nil, c, "cheat fate"); err == nil {
		t.Error("SpendLuck(nil Rand) should error")
	}
	if _, err := SpendLuck(dice.NewRand(1), nil, "cheat fate"); err == nil {
		t.Error("SpendLuck(nil character) should error")
	}
}

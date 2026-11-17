package rules

import (
	"errors"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
)

// TestCompetenceBands checks every band boundary of the Competency Level
// Table (L914-927, digest L533-543): 30/31, 50/51, 70/71, 90/91, 110/111,
// plus the table's edges.
func TestCompetenceBands(t *testing.T) {
	cases := []struct {
		skill      int
		band       string
		name       string
		military   string
		random     string
		descrStart string
	}{
		{1, "1-30%", "Novice", "Cannon Fodder", "1d20+10", "Unqualified"},
		{30, "1-30%", "Novice", "Cannon Fodder", "1d20+10", "Unqualified"},
		{31, "31-50%", "Competent", "Green", "1d20+30", "Can perform easy tasks"},
		{50, "31-50%", "Competent", "Green", "1d20+30", "Can perform easy tasks"},
		{51, "51-70%", "Proficient", "Seasoned", "1d20+50", "Has reached an employable level"},
		{70, "51-70%", "Proficient", "Seasoned", "1d20+50", "Has reached an employable level"},
		{71, "71-90%", "Expert", "Veteran", "1d20+70", "Recognised as a professional"},
		{90, "71-90%", "Expert", "Veteran", "1d20+70", "Recognised as a professional"},
		{91, "91-110%", "Master", "Elite", "1d20+90", "Considered one of the best"},
		{110, "91-110%", "Master", "Elite", "1d20+90", "Considered one of the best"},
		{111, ">110%", "Exemplar", "Heroic", "1d20+110", "Renowned far and wide"},
		{500, ">110%", "Exemplar", "Heroic", "1d20+110", "Renowned far and wide"},
		{0, "1-30%", "Novice", "Cannon Fodder", "1d20+10", "Unqualified"}, // table starts at 1
		{-7, "1-30%", "Novice", "Cannon Fodder", "1d20+10", "Unqualified"},
	}
	for _, tc := range cases {
		got := Competence(tc.skill)
		if got.Band != tc.band || got.Name != tc.name || got.Military != tc.military || got.Random != tc.random {
			t.Errorf("Competence(%d) = {band %q name %q military %q random %q}, want {band %q name %q military %q random %q}",
				tc.skill, got.Band, got.Name, got.Military, got.Random,
				tc.band, tc.name, tc.military, tc.random)
		}
		if len(got.Description) < len(tc.descrStart) || got.Description[:len(tc.descrStart)] != tc.descrStart {
			t.Errorf("Competence(%d).Description = %q, want prefix %q", tc.skill, got.Description, tc.descrStart)
		}
	}
}

// TestRandomSkillRanges checks the printed dice codes land inside their
// printed bands (L922-927): 1d20+10 → 11-30 within 1-30%, …, 1d20+110 →
// 111-130 above 110%.
func TestRandomSkillRanges(t *testing.T) {
	skills := []struct {
		skill int
		min   int
		max   int
	}{
		{1, 11, 30},     // Novice: 1d20+10
		{31, 31, 50},    // Competent: 1d20+30
		{51, 51, 70},    // Proficient: 1d20+50
		{71, 71, 90},    // Expert: 1d20+70
		{91, 91, 110},   // Master: 1d20+90
		{111, 111, 130}, // Exemplar: 1d20+110
	}
	r := dice.NewRand(21)
	for i := 0; i < 300; i++ {
		for _, tc := range skills {
			level := Competence(tc.skill)
			got, err := RandomSkill(r, level)
			if err != nil {
				t.Fatalf("RandomSkill(%s): %v", level.Name, err)
			}
			if got < tc.min || got > tc.max {
				t.Fatalf("RandomSkill(%s) = %d, want %d-%d", level.Name, got, tc.min, tc.max)
			}
		}
	}
}

// TestRandomSkillRefusals checks invalid inputs: a level without a dice
// code, a nil Rand, and a malformed dice expression (wrapped from the dice
// package).
func TestRandomSkillRefusals(t *testing.T) {
	if _, err := RandomSkill(dice.NewRand(1), CompetenceLevel{}); !errors.Is(err, ErrNoRandomValue) {
		t.Errorf("zero level error = %v, want %v", err, ErrNoRandomValue)
	}
	if _, err := RandomSkill(nil, Competence(30)); err == nil {
		t.Error("RandomSkill(nil Rand) should error")
	}
	if _, err := RandomSkill(dice.NewRand(1), CompetenceLevel{Band: "bogus", Random: "not dice"}); !errors.Is(err, dice.ErrInvalidExpr) {
		t.Errorf("malformed code error = %v, want wrapped %v", err, dice.ErrInvalidExpr)
	}
}

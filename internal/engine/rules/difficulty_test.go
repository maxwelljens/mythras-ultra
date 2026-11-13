package rules

import "testing"

func TestDifficultyModifyMultiplicative(t *testing.T) {
	cases := []struct {
		grade   DifficultyGrade
		skill   int
		want    int
		wantErr error
	}{
		{DifficultyVeryEasy, 50, 100, nil},
		{DifficultyEasy, 50, 75, nil}, // add half again
		{DifficultyEasy, 51, 77, nil}, // 51 + 26 — fractions round up (Intro L10-12)
		{DifficultyStandard, 50, 50, nil},
		{DifficultyHard, 60, 40, nil},       // reduce by one-third
		{DifficultyHard, 50, 33, nil},       // 50 − 17 — fractions round up
		{DifficultyFormidable, 50, 25, nil}, // reduce by half
		{DifficultyFormidable, 45, 23, nil}, // 22.5 rounds up
		{DifficultyHerculean, 50, 10, nil},  // reduce to one-fifth
		{DifficultyHerculean, 47, 10, nil},  // 9.4 rounds up
		{DifficultyAutomatic, 50, 0, ErrAutomatic},
		{DifficultyHopeless, 50, 0, ErrHopeless},
	}
	for _, tc := range cases {
		got, err := tc.grade.Modify(tc.skill)
		if tc.wantErr != nil {
			if err == nil {
				t.Errorf("Modify(%v,%d) = %d, want error %v", tc.grade, tc.skill, got, tc.wantErr)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("Modify(%v,%d) = (%d,%v), want %d", tc.grade, tc.skill, got, err, tc.want)
		}
	}
	if _, err := (DifficultyStandard).Modify(-1); err == nil {
		t.Error("Modify should reject negative skill")
	}
}

func TestDifficultyModifySimplified(t *testing.T) {
	cases := []struct {
		grade DifficultyGrade
		skill int
		want  int
	}{
		{DifficultyVeryEasy, 50, 90},
		{DifficultyEasy, 50, 70},
		{DifficultyStandard, 50, 50},
		{DifficultyHard, 50, 30},
		{DifficultyFormidable, 50, 10},
		{DifficultyHerculean, 50, 0}, // clamped at 0 (50-80)
		{DifficultyHerculean, 100, 20},
	}
	for _, tc := range cases {
		got, err := tc.grade.ModifySimplified(tc.skill)
		if err != nil || got != tc.want {
			t.Errorf("ModifySimplified(%v,%d) = (%d,%v), want %d", tc.grade, tc.skill, got, err, tc.want)
		}
	}
}

func TestHarder(t *testing.T) {
	if got := Harder(DifficultyEasy, DifficultyHard); got != DifficultyHard {
		t.Errorf("Harder = %v, want hard", got)
	}
	if got := Harder(DifficultyHerculean, DifficultyStandard); got != DifficultyHerculean {
		t.Errorf("Harder = %v, want herculean", got)
	}
}

func TestDifficultyRoundTrip(t *testing.T) {
	for g := DifficultyAutomatic; g <= DifficultyHopeless; g++ {
		parsed, err := ParseDifficulty(g.String())
		if err != nil {
			t.Fatalf("ParseDifficulty(%q): %v", g, err)
		}
		if parsed != g {
			t.Errorf("round trip: got %v, want %v", parsed, g)
		}
	}
	if _, err := ParseDifficulty("Impossible"); err == nil {
		t.Error("ParseDifficulty should reject undefined grades (the AMBIGUOUS 'Impossible' grade is not defined by the rulebook)")
	}
}

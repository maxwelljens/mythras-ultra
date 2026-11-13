package rules

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name     string
		roll     int
		skill    int
		want     Grade
		wantErr  bool
		rulebook string
	}{
		{"exact skill boundary succeeds", 50, 50, GradeSuccess, false, "roll equal to or less than skill"},
		{"one over skill fails", 51, 50, GradeFailure, false, "roll greater than skill"},
		{"01 always success even over skill", 1, 0, GradeSuccess, false, "01-05 irrespective of skill"},
		{"05 always success even over skill", 5, 3, GradeSuccess, false, "01-05 irrespective of skill"},
		{"06 over tiny skill fails", 6, 3, GradeFailure, false, "no always-success"},
		{"critical at exactly one tenth", 5, 50, GradeCritical, false, "roll <= skill/10"},
		{"critical rounds fractions up", 7, 66, GradeCritical, false, "33-66% => 07 or less example"},
		{"just above critical is plain success", 8, 66, GradeSuccess, false, "outside critical range"},
		{"96 fails even under skill 95", 96, 95, GradeFailure, false, "96-00 always failure"},
		{"96 fails despite skill 120", 96, 120, GradeFailure, false, "96-00 irrespective of skill"},
		{"98 fails despite skill 120", 98, 120, GradeFailure, false, "96-00 irrespective of skill"},
		{"99 fumbles at skill 50", 99, 50, GradeFumble, false, "99-00 fumble"},
		{"00 fumbles at skill 50", 100, 50, GradeFumble, false, "99-00 fumble"},
		{"99 is not fumble over 100% skill", 99, 120, GradeFailure, false, "over 100% fumble only on 00"},
		{"00 still fumbles over 100% skill", 100, 120, GradeFumble, false, "over 100% fumble only on 00"},
		{"critical range scales above 100", 12, 120, GradeCritical, false, "one tenth of modified skill"},
		{"13 with skill 120 succeeds", 13, 120, GradeSuccess, false, "skill above 100 broadens success"},
		{"skill 0 can still crit on nothing", 1, 0, GradeSuccess, false, "critical range 0; always-success band"},
		{"skill 0 fails mid roll", 50, 0, GradeFailure, false, "no skill"},
		{"roll 0 invalid", 0, 50, GradeFumble, true, "d100 rolls are 1-100"},
		{"roll 101 invalid", 101, 50, GradeFumble, true, "d100 rolls are 1-100"},
		{"negative skill invalid", 50, -1, GradeFumble, true, "skill cannot be negative"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Classify(tc.roll, tc.skill)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Classify(%d,%d) = %v, want error", tc.roll, tc.skill, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Classify(%d,%d) error: %v", tc.roll, tc.skill, err)
			}
			if got != tc.want {
				t.Errorf("Classify(%d,%d) = %v, want %v (%s)", tc.roll, tc.skill, got, tc.want, tc.rulebook)
			}
		})
	}
}

func TestCriticalRange(t *testing.T) {
	cases := []struct{ skill, want int }{{0, 0}, {1, 1}, {9, 1}, {10, 1}, {11, 2}, {33, 4}, {66, 7}, {100, 10}, {120, 12}}
	for _, tc := range cases {
		if got := CriticalRange(tc.skill); got != tc.want {
			t.Errorf("CriticalRange(%d) = %d, want %d", tc.skill, got, tc.want)
		}
	}
}

func TestGradeRoundTrip(t *testing.T) {
	for g := GradeFumble; g <= GradeCritical; g++ {
		parsed, err := ParseGrade(g.String())
		if err != nil {
			t.Fatalf("ParseGrade(%q): %v", g, err)
		}
		if parsed != g {
			t.Errorf("round trip: got %v, want %v", parsed, g)
		}
	}
	if _, err := ParseGrade("exceptional"); err == nil {
		t.Error("ParseGrade should reject non-existent grades (no Exceptional tier)")
	}
}

func TestOpposed(t *testing.T) {
	cases := []struct {
		name       string
		a, b       Outcome
		wantWinner int
		wantMargin int
		wantTie    string
		wantBoth   bool
	}{
		{"success beats failure", Outcome{40, 50, GradeSuccess}, Outcome{60, 50, GradeFailure}, 1, 1, "grade", false},
		{"critical beats success", Outcome{4, 50, GradeCritical}, Outcome{30, 50, GradeSuccess}, 1, 1, "grade", false},
		{"critical vs fumble is margin 3", Outcome{4, 50, GradeCritical}, Outcome{99, 50, GradeFumble}, 1, 3, "grade", false},
		{"equal successes: highest roll in range wins", Outcome{40, 50, GradeSuccess}, Outcome{30, 50, GradeSuccess}, 1, 0, "roll", false},
		{"identical rolls: true tie", Outcome{30, 50, GradeSuccess}, Outcome{30, 50, GradeSuccess}, 0, 0, "roll", false},
		{"both fail: no winner", Outcome{80, 50, GradeFailure}, Outcome{70, 50, GradeFailure}, 0, 0, "", true},
		{"failure vs fumble: both failed, no winner", Outcome{70, 50, GradeFailure}, Outcome{99, 50, GradeFumble}, 0, 0, "", true},
		{"b wins over a by two levels", Outcome{99, 50, GradeFumble}, Outcome{30, 50, GradeSuccess}, 2, 2, "grade", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Opposed(tc.a, tc.b)
			if got.Winner != tc.wantWinner || got.Margin != tc.wantMargin || got.TieBreak != tc.wantTie || got.BothFailed != tc.wantBoth {
				t.Errorf("Opposed(%v,%v) = winner %d margin %d tie %q both %v; want winner %d margin %d tie %q both %v",
					tc.a, tc.b, got.Winner, got.Margin, got.TieBreak, got.BothFailed,
					tc.wantWinner, tc.wantMargin, tc.wantTie, tc.wantBoth)
			}
		})
	}
}

func TestDifferentialMatrix(t *testing.T) {
	// Spot checks of the L899-906 matrix (protagonist = a).
	cases := []struct {
		name       string
		a, b       Outcome
		wantWinner int
		wantMargin int
	}{
		{"P crit vs A crit: no benefit", Outcome{1, 50, GradeCritical}, Outcome{2, 50, GradeCritical}, 0, 0},
		{"P crit vs A success: 1", Outcome{1, 50, GradeCritical}, Outcome{30, 50, GradeSuccess}, 1, 1},
		{"P crit vs A failure: 2", Outcome{1, 50, GradeCritical}, Outcome{60, 50, GradeFailure}, 1, 2},
		{"P crit vs A fumble: 3", Outcome{1, 50, GradeCritical}, Outcome{99, 50, GradeFumble}, 1, 3},
		{"P success vs A crit: A 1", Outcome{30, 50, GradeSuccess}, Outcome{1, 50, GradeCritical}, 2, 1},
		{"P success vs A success: no benefit", Outcome{30, 50, GradeSuccess}, Outcome{31, 50, GradeSuccess}, 0, 0},
		{"P success vs A failure: 1", Outcome{30, 50, GradeSuccess}, Outcome{60, 50, GradeFailure}, 1, 1},
		{"P success vs A fumble: 2", Outcome{30, 50, GradeSuccess}, Outcome{99, 50, GradeFumble}, 1, 2},
		{"P failure vs A crit: A 2", Outcome{60, 50, GradeFailure}, Outcome{1, 50, GradeCritical}, 2, 2},
		{"P failure vs A success: A 1", Outcome{60, 50, GradeFailure}, Outcome{30, 50, GradeSuccess}, 2, 1},
		{"P failure vs A failure: no benefit", Outcome{60, 50, GradeFailure}, Outcome{61, 50, GradeFailure}, 0, 0},
		{"P failure vs A fumble: no benefit", Outcome{60, 50, GradeFailure}, Outcome{99, 50, GradeFumble}, 0, 0},
		{"P fumble vs A crit: A 3", Outcome{99, 50, GradeFumble}, Outcome{1, 50, GradeCritical}, 2, 3},
		{"P fumble vs A success: A 2", Outcome{99, 50, GradeFumble}, Outcome{30, 50, GradeSuccess}, 2, 2},
		{"P fumble vs A failure: no benefit", Outcome{99, 50, GradeFumble}, Outcome{60, 50, GradeFailure}, 0, 0},
		{"P fumble vs A fumble: no benefit", Outcome{99, 50, GradeFumble}, Outcome{100, 50, GradeFumble}, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Differential(tc.a, tc.b)
			if got.Winner != tc.wantWinner || got.Margin != tc.wantMargin {
				t.Errorf("Differential(%v,%v) = winner %d margin %d; want winner %d margin %d",
					tc.a, tc.b, got.Winner, got.Margin, tc.wantWinner, tc.wantMargin)
			}
		})
	}
}

func TestEqualiseSkillsOver100(t *testing.T) {
	cases := []struct{ a, b, wantA, wantB int }{
		{50, 60, 50, 60},   // both under 100: unchanged
		{100, 90, 100, 90}, // exactly 100: unchanged
		{120, 60, 100, 40}, // subtract (120-100) from both
		{120, 15, 100, 0},  // clamped at 0
	}
	for _, tc := range cases {
		gotA, gotB := EqualiseSkillsOver100(tc.a, tc.b)
		if gotA != tc.wantA || gotB != tc.wantB {
			t.Errorf("EqualiseSkillsOver100(%d,%d) = (%d,%d), want (%d,%d)", tc.a, tc.b, gotA, gotB, tc.wantA, tc.wantB)
		}
	}
}

func TestOutcomeString(t *testing.T) {
	o := Outcome{Roll: 12, Skill: 66, Grade: GradeCritical}
	if got := o.String(); got != "critical (12 vs 66)" {
		t.Errorf("Outcome.String() = %q", got)
	}
}

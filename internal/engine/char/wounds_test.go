package char

import (
	"errors"
	"strings"
	"testing"
)

// newTestCharacter returns a humanoid character with CON+SIZ = 22, so the
// starting HP are the hand-checked §1.6 values: Head 5, Chest 7, Abdomen 6,
// arms 4, legs 5 (Healing Rate 2 from CON 12).
func newTestCharacter(t *testing.T) *Character {
	t.Helper()
	c := &Character{
		ID:    "pc-1",
		Name:  "Test",
		Chars: Characteristics{STR: 12, CON: 12, SIZ: 10, DEX: 11, INT: 13, POW: 14, CHA: 9},
	}
	if err := c.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return c
}

// TestApplyWound_Thresholds checks the wound-level boundaries of the Damage
// and Wound Levels table (tables.md §3.9) at exact thresholds, plus the
// unconditional condition bookkeeping of Major Wounds.
func TestApplyWound_Thresholds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		location    string
		dmg         int
		wantHP      int
		wantSev     Severity
		wantEffects []string // substrings that must appear; none expected → nil
		wantConds   []string
		noConds     []string
	}{
		{
			name:     "minor: chest 7-6=1 stays positive",
			location: "Chest", dmg: 6, wantHP: 1, wantSev: SeverityMinor,
		},
		{
			name:     "serious at exactly 0 HP",
			location: "Chest", dmg: 7, wantHP: 0, wantSev: SeveritySerious,
			wantEffects: []string{"1d3 Rounds"},
			noConds:     []string{CondUnconscious, CondProne},
		},
		{
			name:     "serious one above the major boundary (-6 of 7)",
			location: "Chest", dmg: 13, wantHP: -6, wantSev: SeveritySerious,
			wantEffects: []string{"unconscious for minutes equal to the damage"},
		},
		{
			name:     "major at exactly -(starting HP)",
			location: "Chest", dmg: 14, wantHP: -7, wantSev: SeverityMajor,
			wantEffects: []string{"immediately Incapacitated", "instant death", "4 Combat Rounds"},
			wantConds:   []string{CondProne, CondIncapacitated, CondUnconscious},
		},
		{
			name:     "major on the head drops unconscious",
			location: "Head", dmg: 10, wantHP: -5, wantSev: SeverityMajor,
			wantEffects: []string{"instant death", "4 Combat Rounds"},
			wantConds:   []string{CondUnconscious, CondProne, CondIncapacitated},
		},
		{
			name:     "serious limb is useless",
			location: "Right Arm", dmg: 4, wantHP: 0, wantSev: SeveritySerious,
			wantEffects: []string{"limb useless", "opposed Endurance"},
		},
		{
			name:     "major limb severs with a 5x Healing Rate timer",
			location: "Left Leg", dmg: 10, wantHP: -5, wantSev: SeverityMajor,
			wantEffects: []string{"severed", "10 minutes (= 5 × Healing Rate)"},
			wantConds:   []string{CondProne, CondIncapacitated},
			noConds:     []string{CondUnconscious},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestCharacter(t)
			sev, effects, err := c.ApplyWound(tt.location, tt.dmg)
			if err != nil {
				t.Fatalf("ApplyWound: %v", err)
			}
			if sev != tt.wantSev {
				t.Errorf("severity = %v, want %v", sev, tt.wantSev)
			}
			if got := c.CurrentHP[tt.location]; got != tt.wantHP {
				t.Errorf("CurrentHP[%s] = %d, want %d", tt.location, got, tt.wantHP)
			}
			joined := strings.Join(effects, " | ")
			for _, want := range tt.wantEffects {
				if !strings.Contains(joined, want) {
					t.Errorf("effects %q missing %q", joined, want)
				}
			}
			for _, cond := range tt.wantConds {
				if !c.HasCondition(cond) {
					t.Errorf("expected condition %q, have %v", cond, c.Conditions)
				}
			}
			for _, cond := range tt.noConds {
				if c.HasCondition(cond) {
					t.Errorf("condition %q should not be present: %v", cond, c.Conditions)
				}
			}
			if tt.wantSev != SeverityNone && len(c.Wounds) != 1 {
				t.Errorf("wound log = %d entries, want 1", len(c.Wounds))
			}
		})
	}
}

// TestApplyWound_ZeroAndInvalid covers: 0 damage inflicts no wound
// (tables.md §3.9 applies damage only when > 0), negative damage and
// unknown locations are refused with sentinels.
func TestApplyWound_ZeroAndInvalid(t *testing.T) {
	t.Parallel()
	t.Run("zero damage", func(t *testing.T) {
		c := newTestCharacter(t)
		sev, effects, err := c.ApplyWound("Chest", 0)
		if err != nil || sev != SeverityNone || effects != nil {
			t.Fatalf("ApplyWound(0) = %v, %v, %v", sev, effects, err)
		}
		if c.CurrentHP["Chest"] != c.Derived.LocationHP["Chest"] || len(c.Wounds) != 0 {
			t.Error("zero damage changed state")
		}
	})
	t.Run("negative damage", func(t *testing.T) {
		c := newTestCharacter(t)
		if _, _, err := c.ApplyWound("Chest", -3); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("err = %v, want ErrInvalidAmount", err)
		}
	})
	t.Run("unknown location", func(t *testing.T) {
		c := newTestCharacter(t)
		if _, _, err := c.ApplyWound("Tail", 5); !errors.Is(err, ErrInvalidLocation) {
			t.Errorf("err = %v, want ErrInvalidLocation", err)
		}
		// Validation runs even when no damage would be applied.
		if _, _, err := c.ApplyWound("Tail", 0); !errors.Is(err, ErrInvalidLocation) {
			t.Errorf("dmg 0 err = %v, want ErrInvalidLocation", err)
		}
	})
}

// TestApplyWound_CreatureLocations checks the AMBIGUOUS creature-class
// mapping: recognised torso names (bear Hindquarters) take the torso
// effects, unrecognised names (crocodile Tail) take the limb reading.
func TestApplyWound_CreatureLocations(t *testing.T) {
	t.Parallel()
	t.Run("torso-class creature location", func(t *testing.T) {
		c := &Character{
			Chars:              Characteristics{STR: 20, CON: 20, SIZ: 34, DEX: 15, INT: 12, POW: 12, CHA: 10},
			HitLocationCatalog: "bear",
		}
		if err := c.Refresh(); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		// Bear Hindquarters: 11 HP → −11 is exactly the major boundary.
		sev, effects, err := c.ApplyWound("Hindquarters", 22)
		if err != nil {
			t.Fatalf("ApplyWound: %v", err)
		}
		if sev != SeverityMajor {
			t.Fatalf("severity = %v, want Major", sev)
		}
		if !c.HasCondition(CondUnconscious) {
			t.Error("torso-class Major Wound should drop unconscious")
		}
		if !strings.Contains(strings.Join(effects, " | "), "instant death") {
			t.Errorf("torso effects = %v", effects)
		}
	})
	t.Run("limb-class creature location", func(t *testing.T) {
		c := &Character{
			Chars:              Characteristics{STR: 20, CON: 20, SIZ: 37, DEX: 15, INT: 12, POW: 12, CHA: 10},
			HitLocationCatalog: "crocodile",
		}
		if err := c.Refresh(); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		// Crocodile Tail: 12 HP → 24 damage lands well past the boundary.
		sev, _, err := c.ApplyWound("Tail", 24)
		if err != nil {
			t.Fatalf("ApplyWound: %v", err)
		}
		if sev != SeverityMajor {
			t.Fatalf("severity = %v, want Major", sev)
		}
		if c.HasCondition(CondUnconscious) {
			t.Error("limb-class Major Wound should not auto-unconscious (Endurance is pending)")
		}
	})
}

// TestSeverityAtAndHeal checks severity recomputation and healing
// bookkeeping: clamping at starting HP and the sentinel errors.
func TestSeverityAtAndHeal(t *testing.T) {
	t.Parallel()
	t.Run("severity follows live HP", func(t *testing.T) {
		c := newTestCharacter(t)
		if sev, err := c.SeverityAt("Chest"); err != nil || sev != SeverityMinor {
			t.Fatalf("fresh chest: %v, %v", sev, err)
		}
		if _, err := c.SeverityAt("Tail"); !errors.Is(err, ErrInvalidLocation) {
			t.Errorf("unknown location: err = %v, want ErrInvalidLocation", err)
		}
		if _, _, err := c.ApplyWound("Chest", 7); err != nil {
			t.Fatalf("ApplyWound: %v", err)
		}
		if sev, err := c.SeverityAt("Chest"); err != nil || sev != SeveritySerious {
			t.Fatalf("wounded chest: %v, %v", sev, err)
		}
	})

	t.Run("heal clamps at starting HP", func(t *testing.T) {
		c := newTestCharacter(t)
		if _, _, err := c.ApplyWound("Left Leg", 8); err != nil { // 5 → −3
			t.Fatalf("ApplyWound: %v", err)
		}
		if err := c.Heal("Left Leg", 2); err != nil { // −3 → −1
			t.Fatalf("Heal: %v", err)
		}
		if got := c.CurrentHP["Left Leg"]; got != -1 {
			t.Errorf("HP after heal = %d, want −1", got)
		}
		if err := c.Heal("Left Leg", 1000); err != nil {
			t.Fatalf("Heal: %v", err)
		}
		if got := c.CurrentHP["Left Leg"]; got != c.Derived.LocationHP["Left Leg"] {
			t.Errorf("HP after big heal = %d, want starting %d", got, c.Derived.LocationHP["Left Leg"])
		}
	})

	t.Run("heal errors", func(t *testing.T) {
		c := newTestCharacter(t)
		if err := c.Heal("Tail", 1); !errors.Is(err, ErrInvalidLocation) {
			t.Errorf("err = %v, want ErrInvalidLocation", err)
		}
		if err := c.Heal("Chest", -1); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("err = %v, want ErrInvalidAmount", err)
		}
	})
}

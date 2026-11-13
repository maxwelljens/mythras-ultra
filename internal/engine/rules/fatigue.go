package rules

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/dice"
)

// FatigueCondition names the condition that carries a character's
// accumulated Fatigue levels on char.Character.Conditions. Fatigue is not
// round-timed, so Rounds stays 0, and the level count lives in Notes as
// "levels: N".
//
// shortcut: char.Character has no Fatigue field and char may not be
// modified here, so Condition.Notes is the persistence slot; combat must
// never decrement Rounds for this condition (only its level count moves,
// through Asphyxiate and RecoverFatigue). Upgrade to a dedicated char field
// if fatigue accrual grows more sources (Blood Loss, disease, exertion).
const FatigueCondition = "Fatigue"

// fatigueNotesPrefix is the exact form of FatigueCondition's Notes written
// and parsed by this package.
const fatigueNotesPrefix = "levels: "

// ErrInvalidFatigue reports a FatigueCondition whose Notes do not carry the
// "levels: N" count this package writes (hand-edited state).
var ErrInvalidFatigue = errors.New("rules: malformed fatigue condition")

// FatigueResult is the audit of one Asphyxiation check (Mythras-Imperative.md,
// Game System › Asphyxiation L935-948; digest L547-554).
type FatigueResult struct {
	// Endurance is the skill the breath-hold was measured in seconds by
	// and the Endurance roll was made against.
	Endurance int `json:"endurance"`
	// Prepared and Strenuous echo the circumstances passed in.
	Prepared  bool `json:"prepared"`
	Strenuous bool `json:"strenuous"`
	// HoldSeconds is how long the breath could be held: the Endurance
	// value, halved when unprepared and passive, one-fifth when
	// unprepared and strenuous (fractions round up, L10-12).
	HoldSeconds int `json:"holdSeconds"`
	// Roll is the d100 result of the Endurance roll made once the held
	// breath ran out; Grade is its success level.
	Roll  int   `json:"roll"`
	Grade Grade `json:"grade"`
	// Gained is the number of Fatigue levels this check added: 0 on a
	// Critical, 1 on a Success, 1d2 on a Failure, 1d3 on a Fumble (the
	// failure/fumble dice are already rolled into it).
	Gained int `json:"gained"`
	// Levels is the character's new Fatigue total after Gained was added.
	Levels int `json:"levels"`
	// RuleRef cites the rulebook section this check implements.
	RuleRef string `json:"ruleRef"`
}

// fatigueLevels reads the accumulated Fatigue levels from the character's
// FatigueCondition. present is false when the condition is absent; a
// condition whose Notes are not "levels: N" wraps [ErrInvalidFatigue].
func fatigueLevels(c *char.Character) (levels int, present bool, err error) {
	for _, cond := range c.Conditions {
		if cond.Name != FatigueCondition {
			continue
		}
		rest, ok := strings.CutPrefix(cond.Notes, fatigueNotesPrefix)
		if !ok {
			return 0, true, fmt.Errorf("%w: notes %q", ErrInvalidFatigue, cond.Notes)
		}
		n, convErr := strconv.Atoi(rest)
		if convErr != nil || n < 0 {
			return 0, true, fmt.Errorf("%w: notes %q", ErrInvalidFatigue, cond.Notes)
		}
		return n, true, nil
	}
	return 0, false, nil
}

// setFatigue stores levels on the character, removing the condition when
// nothing is left to track.
func setFatigue(c *char.Character, levels int) {
	if levels <= 0 {
		c.RemoveCondition(FatigueCondition)
		return
	}
	c.AddCondition(char.Condition{
		Name:  FatigueCondition,
		Notes: fatigueNotesPrefix + strconv.Itoa(levels),
	})
}

// Asphyxiate resolves one Asphyxiation check (Mythras-Imperative.md,
// Game System › Asphyxiation L935-948; digest L547-554): it measures how
// long the character can hold their breath, rolls Endurance once that period
// is over, converts the success level into Fatigue levels (Critical: none;
// Success: 1; Failure: 1d2; Fumble: 1d3, all that Round), and adds them to
// the character's FatigueCondition. The Endurance roll uses Standard
// difficulty in the multiplicative system (both systems agree at Standard).
//
// The breath-hold reductions apply only when the character was not prepared
// (L937); fractions round up (L10-12). Asphyxiation recovery (one level per
// minute, L946) is RecoverFatigue. The GM's optional enduring level for
// smoke or water inhalation (L948) is discretion, not mechanics, and is not
// modelled here.
func Asphyxiate(r *dice.Rand, c *char.Character, prepared, strenuous bool) (FatigueResult, error) {
	if r == nil {
		return FatigueResult{}, errors.New("rules: nil dice source")
	}
	if c == nil {
		return FatigueResult{}, errors.New("rules: nil character")
	}
	endurance, err := SkillValue(c, "Endurance")
	if err != nil {
		return FatigueResult{}, fmt.Errorf("rules: asphyxiation: %w", err)
	}

	hold := endurance
	if !prepared {
		if strenuous {
			hold = (endurance + 4) / 5 // one-fifth, rounding up (L10-12)
		} else {
			hold = (endurance + 1) / 2 // halved, rounding up (L10-12)
		}
	}

	out, _, err := SkillTest(r, endurance, DifficultyStandard, false)
	if err != nil {
		return FatigueResult{}, fmt.Errorf("rules: asphyxiation: %w", err)
	}

	gained := 0
	switch out.Grade {
	case GradeCritical:
		// no further deterioration (L941)
	case GradeSuccess:
		gained = 1 // L942
	case GradeFailure:
		gained = r.D(2) // 1d2 extra levels that Round (L943)
	case GradeFumble:
		gained = r.D(3) // 1d3 extra levels that Round (L944)
	}

	prev, _, err := fatigueLevels(c)
	if err != nil {
		return FatigueResult{}, err
	}
	total := prev + gained
	setFatigue(c, total)

	return FatigueResult{
		Endurance:   endurance,
		Prepared:    prepared,
		Strenuous:   strenuous,
		HoldSeconds: hold,
		Roll:        out.Roll,
		Grade:       out.Grade,
		Gained:      gained,
		Levels:      total,
		RuleRef:     "Mythras Imperative, Game System: Asphyxiation",
	}, nil
}

// RecoverFatigue removes up to minutes Fatigue levels from the character at
// the Asphyxiation recovery rate of one level per minute (L946; digest
// L553-554) and returns how many levels were recovered — never more than the
// character had. Physical-exertion Fatigue recovers far more slowly
// (Recovery Period ÷ Healing Rate, L1084) and is not modelled here; a
// missing condition or a non-positive minutes argument recovers nothing, and
// a malformed level count (see [ErrInvalidFatigue]) recovers nothing while
// leaving the condition untouched for the caller to repair — Asphyxiate is
// the loud path for malformed state.
func RecoverFatigue(c *char.Character, minutes int) int {
	if c == nil || minutes <= 0 {
		return 0
	}
	levels, present, err := fatigueLevels(c)
	if err != nil || !present {
		return 0
	}
	recovered := minutes
	if recovered > levels {
		recovered = levels
	}
	setFatigue(c, levels-recovered)
	return recovered
}

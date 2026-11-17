package rules

import (
	"errors"
	"fmt"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
)

// ErrUnknownSkill reports a name with no testable skill value: it is not on
// the character's skill sheet, not one of their Passions, and carries no base
// formula in the skill catalogue, so no d100 test can be resolved against it.
var ErrUnknownSkill = errors.New("rules: unknown skill")

// TestAudit is the full record of one SkillTest: the dice as rolled, the
// skill before and after the difficulty grade, the grade applied, the
// resulting success level, and the rulebook section it implements. The
// session log stores it verbatim.
type TestAudit struct {
	// Roll is the raw d100 result (1-100, 00 recorded as 100).
	Roll int `json:"roll"`
	// BaseSkill is the skill value before any difficulty grade was applied.
	BaseSkill int `json:"baseSkill"`
	// Grade is the difficulty grade the caller passed (never stacked — see
	// SkillTest).
	Grade DifficultyGrade `json:"grade"`
	// ModifiedSkill is the value the roll was actually classified against.
	ModifiedSkill int `json:"modifiedSkill"`
	// Outcome is the success level the roll produced.
	Outcome Grade `json:"outcome"`
	// RuleRef cites the rulebook section this test implements.
	RuleRef string `json:"ruleRef"`
}

// SkillValue returns the character's effective rating for name, following
// the lookup order the rules imply (Mythras-Imperative.md, Skills › How
// Skills Work L538-540; Step 8 › Passions L484-502; digest L415, L383-388):
//
//  1. the trained value from the character's skill sheet (c.Skills), which
//     includes custom skills the GM added to the sheet;
//  2. a Passion of that name — Passions are rated and rolled like skills;
//  3. the untrained base computed from the characteristics via
//     char.BaseSkill (two characteristics, or one doubled).
//
// Anything else wraps [ErrUnknownSkill]. A passion name only reaches step 3
// when no skill of that name exists on the sheet (the sheet wins).
func SkillValue(c *char.Character, name string) (int, error) {
	if c == nil {
		return 0, errors.New("rules: nil character")
	}
	if sk, ok := c.Skill(name); ok {
		return sk.Value, nil
	}
	if p, ok := c.Passions[name]; ok && p != nil {
		return p.Value, nil
	}
	if base, known := char.BaseSkill(name, c.Chars); known {
		return base, nil
	}
	return 0, fmt.Errorf("%w: %q", ErrUnknownSkill, name)
}

// SkillTest resolves one skill test: it applies grade to skill with the
// multiplicative Difficulty Grade Table (DifficultyGrade.Modify) or the
// simplified flat percentages (DifficultyGrade.ModifySimplified when
// simplified is true), rolls 1d100, and classifies the roll against the
// modified value (Mythras-Imperative.md, Skills › How Skills Work /
// Modifying Skills L538-588; digest L415-454).
//
// Difficulty grades never stack: the caller passes a single grade — combine
// several simultaneous penalties with Harder upstream (L586).
// [ErrAutomatic] and [ErrHopeless] propagate before any dice are rolled, so
// callers can branch (skip the roll / refuse the attempt) without consuming
// the replayable RNG sequence; a negative skill propagates
// [ErrInvalidSkill].
func SkillTest(r *dice.Rand, skill int, grade DifficultyGrade, simplified bool) (Outcome, TestAudit, error) {
	if r == nil {
		return Outcome{}, TestAudit{}, errors.New("rules: nil dice source")
	}
	var (
		modified int
		err      error
	)
	if simplified {
		modified, err = grade.ModifySimplified(skill)
	} else {
		modified, err = grade.Modify(skill)
	}
	if err != nil {
		return Outcome{}, TestAudit{}, err
	}
	roll := r.D100()
	g, err := Classify(roll, modified)
	if err != nil {
		return Outcome{}, TestAudit{}, fmt.Errorf("rules: skill test: %w", err)
	}
	audit := TestAudit{
		Roll:          roll,
		BaseSkill:     skill,
		Grade:         grade,
		ModifiedSkill: modified,
		Outcome:       g,
		RuleRef:       "Mythras Imperative, Skills: How Skills Work",
	}
	return Outcome{Roll: roll, Skill: modified, Grade: g}, audit, nil
}

// SkillTestCharacter looks up name with SkillValue and resolves the test
// with SkillTest, so one call covers trained skills, Passions and untrained
// characteristic bases alike. [ErrUnknownSkill] and the difficulty sentinels
// propagate for the caller to translate.
func SkillTestCharacter(r *dice.Rand, c *char.Character, name string, grade DifficultyGrade, simplified bool) (Outcome, TestAudit, error) {
	skill, err := SkillValue(c, name)
	if err != nil {
		return Outcome{}, TestAudit{}, fmt.Errorf("rules: skill test %q: %w", name, err)
	}
	return SkillTest(r, skill, grade, simplified)
}

// Package rules implements Mythras Imperative resolution mechanics: success
// grades, difficulty grades, skill tests, opposed and differential rolls,
// experience and character improvement.
//
// Determinism contract: every roll goes through an explicit *dice.Rand passed
// by the caller; results carry the full audit trail (roll, modified skill,
// grade) so the session log can record exactly what happened.
//
// Source of truth: Mythras-Imperative.md ("Skills — How Skills Work,
// Criticals and Fumbles"), mirrored in docs/rules/digest.md §Skills.
package rules

import (
	"errors"
	"fmt"
)

// Grade is one of exactly four success levels in Mythras Imperative:
// Critical Success, Success, Failure, Fumble. There is no "Exceptional
// Success" tier (docs/rules/digest.md "Criticals and Fumbles", L425-433).
type Grade int

// The four success levels, ordered from worst to best so that comparison
// operators express "better/worse level of success" directly.
const (
	GradeFumble   Grade = iota // roll of 99-00 (see Classify for the >100% rule)
	GradeFailure               // roll above the skill value
	GradeSuccess               // roll equal to or under the skill value
	GradeCritical              // roll within one-tenth of the skill value
)

// String returns the canonical lower-case name of the grade.
func (g Grade) String() string {
	switch g {
	case GradeCritical:
		return "critical"
	case GradeSuccess:
		return "success"
	case GradeFailure:
		return "failure"
	case GradeFumble:
		return "fumble"
	default:
		return "invalid"
	}
}

// Valid reports whether g is one of the four defined grades.
func (g Grade) Valid() bool { return g >= GradeFumble && g <= GradeCritical }

// ParseGrade is the inverse of String, for harness/UI round-trips.
func ParseGrade(s string) (Grade, error) {
	switch s {
	case "critical":
		return GradeCritical, nil
	case "success":
		return GradeSuccess, nil
	case "failure":
		return GradeFailure, nil
	case "fumble":
		return GradeFumble, nil
	default:
		return GradeFumble, fmt.Errorf("rules: %q is not a success grade", s)
	}
}

// Automatic-result bands (docs/rules/digest.md "How Skills Work", L418-423):
// any roll of 01-05 succeeds irrespective of skill, any roll of 96-00 fails
// irrespective of skill. (00 counts as 100.)
const (
	alwaysSuccessMax = 5  // rolls 01-05 always succeed
	alwaysFailureMin = 96 // rolls 96-00 always fail (unless Fumble applies)
)

// ErrInvalidRoll and ErrInvalidSkill mark out-of-range inputs to Classify.
var (
	ErrInvalidRoll  = errors.New("rules: dice roll out of range 1-100")
	ErrInvalidSkill = errors.New("rules: skill value cannot be negative")
)

// CriticalRange returns the highest d100 roll that is a Critical Success for
// the given (modified) skill value: one-tenth of the skill, fractions rounded
// up (docs/rules/digest.md L426-428: skill 33-66% => critical on 07 or less).
func CriticalRange(skill int) int {
	if skill <= 0 {
		return 0
	}
	return (skill + 9) / 10
}

// Classify maps a d100 roll against a (modified) skill value onto one of the
// four success levels, in this order (docs/rules/digest.md L418-433):
//
//  1. Fumble: rolls 99-00. Skills over 100% fumble only on 00 (roll 99 then
//     falls through to the always-failure band).
//  2. Critical Success: roll within one-tenth of the skill (fractions up).
//  3. Success: roll 01-05 (always), or roll equal to or under the skill and
//     outside the always-failure band (rolls 96-00 always fail).
//  4. Failure: everything else.
//
// roll must be 1-100 (00 = 100) and skill >= 0; anything else returns
// ErrInvalidRoll or ErrInvalidSkill.
func Classify(roll, skill int) (Grade, error) {
	if roll < 1 || roll > 100 {
		return GradeFumble, fmt.Errorf("%w: %d", ErrInvalidRoll, roll)
	}
	if skill < 0 {
		return GradeFumble, fmt.Errorf("%w: %d", ErrInvalidSkill, skill)
	}

	// Fumble band: 99-00, but only 00 when the skill exceeds 100%.
	if skill > 100 {
		if roll == 100 {
			return GradeFumble, nil
		}
	} else if roll >= 99 {
		return GradeFumble, nil
	}

	if roll <= CriticalRange(skill) {
		return GradeCritical, nil
	}
	if roll <= alwaysSuccessMax {
		return GradeSuccess, nil
	}
	if roll <= skill && roll < alwaysFailureMin {
		return GradeSuccess, nil
	}
	return GradeFailure, nil
}

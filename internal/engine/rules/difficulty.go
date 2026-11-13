package rules

import (
	"errors"
	"fmt"
	"strings"
)

// DifficultyGrade is a task difficulty from the Difficulty Grade Table
// (Mythras-Imperative.md, Skills › Modifying Skills, L567-588). The rulebook
// offers two alternative modifier systems — multiplicative or simplified
// flat percentages — to be used consistently; both are implemented here
// (Modify and ModifySimplified).
//
// Difficulty grades never stack: when several penalties apply, the hardest
// grade takes precedence (L453-454).
type DifficultyGrade int

// The eight difficulty grades, ordered from easiest to hardest so that
// comparison expresses "harder" directly.
const (
	DifficultyAutomatic  DifficultyGrade = iota // no roll needed
	DifficultyVeryEasy                          // double skill / +40%
	DifficultyEasy                              // add half again / +20%
	DifficultyStandard                          // no adjustment
	DifficultyHard                              // reduce by one-third / -20%
	DifficultyFormidable                        // reduce by half / -40%
	DifficultyHerculean                         // reduce to one-fifth / -80%
	DifficultyHopeless                          // no attempt can be made
)

// ErrAutomatic and ErrHopeless are returned by Modify when the grade makes a
// roll meaningless: the caller either skips the roll or refuses the attempt.
var (
	ErrAutomatic = errors.New("rules: difficulty Automatic — no roll needed")
	ErrHopeless  = errors.New("rules: difficulty Hopeless — no attempt can be made")
)

// String returns the canonical grade name.
func (d DifficultyGrade) String() string {
	switch d {
	case DifficultyAutomatic:
		return "automatic"
	case DifficultyVeryEasy:
		return "very easy"
	case DifficultyEasy:
		return "easy"
	case DifficultyStandard:
		return "standard"
	case DifficultyHard:
		return "hard"
	case DifficultyFormidable:
		return "formidable"
	case DifficultyHerculean:
		return "herculean"
	case DifficultyHopeless:
		return "hopeless"
	default:
		return "invalid"
	}
}

// Valid reports whether d is one of the eight defined grades.
func (d DifficultyGrade) Valid() bool {
	return d >= DifficultyAutomatic && d <= DifficultyHopeless
}

// ParseDifficulty maps names as printed in the rulebook (case-insensitive
// single words, e.g. "herculean", "very easy") onto DifficultyGrade values.
func ParseDifficulty(s string) (DifficultyGrade, error) {
	for g := DifficultyAutomatic; g <= DifficultyHopeless; g++ {
		if strings.EqualFold(g.String(), s) {
			return g, nil
		}
	}
	return DifficultyStandard, fmt.Errorf("rules: %q is not a difficulty grade", s)
}

// Harder returns the harder of two grades (hardest takes precedence — grades
// are never stacked, L453-454). Automatic is "harder" than nothing only in
// the sense that no roll remains; callers treat it via Modify.
func Harder(a, b DifficultyGrade) DifficultyGrade {
	if a > b {
		return a
	}
	return b
}

// Modify applies the multiplicative system to a skill value: Very Easy
// doubles, Easy adds half again, Hard removes one-third, Formidable halves,
// Herculean reduces to one-fifth (L442-451). Automatic and Hopeless return
// ErrAutomatic/ErrHopeless instead of a value. Per the rulebook's Introduction
// (Mythras-Imperative.md L10-12) any fractional division rounds UP, so e.g.
// Easy on 51% adds 26% (=77%), Formidable on 45% halves to 23%, Herculean on
// 47% reduces to 10%. Results clamp at 0 minimum.
func (d DifficultyGrade) Modify(skill int) (int, error) {
	if skill < 0 {
		return 0, fmt.Errorf("%w: %d", ErrInvalidSkill, skill)
	}
	switch d {
	case DifficultyAutomatic:
		return 0, ErrAutomatic
	case DifficultyHopeless:
		return 0, ErrHopeless
	}
	var out int
	switch d {
	case DifficultyVeryEasy:
		out = skill * 2
	case DifficultyEasy:
		out = skill + (skill+1)/2 // "add half again"; fractions round up
	case DifficultyStandard:
		out = skill
	case DifficultyHard:
		out = skill - (skill+2)/3 // "reduce by one-third"; fractions round up
	case DifficultyFormidable:
		out = (skill + 1) / 2 // "reduce by half"; fractions round up
	case DifficultyHerculean:
		out = (skill + 4) / 5 // "reduce to one-fifth"; fractions round up
	default:
		return 0, fmt.Errorf("rules: invalid difficulty grade %d", int(d))
	}
	if out < 0 {
		out = 0
	}
	return out, nil
}

// ModifySimplified applies the simplified flat-percentage system: Very Easy
// +40%, Easy +20%, Hard -20%, Formidable -40%, Herculean -80% (L442-451).
// Automatic and Hopeless behave as in Modify. Results clamp at 0 minimum.
func (d DifficultyGrade) ModifySimplified(skill int) (int, error) {
	if skill < 0 {
		return 0, fmt.Errorf("%w: %d", ErrInvalidSkill, skill)
	}
	switch d {
	case DifficultyAutomatic:
		return 0, ErrAutomatic
	case DifficultyHopeless:
		return 0, ErrHopeless
	}
	var out int
	switch d {
	case DifficultyVeryEasy:
		out = skill + 40
	case DifficultyEasy:
		out = skill + 20
	case DifficultyStandard:
		out = skill
	case DifficultyHard:
		out = skill - 20
	case DifficultyFormidable:
		out = skill - 40
	case DifficultyHerculean:
		out = skill - 80
	default:
		return 0, fmt.Errorf("rules: invalid difficulty grade %d", int(d))
	}
	if out < 0 {
		out = 0
	}
	return out, nil
}

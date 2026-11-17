package resolve

import (
	"fmt"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/engine/rules"
)

// RollDice rolls a dice expression on the session RNG (tool: roll_dice).
// The audit carries the full Roll — every die, the modifier and the total —
// and a citation of the rulebook's dice section.
type RollDice struct {
	// Expr is the dice expression to roll (see the dice package grammar).
	Expr string `json:"expr"`
	// Reason is why the dice are being rolled; recorded for the log.
	Reason string `json:"reason,omitempty"`
}

// Kind implements [Move].
func (m RollDice) Kind() string { return "roll_dice" }

// Apply rolls the expression. Parse failures wrap dice.ErrInvalidExpr or
// dice.ErrTooManyDice and consume no randomness.
func (m RollDice) Apply(s *State) (Result, error) {
	roll, err := s.Rand().Roll(m.Expr)
	if err != nil {
		return Result{}, err
	}
	facts := []string{"roll: " + roll.String()}
	if m.Reason != "" {
		facts = append(facts, "reason: "+m.Reason)
	}
	a := Audit{}
	a.addRoll(roll)
	a.addCitation("Mythras Imperative, Introduction: Dice Used")
	return newResult(roll.String(), facts, a), nil
}

// SkillTest resolves one skill test for a character against a difficulty
// grade (tool: skill_test): the effective rating is looked up (trained
// skill, Passion or untrained characteristic base), the hardest grade wins,
// and 1d100 is classified into a success grade
// (Mythras Imperative, Skills: How Skills Work).
type SkillTest struct {
	// Actor is the character tested (ID).
	Actor string `json:"actor"`
	// Skill is the skill, Passion or characteristic-base skill name.
	Skill string `json:"skill"`
	// Grade is the difficulty grade name ("very easy" … "hopeless"); empty
	// means standard difficulty.
	Grade string `json:"grade,omitempty"`
	// Modifiers lists additional difficulty names (darkness, distance, …).
	// Grades never stack: the hardest of Grade and Modifiers applies
	// (Mythras Imperative, Skills: Modifying Skills).
	Modifiers []string `json:"modifiers,omitempty"`
}

// Kind implements [Move].
func (m SkillTest) Kind() string { return "skill_test" }

// Apply validates actor, skill and difficulty before rolling, then runs
// rules.SkillTestCharacter. Difficulty sentinels (rules.ErrAutomatic,
// rules.ErrHopeless) and rules.ErrUnknownSkill propagate, so a refused
// attempt consumes no dice.
func (m SkillTest) Apply(s *State) (Result, error) {
	c, err := s.character(m.Actor)
	if err != nil {
		return Result{}, err
	}
	if m.Skill == "" {
		return Result{}, fmt.Errorf("%w: empty skill name", ErrInvalidArgument)
	}
	grade, err := difficulty(m.Grade, m.Modifiers)
	if err != nil {
		return Result{}, err
	}
	outcome, audit, err := rules.SkillTestCharacter(s.Rand(), c, m.Skill, grade, false)
	if err != nil {
		return Result{}, err
	}
	facts := []string{
		fmt.Sprintf("%s: %d%% base → %d%% under %s difficulty", m.Skill, audit.BaseSkill, audit.ModifiedSkill, audit.Grade),
		fmt.Sprintf("roll: %d", audit.Roll),
		"outcome: " + outcome.Grade.String(),
	}
	a := Audit{}
	a.addRoll(d100Roll(audit.Roll))
	a.addCitation(audit.RuleRef)
	summary := fmt.Sprintf("%s test: %d vs %d%% (%s) — %s", m.Skill, audit.Roll, audit.ModifiedSkill, audit.Grade, outcome.Grade)
	return newResult(summary, facts, a), nil
}

// OpposedTest resolves a contest between two characters (tool:
// opposed_test): both sides roll their named skills under the same
// difficulty, then the better Level of Success wins with ties broken by the
// higher roll (Mythras Imperative, Skills: Opposed Rolls).
type OpposedTest struct {
	// ActorA and SkillA identify the first side (character ID and skill).
	ActorA string `json:"actor_a"`
	SkillA string `json:"skill_a"`
	// ActorB and SkillB identify the second side.
	ActorB string `json:"actor_b"`
	SkillB string `json:"skill_b"`
	// Grade is the shared difficulty grade name; empty means standard.
	Grade string `json:"grade,omitempty"`
	// Modifiers lists additional difficulty names; the hardest wins (see
	// [SkillTest]).
	Modifiers []string `json:"modifiers,omitempty"`
}

// Kind implements [Move].
func (m OpposedTest) Kind() string { return "opposed_test" }

// Apply validates both characters, both skills and the difficulty before
// rolling anything, rolls both d100s through rules.SkillTest and resolves
// the contest with rules.Opposed. Both skills are looked up first so a typo
// in the second side cannot consume the first side's roll.
func (m OpposedTest) Apply(s *State) (Result, error) {
	actorA, err := s.character(m.ActorA)
	if err != nil {
		return Result{}, err
	}
	actorB, err := s.character(m.ActorB)
	if err != nil {
		return Result{}, err
	}
	if m.SkillA == "" || m.SkillB == "" {
		return Result{}, fmt.Errorf("%w: both sides need a skill name", ErrInvalidArgument)
	}
	grade, err := difficulty(m.Grade, m.Modifiers)
	if err != nil {
		return Result{}, err
	}
	skillA, err := rules.SkillValue(actorA, m.SkillA)
	if err != nil {
		return Result{}, fmt.Errorf("resolve: opposed test side A: %w", err)
	}
	skillB, err := rules.SkillValue(actorB, m.SkillB)
	if err != nil {
		return Result{}, fmt.Errorf("resolve: opposed test side B: %w", err)
	}
	outA, auditA, err := rules.SkillTest(s.Rand(), skillA, grade, false)
	if err != nil {
		return Result{}, err
	}
	outB, auditB, err := rules.SkillTest(s.Rand(), skillB, grade, false)
	if err != nil {
		return Result{}, err
	}
	contest := rules.Opposed(outA, outB)
	facts := []string{
		fmt.Sprintf("%s (%s): roll %d vs %d%% — %s", m.ActorA, m.SkillA, auditA.Roll, auditA.ModifiedSkill, outA.Grade),
		fmt.Sprintf("%s (%s): roll %d vs %d%% — %s", m.ActorB, m.SkillB, auditB.Roll, auditB.ModifiedSkill, outB.Grade),
	}
	var winnerLine string
	switch {
	case contest.BothFailed:
		winnerLine = "winner: none — both sides failed"
	case contest.Winner == 1:
		winnerLine = fmt.Sprintf("winner: %s by %d Level(s) of Success", m.ActorA, contest.Margin)
	case contest.Winner == 2:
		winnerLine = fmt.Sprintf("winner: %s by %d Level(s) of Success", m.ActorB, contest.Margin)
	default:
		winnerLine = "winner: none"
	}
	facts = append(facts, winnerLine)
	if contest.TieBreak != "" {
		facts = append(facts, "tie broken by: "+contest.TieBreak)
	}
	a := Audit{}
	a.addRoll(d100Roll(auditA.Roll))
	a.addRoll(d100Roll(auditB.Roll))
	a.addCitation(auditA.RuleRef)
	a.addCitation(auditB.RuleRef)
	a.addCitation("Mythras Imperative, Skills: Opposed Rolls")
	summary := fmt.Sprintf("opposed %s vs %s: %d/%d — %s", m.SkillA, m.SkillB, outA.Grade, outB.Grade, winnerLine)
	return newResult(summary, facts, a), nil
}

// LuckPoint spends one Luck Point on a printed use — Cheat Fate, Desperate
// Effort or Mitigate Damage (tool: luck_point; rules.SpendLuck does the
// deterministic bookkeeping, including the Cheat Fate replacement d100).
type LuckPoint struct {
	// Actor is the character spending the point (ID).
	Actor string `json:"actor"`
	// Purpose is the printed use: "cheat fate", "desperate effort" or
	// "mitigate damage" (case-insensitive).
	Purpose string `json:"purpose"`
}

// Kind implements [Move].
func (m LuckPoint) Kind() string { return "luck_point" }

// Apply validates the character and the use before rolling anything; on
// success the audit records the printed effect for narration. The Luck Point
// pool itself has no mutable field on char.Character (rules.SpendLuck
// documents this), so the spend is recorded as facts, not a decrement.
func (m LuckPoint) Apply(s *State) (Result, error) {
	c, err := s.character(m.Actor)
	if err != nil {
		return Result{}, err
	}
	use, err := rules.SpendLuck(s.Rand(), c, m.Purpose)
	if err != nil {
		return Result{}, err
	}
	facts := []string{
		fmt.Sprintf("purpose: %s (1 of %d Luck Points)", use.Purpose, use.Max),
		"effect: " + use.Effect,
	}
	a := Audit{}
	if use.Reroll != 0 {
		facts = append(facts, fmt.Sprintf("replacement roll: %d", use.Reroll))
		a.addRoll(d100Roll(use.Reroll))
	}
	a.addCitation(use.RuleRef)
	summary := use.Purpose + ": 1 Luck Point spent"
	return newResult(summary, facts, a), nil
}

// difficulty folds an optional grade name and a modifier list into the one
// DifficultyGrade the rules allow: grades never stack and the hardest wins
// (Mythras Imperative, Skills: Modifying Skills L586). An empty grade means
// standard difficulty. Unknown names error wrapping [ErrInvalidArgument]
// before any dice are rolled.
func difficulty(grade string, modifiers []string) (rules.DifficultyGrade, error) {
	g := rules.DifficultyStandard
	if strings.TrimSpace(grade) != "" {
		parsed, err := rules.ParseDifficulty(grade)
		if err != nil {
			return 0, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
		}
		g = parsed
	}
	for _, mod := range modifiers {
		parsed, err := rules.ParseDifficulty(mod)
		if err != nil {
			return 0, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
		}
		g = rules.Harder(g, parsed)
	}
	return g, nil
}

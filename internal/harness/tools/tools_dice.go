package tools

import (
	"context"
	"encoding/json"

	"github.com/maxwelljens/mythras-ultra/internal/engine/resolve"
)

// difficultyGrades is the printed Difficulty Grade Table vocabulary
// (rules.ParseDifficulty accepts these names case-insensitively).
var difficultyGrades = []string{
	"automatic", "very easy", "easy", "standard", "hard", "formidable", "herculean", "hopeless",
}

// diceTools returns the dice and test tools.
func diceTools() []Tool {
	return []Tool{
		{
			Name: "roll_dice",
			Description: "Roll a dice expression on the session RNG. Use for every " +
				"free-form roll — tie-breaks, NPC d100s, table lookups — never roll " +
				"dice inside narration." + factsNote,
			Schema: objSchema(map[string]any{
				"expr":   strProp("Dice expression, e.g. \"2d6+3\", \"1d100\", \"4d6\"."),
				"reason": strProp("Why the dice are rolled; recorded in the audit log."),
			}, "expr", "reason"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Expr   string `json:"expr"`
					Reason string `json:"reason"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("expr", a.Expr); err != nil {
					return Result{}, err
				}
				if err := requireField("reason", a.Reason); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.RollDice{Expr: a.Expr, Reason: a.Reason})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "skill_test",
			Description: "Resolve one skill test against a difficulty grade. Use " +
				"whenever an action's outcome is uncertain and the rules call for a " +
				"test; the roll, effective rating and outcome grade come back as facts." + factsNote,
			Schema: objSchema(map[string]any{
				"actor":     strProp("Character ID being tested (e.g. \"pc-1\")."),
				"skill":     strProp("Skill, Passion or characteristic-base skill name."),
				"grade":     enumProp("Difficulty grade; omit for standard.", difficultyGrades...),
				"modifiers": strArrProp("Additional difficulty grade names; the hardest wins, grades never stack."),
			}, "actor", "skill"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor     string   `json:"actor"`
					Skill     string   `json:"skill"`
					Grade     string   `json:"grade"`
					Modifiers []string `json:"modifiers"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				if err := requireField("skill", a.Skill); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.SkillTest{
					Actor: a.Actor, Skill: a.Skill, Grade: a.Grade, Modifiers: a.Modifiers,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "opposed_test",
			Description: "Resolve an opposed roll between two characters — both roll " +
				"the named skills, the better Level of Success wins. Use when two " +
				"characters' actions directly contest each other." + factsNote,
			Schema: objSchema(map[string]any{
				"actor_a":   strProp("First side's character ID."),
				"skill_a":   strProp("First side's skill name."),
				"actor_b":   strProp("Second side's character ID."),
				"skill_b":   strProp("Second side's skill name."),
				"modifiers": strArrProp("Additional difficulty grade names applied to both sides; the hardest wins."),
			}, "actor_a", "skill_a", "actor_b", "skill_b"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					ActorA    string   `json:"actor_a"`
					SkillA    string   `json:"skill_a"`
					ActorB    string   `json:"actor_b"`
					SkillB    string   `json:"skill_b"`
					Modifiers []string `json:"modifiers"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				for _, f := range []struct{ name, value string }{
					{"actor_a", a.ActorA}, {"skill_a", a.SkillA},
					{"actor_b", a.ActorB}, {"skill_b", a.SkillB},
				} {
					if err := requireField(f.name, f.value); err != nil {
						return Result{}, err
					}
				}
				res, err := resolve.Apply(s, resolve.OpposedTest{
					ActorA: a.ActorA, SkillA: a.SkillA, ActorB: a.ActorB, SkillB: a.SkillB,
					Modifiers: a.Modifiers,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "luck_point",
			Description: "Spend one of the actor's Luck Points on a printed use — " +
				"cheat fate, desperate effort or mitigate damage. Use only for those " +
				"three printed uses; the effect and replacement roll come back as facts." + factsNote,
			Schema: objSchema(map[string]any{
				"actor":   strProp("Character ID spending the point."),
				"purpose": enumProp("The printed Luck Point use.", "cheat fate", "desperate effort", "mitigate damage"),
			}, "actor", "purpose"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor   string `json:"actor"`
					Purpose string `json:"purpose"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				if err := requireField("purpose", a.Purpose); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.LuckPoint{Actor: a.Actor, Purpose: a.Purpose})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
	}
}

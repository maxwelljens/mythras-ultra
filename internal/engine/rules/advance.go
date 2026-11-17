package rules

import (
	"errors"
	"fmt"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
)

var (
	// ErrAlreadyExperienced reports a second Experience Roll spent on the
	// same skill or passion within one session: "The same skill may not
	// benefit from more than one Experience Roll per session"
	// (Mythras-Imperative.md L977; digest L562). The guard reads the
	// Advanced flag; the caller clears that flag for the whole character
	// when the GM dispenses the next batch of Experience Rolls.
	ErrAlreadyExperienced = errors.New("rules: skill already received its Experience Roll this session")
)

// Gain is the audit of one applied Experience Roll (Game System ›
// Character Improvement, L950-989; digest L556-565).
type Gain struct {
	// Skill is the improved skill's name.
	Skill string `json:"skill"`
	// Old and New are the skill value before and after the whole gain.
	Old int `json:"old"`
	New int `json:"new"`
	// Roll is the raw d100 result; INT is the characteristic added to it
	// (the compared number is Roll+INT).
	Roll int `json:"roll"`
	INT  int `json:"int"`
	// Dice is the improvement die: 1 on a missed improvement, otherwise
	// 1d4+1. The free fumble increase is not part of Dice — see Free.
	Dice int `json:"dice"`
	// Free reports that the skill's pending fumble increase of +1% was
	// applied by this call (L975), so New = Old + Dice + (Free ? 1 : 0).
	Free bool `json:"free"`
}

// expTarget is one improvable sheet entry: a Skill or a Passion, both of
// which carry Value plus the Fumbled/Advanced experience flags (passions
// improve like skills — Mythras-Imperative.md, Character Improvement:
// "Passions: improved like skills"; digest L563).
type expTarget struct {
	value    *int
	fumbled  *bool
	advanced *bool
}

// experienceTarget resolves name to the sheet entry an Experience Roll can
// improve: [ErrUnknownSkill] when neither a skill nor a passion of that
// name is on the character sheet (a skill never learned must go through
// Learning New Skills, L987, which this package does not model).
func experienceTarget(c *char.Character, name string) (expTarget, error) {
	if sk, ok := c.Skill(name); ok {
		return expTarget{value: &sk.Value, fumbled: &sk.Fumbled, advanced: &sk.Advanced}, nil
	}
	if p, ok := c.Passions[name]; ok && p != nil {
		return expTarget{value: &p.Value, fumbled: &p.Fumbled, advanced: &p.Advanced}, nil
	}
	return expTarget{}, fmt.Errorf("%w: %q is not on the character sheet", ErrUnknownSkill, name)
}

// ExperienceRoll spends one already-awarded Experience Roll on the named
// skill or passion of the character sheet (Mythras-Imperative.md ›
// Increasing Existing Skills L967-977; Passions improve like skills,
// L563; digest L559-563):
//
//  1. if the target carries a pending fumble, its free +1% is applied first
//     and the Fumbled flag is cleared — multiple fumbles never stack, and
//     the flag lives on the sheet entry, so re-fumbling re-arms it;
//  2. roll 1d100, add INT, and compare to the value at roll time
//     (including that free +1%): equal or greater → +1d4+1%, less → +1%;
//  3. set the Advanced flag so a second roll this session fails with
//     [ErrAlreadyExperienced].
//
// Rolls are GM-dispensed (end of a scenario, or every 2-3 sessions of a
// long story; 1-3 rolls per session as a rule of thumb, L961-965); this
// function spends one, it does not award any. On error nothing is mutated
// and no dice are rolled.
func ExperienceRoll(r *dice.Rand, c *char.Character, skill string) (Gain, error) {
	if r == nil {
		return Gain{}, errors.New("rules: nil dice source")
	}
	if c == nil {
		return Gain{}, errors.New("rules: nil character")
	}
	sk, err := experienceTarget(c, skill)
	if err != nil {
		return Gain{}, err
	}
	if *sk.advanced {
		return Gain{}, fmt.Errorf("%w: %q", ErrAlreadyExperienced, skill)
	}

	old := *sk.value
	free := *sk.fumbled
	if free {
		// L975: the free 1% from a Fumble is applied before the roll is
		// made to improve the skill; clearing the flag means repeated
		// Fumbles of the same skill do not stack.
		*sk.value++
		*sk.fumbled = false
	}

	roll := r.D100()
	total := roll + c.Chars.INT
	diceGain := 1
	if total >= *sk.value {
		diceGain = r.D(4) + 1 // +1d4+1%
	}
	*sk.value += diceGain
	*sk.advanced = true

	return Gain{
		Skill: skill,
		Old:   old,
		New:   *sk.value,
		Roll:  roll,
		INT:   c.Chars.INT,
		Dice:  diceGain,
		Free:  free,
	}, nil
}

// ApplyFumbleIncrease grants the free +1% a skill or passion gains for
// having fumbled since the last set of Experience Rolls (Mythras-Imperative.md
// L975; digest L560-562), without spending an Experience Roll: the increase
// is free, so it applies whether or not the player later raises the skill
// with a roll (ExperienceRoll applies it itself when it is still pending).
//
// It reports whether an increase was granted. The Fumbled flag is cleared
// when one is, so multiple fumbles of the same skill never stack. It
// propagates [ErrUnknownSkill] as ExperienceRoll does.
func ApplyFumbleIncrease(c *char.Character, skill string) (bool, error) {
	if c == nil {
		return false, errors.New("rules: nil character")
	}
	sk, err := experienceTarget(c, skill)
	if err != nil {
		return false, err
	}
	if !*sk.fumbled {
		return false, nil
	}
	*sk.value++
	*sk.fumbled = false
	return true, nil
}

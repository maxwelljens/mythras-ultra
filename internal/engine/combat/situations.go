package combat

import (
	"errors"
	"fmt"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
	"github.com/maxwelljens/mythras-ultra/internal/engine/rules"
)

// Combat situations the rulebook quantifies are resolved here as
// deterministic, audit-rich operations; the ones it leaves to prose are
// covered by other package operations rather than invented mechanics:
//
//   - Charging: spend the Charge action (SpendAP) and strike through
//     ResolveAttackWith with AttackParams.Charging — Hard, weapon Size +1
//     step, Damage Modifier +1 step (+2 quadruped) (tables.md §4.9).
//   - Evade / Withdraw: the dodge Reaction (leaves the defender prone) and
//     the Withdraw Special Effect (breaks engagement) in effects.go.
//   - Knockback: the Knockback operation below (tables.md §4.10).
//   - Surprise: Surprise and SurpriseTurn below (tables.md §4.12).
//
// Not modelled: mounted trample and vehicle ramming — the source prints
// them only as creature stat-block lines with undefined terms ("Dragon
// Trample Size: C", digest §9.11) — and Outmanoeuvre's group contest,
// which needs a per-pair engagement model the engine does not carry.

// KnockbackResult is the audit of one knockback attempt.
type KnockbackResult struct {
	// Triggered reports that the damage exceeded the recipient's SIZ.
	Triggered bool `json:"triggered"`
	// Damage is the pre-reduction damage offered, SIZ the recipient's
	// characteristic, and Braced that the Brace action raised it.
	Damage int  `json:"damage"`
	SIZ    int  `json:"siz"`
	Braced bool `json:"braced,omitempty"`
	// Distance is metres pushed (1 metre per 5 points of damage over SIZ,
	// fractions rounding up), Fell that the recipient dropped prone.
	Distance int  `json:"distance,omitempty"`
	Fell     bool `json:"fell,omitempty"`
	// Rolls carries the avoided-fall rolls (Acrobatics, then Athletics).
	Rolls []NamedRoll `json:"rolls,omitempty"`
	// Facts are human-readable, present-tense audit lines for the GM.
	Facts []string `json:"facts,omitempty"`
}

// Knockback resolves the knockback the damage tables trigger: damage
// before parry and armour exceeding the recipient's SIZ pushes them back,
// and they avoid falling prone with Easy Acrobatics or Standard
// Athletics; Brace raises the effective SIZ by 50% against it
// (Mythras-Imperative.md, Combat › Close Combat › Knockback L1784;
// tables.md §4.10; Brace action tables.md §4.2).
//
// The distance itself has no avoidance roll — only falling does — and
// driving the recipient into an obstacle (Hard Athletics or Acrobatics)
// needs terrain the engine does not model, so it is announced as a GM
// fact. Errors wrap the dice and skill operations.
func Knockback(r *dice.Rand, victim *char.Character, damage int) (KnockbackResult, error) {
	res := KnockbackResult{Damage: damage}
	if r == nil {
		return res, errors.New("combat: nil rand")
	}
	if victim == nil {
		return res, errors.New("combat: nil character")
	}
	if damage < 0 {
		return res, fmt.Errorf("combat: knockback damage %d: %w", damage, char.ErrInvalidAmount)
	}
	siz := victim.Chars.SIZ
	res.SIZ = siz
	if victim.HasCondition(CondBraced) {
		res.Braced = true
		siz = (siz*3 + 1) / 2 // Brace: SIZ treated as 50% bigger vs Knockback
		res.Facts = append(res.Facts, fmt.Sprintf("%s braces: SIZ counts as %d vs the knockback.", idOf(victim), siz))
	}
	if damage <= siz {
		res.Facts = append(res.Facts, fmt.Sprintf("damage %d does not exceed SIZ %d: no knockback.", damage, siz))
		return res, nil
	}
	res.Triggered = true
	res.Distance = ceilInt(damage-siz, 5) // 1 m per 5 points (or fraction)
	res.Facts = append(res.Facts, fmt.Sprintf("%d damage over SIZ pushes %s back %d metre(s).",
		damage-siz, idOf(victim), res.Distance))

	// Avoid falling: Easy Acrobatics first, then Standard Athletics.
	fell, rolls, facts, err := avoidFall(r, victim)
	if err != nil {
		return res, err
	}
	res.Fell = fell
	res.Rolls = rolls
	res.Facts = append(res.Facts, facts...)
	if fell {
		victim.AddCondition(char.Condition{Name: char.CondProne})
		res.Facts = append(res.Facts, idOf(victim)+" fails to keep their footing and falls prone.")
	}
	res.Facts = append(res.Facts, "GM: pushed into an obstacle, the recipient needs Hard Athletics or Acrobatics or falls and trips.")
	return res, nil
}

// avoidFall rolls Easy Acrobatics and, failing that, Standard Athletics
// ("Easy Acrobatics or Standard Athletics to avoid falling prone"). It
// reports whether the victim fell, the rolls made, and narration facts.
func avoidFall(r *dice.Rand, victim *char.Character) (fell bool, rolls []NamedRoll, facts []string, err error) {
	for _, attempt := range []struct {
		skill string
		grade rules.DifficultyGrade
	}{
		{"Acrobatics", rules.DifficultyEasy},
		{"Athletics", rules.DifficultyStandard},
	} {
		sk, skErr := rules.SkillValue(victim, attempt.skill)
		if skErr != nil {
			// An unknown elective skill simply is not tried; Athletics is
			// standard and always present, so only Acrobatics can miss.
			if attempt.skill == "Athletics" {
				return true, rolls, facts, fmt.Errorf("combat: knockback: %w", skErr)
			}
			continue
		}
		modified, modErr := attempt.grade.Modify(sk)
		if modErr != nil {
			return true, rolls, facts, fmt.Errorf("combat: knockback difficulty: %w", modErr)
		}
		roll, rollErr := r.Roll("1d100")
		if rollErr != nil {
			return true, rolls, facts, fmt.Errorf("combat: knockback %s roll: %w", attempt.skill, rollErr)
		}
		rolls = append(rolls, NamedRoll{attempt.skill, roll})
		g, clsErr := rules.Classify(roll.Total, modified)
		if clsErr != nil {
			return true, rolls, facts, fmt.Errorf("combat: classify %s: %w", attempt.skill, clsErr)
		}
		if g >= rules.GradeSuccess {
			facts = append(facts, fmt.Sprintf("%s keeps their feet on a %s roll (%d vs %d).", idOf(victim), attempt.skill, roll.Total, modified))
			return false, rolls, facts, nil
		}
		facts = append(facts, fmt.Sprintf("%s fails Easy %s (%d vs %d).", idOf(victim), attempt.skill, roll.Total, modified))
	}
	return true, rolls, facts, nil
}

// SurpriseResult is the audit of one surprise application.
type SurpriseResult struct {
	// Target is the surprised character; Applied reports the state was
	// newly set (false when they were already surprised — the initiative
	// penalty never stacks).
	Target  string `json:"target"`
	Applied bool   `json:"applied"`
	// OrderBefore and OrderAfter are the target's initiative score around
	// the surprise penalty (tables.md §4.12: −10 to Initiative).
	OrderBefore int `json:"orderBefore,omitempty"`
	OrderAfter  int `json:"orderAfter,omitempty"`
	// DefenceRestored reports SurpriseTurn has run for this Round.
	DefenceRestored bool `json:"defenceRestored,omitempty"`
	// Facts are human-readable, present-tense audit lines for the GM.
	Facts []string `json:"facts,omitempty"`
}

// Surprise marks a participant surprised: −10 to their Initiative, and
// the flat-footed / no-offensive-action state until the Round ends
// (Mythras-Imperative.md, Combat › Close Combat › Surprise L1810;
// tables.md §4.12). The first successful attack against them grants a
// bonus Special Effect (ResolveAttack records it), and the strike order
// is re-sorted so the penalty shows immediately. Applying it twice does
// not stack the penalty; call SurpriseTurn when their initiative arrives
// to restore their defence for the rest of the Round.
func Surprise(e *Encounter, target *char.Character) (SurpriseResult, error) {
	res := SurpriseResult{}
	if err := e.running(); err != nil {
		return res, err
	}
	idx, err := e.indexOf(target)
	if err != nil {
		return res, err
	}
	res.Target = idOf(target)
	score := func() int {
		for i := range e.Order {
			if e.Order[i].Index == idx {
				return e.Order[i].Score
			}
		}
		return 0
	}
	if target.HasCondition(CondSurprised) {
		// Already surprised: the −10 does not stack, but the audit still
		// reports the current score.
		res.OrderBefore, res.OrderAfter = score(), score()
		res.Facts = append(res.Facts, (res.Target + " is already surprised: the initiative penalty does not stack."))
		return res, nil
	}
	res.OrderBefore = score()
	e.adjustInitiative(idx, -10)
	res.OrderAfter = score()
	e.sortOrder()
	target.AddCondition(char.Condition{Name: CondSurprised, Rounds: 1})
	res.Applied = true
	res.Facts = append(res.Facts,
		fmt.Sprintf("%s is surprised: −10 Initiative (now %d), flat-footed until their initiative arrives, no offensive Actions for the rest of the Round, and the first successful attack against them gains a bonus Special Effect.", res.Target, res.OrderAfter))
	return res, nil
}

// SurpriseTurn records that a surprised participant's initiative has
// arrived: their defence returns (they are no longer flat-footed) while
// the ban on offensive Actions stands until the Round ends (tables.md
// §4.12: "Until their Initiative | flat-footed, cannot defend" vs. "Rest
// of the Round | no offensive Actions"). Calling it for a character who
// is not surprised changes nothing.
func SurpriseTurn(e *Encounter, target *char.Character) (SurpriseResult, error) {
	res := SurpriseResult{}
	if err := e.running(); err != nil {
		return res, err
	}
	if _, err := e.indexOf(target); err != nil {
		return res, err
	}
	res.Target = idOf(target)
	cond := findCondition(target, CondSurprised)
	if cond == nil {
		res.Facts = append(res.Facts, (res.Target + " is not surprised."))
		return res, nil
	}
	if strings.Contains(cond.Notes, "defence restored") {
		res.DefenceRestored = true
		return res, nil
	}
	marked := *cond
	if marked.Notes == "" {
		marked.Notes = "defence restored"
	} else {
		marked.Notes += "; defence restored"
	}
	target.AddCondition(marked)
	res.DefenceRestored = true
	res.Applied = true
	res.Facts = append(res.Facts, (res.Target + "'s initiative arrives: they may defend again, but still no offensive Actions until the Round ends."))
	return res, nil
}

package rules

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/dice"
)

var (
	// ErrUnknownLuckUse reports a purpose that is not one of the three
	// options the rulebook defines for spending a Luck Point (Cheat Fate,
	// Desperate Effort, Mitigate Damage). Anything else — in particular an
	// Experience or Training roll, which may never be bought with Luck
	// (Mythras-Imperative.md L1267; digest L762) — is refused.
	ErrUnknownLuckUse = errors.New("rules: unknown Luck Point use")

	// ErrNoLuck reports a character with no Luck Points to spend, which for
	// a legal character (the POW table bottoms out at 1) means the cached
	// Derived snapshot was never refreshed: call Character.Refresh first.
	ErrNoLuck = errors.New("rules: no Luck Points available (refresh the character's Derived snapshot)")
)

// LuckUse is the audit of one spent Luck Point: which printed use bought it,
// the single point it cost (one Luck Point per action, L1267), the pool the
// character draws from, the rulebook's stated effect as a narration fact,
// and — for Cheat Fate — a fresh replacement d100 for the re-roll option.
type LuckUse struct {
	// Purpose is the canonical rulebook name of the use.
	Purpose string `json:"purpose"`
	// Cost is always 1 (L1267: only one Luck Point per Action).
	Cost int `json:"cost"`
	// Max is the character's Luck Point total at the time of the spend.
	Max int `json:"max"`
	// Reroll is a fresh d100 replacement for Cheat Fate only (the printed
	// "re-roll" option, L1271); 0 for the other uses. The printed
	// digit-swap option (75 → 57) needs no dice — the caller holding the
	// original roll computes it from that roll.
	Reroll int `json:"reroll,omitempty"`
	// Effect restates the printed rulebook effect for narration.
	Effect string `json:"effect"`
	// RuleRef cites the rulebook section this use implements.
	RuleRef string `json:"ruleRef"`
}

// luckUses transcribes the three options of Using Luck Points
// (Mythras-Imperative.md, Game System › Luck Points L1259-1279; digest
// L760-770); reroll marks the only option that involves dice.
var luckUses = []struct {
	name   string
	reroll bool
	effect string
}{
	{
		name:   "Cheat Fate",
		reroll: true,
		effect: "re-roll or swap any dice roll the character makes (skill, damage, anything with an effect), or force an opponent to re-roll an attack or damage roll made against them",
	},
	{
		name:   "Desperate Effort",
		effect: "gain one additional Action Point after Action Points are exhausted",
	},
	{
		name:   "Mitigate Damage",
		effect: "downgrade a Major Wound to a Serious Wound: the damage becomes one Hit Point less than the Major Wound threshold",
	},
}

// LuckPoints returns the character's Luck Point total from POW (tables.md
// §1.5; digest L211-212) — the value Luck Points replenish to at the
// beginning of each session (L1263). It reads the cached Derived snapshot,
// so callers keep it fresh with Character.Refresh; a nil character or an
// unrefreshed cache yields 0.
func LuckPoints(c *char.Character) int {
	if c == nil {
		return 0
	}
	return c.Derived.LuckPoints
}

// SpendLuck records spending one Luck Point on purpose, where purpose is one
// of the three printed uses matched case-insensitively ("cheat fate",
// "desperate effort", "mitigate damage"). It validates the use and the pool,
// then returns the [LuckUse] audit carrying the printed effect for
// narration; for Cheat Fate it also rolls the replacement d100 (the
// digit-swap option stays with the caller, who holds the original roll).
// The effects themselves — which roll to re-roll, which wound to downgrade,
// the extra Action Point — belong to the caller (resolve/combat), which owns
// that context; this function is the deterministic bookkeeping.
//
// ErrUnknownLuckUse and ErrNoLuck propagate; nothing is rolled before both
// checks pass, so a refusal consumes no dice.
//
// shortcut: char.Character has no current-Luck field (char may not be
// modified), so cumulative affordability across a session is tracked by the
// caller from the returned LuckUse records — this call validates one
// expenditure only; upgrade to a char luck field if the harness needs the
// pool enforced inside the engine.
func SpendLuck(r *dice.Rand, c *char.Character, purpose string) (LuckUse, error) {
	if r == nil {
		return LuckUse{}, errors.New("rules: nil dice source")
	}
	if c == nil {
		return LuckUse{}, errors.New("rules: nil character")
	}
	max := LuckPoints(c)
	if max <= 0 {
		return LuckUse{}, ErrNoLuck
	}
	var use struct {
		name   string
		reroll bool
		effect string
	}
	found := false
	for _, u := range luckUses {
		if strings.EqualFold(u.name, purpose) {
			use, found = u, true
			break
		}
	}
	if !found {
		return LuckUse{}, fmt.Errorf("%w: %q", ErrUnknownLuckUse, purpose)
	}
	out := LuckUse{
		Purpose: use.name,
		Cost:    1,
		Max:     max,
		Effect:  use.effect,
		RuleRef: "Mythras Imperative, Game System: Luck Points",
	}
	if use.reroll {
		out.Reroll = r.D100()
	}
	return out, nil
}

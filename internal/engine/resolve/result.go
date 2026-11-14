package resolve

import (
	"fmt"
	"slices"

	"github.com/mythras-ultra/myul/internal/engine/dice"
)

// Audit is the mechanical trail a Move leaves behind: every dice roll it
// made, the rulebook sections it implements, and the state changes it
// committed. The session log stores it verbatim and the TUI's dice theatre
// animates Audit.Rolls (harness-surface.md, Tool contract).
type Audit struct {
	// Rolls are the dice as rolled, in roll order. Bare d100s the engine
	// reports only as integers are materialised as "1d100" records so the
	// theatre and the log see every draw.
	Rolls []dice.Roll `json:"rolls,omitempty"`
	// Citations are rulebook references such as "Mythras Imperative,
	// Combat: How Combat Works", taken from the engine types that carry
	// them (rules.TestAudit.RuleRef, combat.Action.Citation, ...) where
	// available. Harness-level constructs (world flags, scene, recaps) cite
	// nothing, because Mythras Imperative defines no rules for them.
	Citations []string `json:"citations,omitempty"`
	// Changes are the committed deltas in "field: before → after" form —
	// the audit twin of Result.Facts, which is written for narration.
	Changes []string `json:"changes,omitempty"`
}

// Result is what one accepted Move hands back: a one-line narratable
// Summary, machine Facts stating after-values ("arrows in quiver: 7 → 6"),
// and the mechanical Audit. The harness projects it into the tool result
// {ok, summary, facts, audit} (harness-surface.md).
type Result struct {
	// Summary is the single line the GM/UI shows for the operation.
	Summary string `json:"summary"`
	// Facts are the authoritative values the GM narrates from and must
	// never contradict.
	Facts []string `json:"facts"`
	// Audit is the dice trail, citations and changes; omitted when empty.
	Audit *Audit `json:"audit,omitempty"`
}

// newResult assembles a Result, attaching the audit only when it carries
// something (an empty Audit never reaches the log).
func newResult(summary string, facts []string, audit Audit) Result {
	r := Result{Summary: summary, Facts: facts}
	if len(audit.Rolls)+len(audit.Citations)+len(audit.Changes) > 0 {
		r.Audit = &audit
	}
	return r
}

// addRoll records r when it actually rolled dice: engine APIs report unused
// dice.Roll zero values that must not reach the audit.
func (a *Audit) addRoll(r dice.Roll) {
	if len(r.Dice) > 0 {
		a.Rolls = append(a.Rolls, r)
	}
}

// addCitation records a rulebook reference once (several facts of one move
// often cite the same section).
func (a *Audit) addCitation(c string) {
	if c != "" && !slices.Contains(a.Citations, c) {
		a.Citations = append(a.Citations, c)
	}
}

// change renders one "field: before → after" delta for Facts and Changes.
func change(what string, before, after any) string {
	return fmt.Sprintf("%s: %v → %v", what, before, after)
}

// d100Roll materialises a bare 1d100 the engine reported only as an integer
// (rules.TestAudit.Roll and friends) into an audit Roll record, so every
// draw the RNG made appears in Audit.Rolls.
func d100Roll(value int) dice.Roll {
	return dice.Roll{
		Expr:  "1d100",
		Dice:  []dice.Die{{Sides: 100, Value: value}},
		Total: value,
	}
}

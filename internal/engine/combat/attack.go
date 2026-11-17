package combat

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
	"github.com/maxwelljens/mythras-ultra/internal/engine/rules"
)

// Reaction is the defender's response to an attack: a Parry with a Combat
// Style, an Evade (dodge), or none. A defender who does not react — or
// cannot — is treated as having automatically rolled a Failure, granting a
// successful attacker its Special Effects (Mythras-Imperative.md, Combat ›
// Unable or Unwilling to Parry).
type Reaction string

const (
	// ReactionParry defends with the parrying weapon or shield.
	ReactionParry Reaction = "parry"
	// ReactionDodge defends with the Evade skill; it leaves the defender
	// prone whatever the outcome (Combat › Evading).
	ReactionDodge Reaction = "dodge"
	// ReactionNone declines to defend: an automatic Failure.
	ReactionNone Reaction = "none"
)

// Valid reports whether r is one of the three defined reactions.
func (r Reaction) Valid() bool {
	return r == ReactionParry || r == ReactionDodge || r == ReactionNone
}

// Winner values reported in AttackResult.Winner.
const (
	// WinnerAttacker means the attacker won the Differential Roll.
	WinnerAttacker = "attacker"
	// WinnerDefender means the defender won the Differential Roll.
	WinnerDefender = "defender"
	// WinnerNone means the Differential Roll granted no benefit (equal
	// levels, or nobody succeeded).
	WinnerNone = ""
)

// AttackParams carries everything ResolveAttackWith needs beyond the two
// participants: the weapon, the chosen reaction, the ranged distance in
// metres, and the GM's situational Difficulty Grade.
type AttackParams struct {
	// Attacker and Defender must both be participants of e.
	Attacker *char.Character
	Defender *char.Character
	// Weapon is the weapon the attacker strikes with: an inventory item
	// name, a sample-table name, or "" for an unarmed attack.
	Weapon string
	// Reaction is the defender's chosen response (default ReactionNone).
	Reaction Reaction
	// Distance is the range to the target in metres; 0 means melee or
	// unspecified. For ranged weapons it selects the Close/Effective/Long
	// band printed on the weapon (tables.md §5.4): at Long the damage is
	// halved and Force −1 step, and Close Range opens Choose Location
	// against stationary or unaware targets.
	Distance int
	// Grade is the GM's situational Difficulty Grade for the attack
	// (Close/Ranged Situational Modifiers tables, tables.md §4.4, §4.14).
	// Use ParseRangedGrade for the Ranged table's "Impossible" row. The
	// hardest applicable grade wins — grades never stack.
	Grade rules.DifficultyGrade
	// Charging makes the attack a Charge: Hard, weapon Size +1 step and
	// Damage Modifier +1 step (Combat › Charging; tables.md §4.9).
	Charging bool
	// Quadruped raises the charging Damage Modifier step to +2 ("or two
	// steps if a quadruped (or more legs)").
	Quadruped bool
	// StationaryOrUnaware lets ranged attackers use Choose Location below
	// Critical when they are within Close Range (Special Effect
	// Descriptions › Choose Location).
	StationaryOrUnaware bool
}

// WeaponFacts is the weapon metadata carried on an AttackResult so Special
// Effects can apply later without re-looking the weapon up.
type WeaponFacts struct {
	// Name is the weapon's display name ("Broadsword", "Bow", ...).
	Name string `json:"name"`
	// Damage is the weapon's damage dice expression.
	Damage string `json:"damage,omitempty"`
	// DamageType is the setting/GM damage-type label ("" on printed sample
	// weapons — the tables print notes, not types).
	DamageType string `json:"damageType,omitempty"`
	// Size is the melee Size (S/M/L/H/E); Force is the ranged equivalent.
	Size  string `json:"size,omitempty"`
	Force string `json:"force,omitempty"`
	// ImpaleSize is the Size used on the Impale Effects Table for ranged
	// weapons that print one.
	ImpaleSize string `json:"impaleSize,omitempty"`
	// Notes carries the printed weapon notes ("Two handed", "Can entangle").
	Notes string `json:"notes,omitempty"`
	// Ranged, Firearm, HighTech, Shield and Unarmed classify the weapon
	// for the Special Effects matrix's weapon-type columns.
	Ranged   bool `json:"ranged,omitempty"`
	Firearm  bool `json:"firearm,omitempty"`
	HighTech bool `json:"highTech,omitempty"`
	Shield   bool `json:"shield,omitempty"`
	Unarmed  bool `json:"unarmed,omitempty"`
	// DMAdds reports whether the attacker's Damage Modifier applies.
	DMAdds bool `json:"dmAdds,omitempty"`
	// Load is the printed Turns to load/reload (0 = none printed); Rapid
	// Reload reduces it by one per stack.
	Load int `json:"load,omitempty"`
	// AP and HP are the weapon's own protection and durability (the values
	// Damage Weapon and Sunder act on; 0 HP means the table prints none).
	AP int `json:"ap,omitempty"`
	HP int `json:"hp,omitempty"`
}

// ConsequenceRoll is one Endurance-opposed wound consequence: the severity
// that triggered it, the defender's Endurance roll, the (re-equalised)
// attacker roll it is opposed against, and the resulting Opposed outcome.
// A = Endurance, so Opposed.Winner 1 means the victim resists.
type ConsequenceRoll struct {
	// Location is the wounded hit location.
	Location string `json:"location"`
	// Severity is the wound level that demanded the roll.
	Severity char.Severity `json:"severity"`
	// EnduranceRoll is the defender's 1d100.
	EnduranceRoll dice.Roll `json:"enduranceRoll"`
	// EnduranceOutcome is the roll classified against the (equalised)
	// Endurance skill.
	EnduranceOutcome rules.Outcome `json:"enduranceOutcome"`
	// AttackOutcome is the attacker's original roll, re-equalised against
	// the Endurance skill and reclassified — the resistance target.
	AttackOutcome rules.Outcome `json:"attackOutcome"`
	// Opposed is the contest (A = Endurance, B = attack).
	Opposed rules.OpposedResult `json:"opposed"`
	// Resisted reports whether the victim won (failed consequences are
	// then applied — see Effect).
	Resisted bool `json:"resisted"`
	// Effect describes what a failed roll inflicted ("" when resisted).
	Effect string `json:"effect,omitempty"`
	// Forced reports that a Force Failure special effect auto-failed this
	// resistance roll.
	Forced bool `json:"forced,omitempty"`
}

// AttackResult is the complete audit of one ResolveAttack: every roll as a
// dice.Roll, every grade, the differential outcome, AP spent, ammunition
// consumed, damage arithmetic, wound severity and effects, the Endurance
// consequences, and the Special Effects the winner may now pick.
type AttackResult struct {
	// Round, Attacker and Defender locate the exchange (identifiers via
	// char ID, falling back to name).
	Round    int    `json:"round"`
	Attacker string `json:"attacker"`
	Defender string `json:"defender"`
	// Weapon is the attacking weapon's name.
	Weapon string `json:"weapon"`
	// WeaponFacts and ReactionWeapon describe the attacking weapon and the
	// defender's parrying weapon (empty Name when barehanded, dodged or no
	// reaction). Effects read them for weapon-type gating.
	WeaponFacts    WeaponFacts `json:"weaponFacts"`
	ReactionWeapon WeaponFacts `json:"reactionWeaponFacts"`

	// Ranged reports a ranged attack; Distance is metres; RangeBand is
	// "", "close", "effective" or "long" ("long" also covers distances
	// beyond the printed Long band — see ResolveAttackWith).
	Ranged    bool   `json:"ranged,omitempty"`
	Distance  int    `json:"distance,omitempty"`
	RangeBand string `json:"rangeBand,omitempty"`
	// LongRange reports the Long band: damage halved and Force −1 step
	// (tables.md §4.13).
	LongRange bool `json:"longRange,omitempty"`
	// Charging and Quadruped record the charge modifiers applied.
	Charging  bool `json:"charging,omitempty"`
	Quadruped bool `json:"quadruped,omitempty"`
	// Grade is the final applied situational Difficulty Grade; GradeNotes
	// lists which situations contributed (hardest wins, none stack).
	Grade      rules.DifficultyGrade `json:"grade"`
	GradeNotes []string              `json:"gradeNotes,omitempty"`

	// AttackSkill is the (modified, equalised) skill the attack rolled
	// against; AttackRoll and AttackOutcome are the roll and its grade.
	AttackSkill   int           `json:"attackSkill"`
	AttackRoll    dice.Roll     `json:"attackRoll"`
	AttackOutcome rules.Outcome `json:"attackOutcome"`
	AutoAttack    bool          `json:"autoAttack,omitempty"`    // difficulty Automatic: no roll
	NoAttacksRoll dice.Roll     `json:"noAttacksRoll,omitempty"` // the Serious-Wound 1d3 Rounds ban
	// Reaction records the defender's response; ReactionSkill,
	// ReactionRoll and ReactionOutcome mirror the attack fields.
	Reaction         Reaction      `json:"reaction"`
	ReactionSkill    int           `json:"reactionSkill,omitempty"`
	ReactionRoll     dice.Roll     `json:"reactionRoll,omitempty"`
	ReactionOutcome  rules.Outcome `json:"reactionOutcome,omitempty"`
	ReactionAutoFail bool          `json:"reactionAutoFail,omitempty"`
	ReactionReason   string        `json:"reactionReason,omitempty"`
	// ParriedWith names the weapon used to parry ("" when barehanded or
	// no parry).
	ParriedWith string `json:"parriedWith,omitempty"`

	// Contest is the Differential Roll; Winner is WinnerAttacker,
	// WinnerDefender or WinnerNone; SpecialEffects is the number of
	// effects the winner may pick (Levels of Success won, plus any
	// Surprise bonus), and AvailableSpecialEffects lists the matrix rows
	// that pass their weapon/roll gates for the winner.
	Contest                 rules.OpposedResult `json:"contest"`
	Winner                  string              `json:"winner"`
	SpecialEffects          int                 `json:"specialEffects,omitempty"`
	AvailableSpecialEffects []string            `json:"availableSpecialEffects,omitempty"`

	// Hit reports attacker Success or Critical; Damaged reports that the
	// damage path ran (per reaction rules — a winning dodge avoids it).
	Hit     bool `json:"hit"`
	Damaged bool `json:"damaged"`

	// Location and LocationRoll are the 1d20 hit-location roll resolved
	// through the defender's catalog (overlapping ranges resolve to the
	// first matching catalog entry — see rollLocation).
	Location     string    `json:"location,omitempty"`
	LocationRoll dice.Roll `json:"locationRoll,omitempty"`
	// DamageRoll is the weapon damage, DamageModifierRoll the Damage
	// Modifier; DamagePre is their sum after the Long-Range halving and
	// the negative-DM floor (step 1–2 of the damage application order).
	DamageRoll         dice.Roll `json:"damageRoll,omitempty"`
	DamageModifierRoll dice.Roll `json:"damageModifierRoll,omitempty"`
	DamagePre          int       `json:"damagePre,omitempty"`
	// AttackSizeUsed is the attacker's effective weapon Size for parry
	// comparison (charge step and Long-Range Force step already applied).
	AttackSizeUsed string `json:"attackSizeUsed,omitempty"`
	// ParryApplied, ParryReduction: step 3 — comparative weapon Sizes
	// (tables.md §4.5). ArmorPoints, ArmorAbsorbed: step 4. DamageApplied
	// is what reached the location's Hit Points.
	ParryApplied    bool             `json:"parryApplied,omitempty"`
	ParryReduction  int              `json:"parryReduction,omitempty"`
	ArmorPoints     int              `json:"armorPoints,omitempty"`
	ArmorComponents []ArmorComponent `json:"armorComponents,omitempty"`
	ArmorAbsorbed   int              `json:"armorAbsorbed,omitempty"`
	DamageApplied   int              `json:"damageApplied,omitempty"`
	// Severity and WoundEffects come from char.ApplyWound; Consequences
	// carries the Endurance-opposed rolls rolled afterwards.
	Severity     char.Severity     `json:"severity"`
	WoundEffects []string          `json:"woundEffects,omitempty"`
	Consequences []ConsequenceRoll `json:"consequences,omitempty"`

	// Adjustments records damage changes made by later Special Effects
	// (Maximise Damage, Impale, Bypass Armor, Circumvent Parry, ...), in
	// the order they were applied.
	Adjustments []string `json:"adjustments,omitempty"`
	// AppliedEffects lists the Special Effects already applied to this
	// exchange, in application order: the count won (SpecialEffects) caps
	// it, and a non-stackable row cannot repeat.
	AppliedEffects []string `json:"appliedEffects,omitempty"`
	// Maximised counts damage dice substituted by Maximise Damage;
	// BypassedArmor names the armour components Bypass Armor skipped.
	Maximised     int      `json:"maximised,omitempty"`
	BypassedArmor []string `json:"bypassedArmor,omitempty"`
	// ForcedFailure records that Force Failure was applied in this
	// exchange: later opposed-effect resistance rolls auto-fail.
	ForcedFailure bool `json:"forcedFailure,omitempty"`

	// APSpent and ReactionAPSpent are the Action Points paid; AmmoConsumed
	// is the ammunition round fired (0 for melee and thrown weapons).
	APSpent         int `json:"apSpent"`
	ReactionAPSpent int `json:"reactionAPSpent,omitempty"`
	AmmoConsumed    int `json:"ammoConsumed,omitempty"`
	// Malfunction reports a firearm fumble: the intended target then picks
	// the fumble-specific Special Effect the shooter suffers (applying
	// Weapon Malfunction jams it — Combat › Fumbling with Firearms).
	Malfunction bool `json:"malfunction,omitempty"`

	// Facts are human-readable, present-tense audit lines for the GM.
	Facts []string `json:"facts,omitempty"`

	// preCond snapshots the defender's conditions and life flag the moment
	// before the exchange's wound was applied, so a later Special Effect
	// that deflects the entire blow (Enhance Parry) can reverse it
	// exactly. Unexported: the session log keeps the audit fields, not the
	// rollback state.
	preCond *conditionSnapshot
}

// conditionSnapshot is the pre-wound defender state held for reversal.
type conditionSnapshot struct {
	Conditions []char.Condition
	Dead       bool
	// WoundsLen is len(Defender.Wounds) before the exchange appended its
	// record, so a reversal truncates exactly the entries added since.
	WoundsLen int
}

// ArmorComponent is one protection element at a hit location: the catalog's
// natural armour or one worn item, with the AP it contributed. Effects such
// as Bypass Armor and Sunder act on components by name.
type ArmorComponent struct {
	// Name is "natural armour" or the inventory item's ID.
	Name string `json:"name"`
	// Label is the human-readable name (item name or location name).
	Label string `json:"label"`
	// AP is the Armour Points it contributes at this location.
	AP int `json:"ap"`
}

// ResolveAttack is the atomic close-combat exchange: the attacker spends 1
// AP and rolls their Combat Style (or Unarmed), the defender's chosen
// reaction is resolved and paid for, the two are compared as a Differential
// Roll (How Combat Works steps 1-4), and on a hit the pipeline runs — hit
// location through the defender's catalog, weapon damage plus Damage
// Modifier, parry Size reduction, armour, char.ApplyWound and the
// Endurance-opposed wound consequences (steps 5-7; digest L876-889). It
// spends the Action Points itself, so the exchange is one deterministic
// operation. Distance, situational grades and charge modifiers go through
// ResolveAttackWith.
func ResolveAttack(r *dice.Rand, e *Encounter, att *char.Character, weapon string, def *char.Character, reaction Reaction) (AttackResult, error) {
	return ResolveAttackWith(r, e, AttackParams{
		Attacker: att,
		Defender: def,
		Weapon:   weapon,
		Reaction: reaction,
		Grade:    rules.DifficultyStandard,
	})
}

// ResolveAttackWith runs the full attack pipeline for the given parameters.
// See ResolveAttack for the sequence; this variant adds range bands,
// situational Difficulty Grades and charge modifiers.
func ResolveAttackWith(r *dice.Rand, e *Encounter, p AttackParams) (AttackResult, error) {
	res := AttackResult{
		Reaction:  p.Reaction,
		Grade:     p.Grade,
		Charging:  p.Charging,
		Quadruped: p.Quadruped,
	}
	if r == nil {
		return res, errors.New("combat: nil rand")
	}
	if err := e.running(); err != nil {
		return res, err
	}
	if p.Reaction == "" {
		p.Reaction = ReactionNone
		res.Reaction = ReactionNone
	}
	if !p.Reaction.Valid() {
		return res, fmt.Errorf("%w: %q", ErrUnknownReaction, p.Reaction)
	}
	if !p.Grade.Valid() {
		return res, fmt.Errorf("combat: invalid difficulty grade %d", int(p.Grade))
	}
	if _, err := e.indexOf(p.Attacker); err != nil {
		return res, fmt.Errorf("combat: attacker: %w", err)
	}
	if _, err := e.indexOf(p.Defender); err != nil {
		return res, fmt.Errorf("combat: defender: %w", err)
	}
	res.Round = e.Round
	res.Attacker = idOf(p.Attacker)
	res.Defender = idOf(p.Defender)

	// Weapon lookup and classification; errors before any state change.
	wi, err := resolveWeapon(p.Attacker, p.Weapon)
	if err != nil {
		return res, err
	}
	res.Weapon = wi.name
	res.WeaponFacts = factsOf(wi)
	res.Ranged = wi.ranged
	res.Distance = p.Distance
	res.RangeBand, res.LongRange = rangeBand(wi, p.Distance)

	// A jammed firearm stays jammed until cleaned (ErrMalfunction).
	if cond := malfunctionFor(p.Attacker, wi.name); cond != nil {
		return res, fmt.Errorf("%w: %s", ErrMalfunction, wi.name)
	}

	// Attack skill: the Combat Style covering the weapon, else the
	// first style (one difficulty grade harder — L1317), else Unarmed.
	atkSkill, _, outOfStyle, err := combatSkill(p.Attacker, wi.name, wi.unarmed)
	if err != nil {
		return res, err
	}

	// Situational difficulty: the hardest situation wins, none stack
	// (Difficulty Grade Table L453-454).
	grade := p.Grade
	if p.Grade != rules.DifficultyStandard {
		res.GradeNotes = append(res.GradeNotes, fmt.Sprintf("situational: %s", p.Grade))
	}
	if p.Charging {
		// Charging: "the attack roll suffers a Difficulty Grade of Hard".
		if harder(rules.DifficultyHard, grade) {
			grade = rules.DifficultyHard
			res.GradeNotes = append(res.GradeNotes, "charge: hard")
		}
	}
	if outOfStyle {
		// "Characters who attempt to use a weapon not covered by their
		// style should suffer a skill penalty of one or more difficulty
		// grades" (L1317). [AMBIGUOUS: the source gives no count; the
		// engine applies the minimum — one grade.]
		if harder(rules.DifficultyHard, grade) {
			grade = rules.DifficultyHard
			res.GradeNotes = append(res.GradeNotes, "weapon outside combat style: one grade harder")
		}
	}
	if p.Attacker.HasCondition(char.CondProne) {
		if harder(rules.DifficultyFormidable, grade) {
			grade = rules.DifficultyFormidable
			res.GradeNotes = append(res.GradeNotes, "fighting while prone: formidable")
		}
	}
	if p.Attacker.HasCondition(CondBlinded) {
		bg := blindedGrade(p.Attacker)
		if harder(bg, grade) {
			grade = bg
			res.GradeNotes = append(res.GradeNotes, "blinded: "+bg.String())
		}
	}
	res.Grade = grade

	// Apply the grade before spending anything so Hopeless refuses first.
	var attackSkill int
	autoAttack := false
	modified, err := grade.Modify(atkSkill)
	switch {
	case err == nil:
		attackSkill = modified
	case errors.Is(err, rules.ErrAutomatic):
		// "Attacking a helpless target | Automatic | No need to roll":
		// the blow lands without a roll (recorded as a plain Success).
		autoAttack = true
		attackSkill = atkSkill
	case errors.Is(err, rules.ErrHopeless):
		return res, fmt.Errorf("combat: attack is hopeless: %w", err)
	default:
		return res, fmt.Errorf("combat: apply difficulty: %w", err)
	}

	// Resolve the reaction plan (weapon, skill, impossibilities) before
	// any AP moves.
	plan, err := planReaction(p.Defender, p.Reaction, wi.ranged, wi.firearm)
	if err != nil {
		return res, err
	}
	res.ReactionWeapon = factsOf(plan.weapon)
	res.ParriedWith = plan.weapon.name

	// Ammunition must exist before anything is spent (thrown weapons have
	// no printed Load and consume nothing).
	if wi.ammo {
		left, err := p.Attacker.ArrowsLeft()
		if err != nil || left == 0 {
			return res, fmt.Errorf("combat: fire %s: %w", wi.name, char.ErrNoAmmo)
		}
	}

	// Step 1: the attacker spends 1 AP on the Attack action.
	if err := SpendAP(e, p.Attacker, "Attack"); err != nil {
		return res, err
	}
	res.APSpent = 1

	// Step 2: the defender spends 1 AP on their reaction — when they can.
	if p.Reaction != ReactionNone {
		if reason, blocked := reactionBlocked(e, p.Defender, plan); blocked {
			res.ReactionAutoFail, res.ReactionReason = true, reason
		} else if err := SpendAP(e, p.Defender, plan.action()); err != nil {
			return res, err
		} else {
			res.ReactionAPSpent = 1
		}
	} else {
		res.ReactionAutoFail = true
		res.ReactionReason = "defender does not react — treated as an automatic Failure (Combat › Unable or Unwilling to Parry)"
	}

	// Equalise skills over 100% before rolling (Skills L908), when both
	// sides actually roll.
	eqAttack := attackSkill
	eqReaction := plan.skill
	if !autoAttack && !res.ReactionAutoFail {
		eqAttack, eqReaction = rules.EqualiseSkillsOver100(attackSkill, plan.skill)
	}
	res.AttackSkill = eqAttack
	res.ReactionSkill = plan.skill
	if p.Reaction != ReactionNone && !res.ReactionAutoFail {
		if plan.weapon.name != "" {
			res.Facts = append(res.Facts, fmt.Sprintf("%s defends with %s (%s).", res.Defender, plan.weapon.name, plan.skillRef))
		} else {
			res.Facts = append(res.Facts, fmt.Sprintf("%s defends with %s.", res.Defender, plan.skillRef))
		}
	}

	// Attack roll.
	if autoAttack {
		res.AutoAttack = true
		res.AttackOutcome = rules.Outcome{Roll: 0, Skill: eqAttack, Grade: rules.GradeSuccess}
		res.Facts = append(res.Facts, "difficulty Automatic: no roll is needed — the attack succeeds.")
	} else {
		roll, err := r.Roll("1d100")
		if err != nil {
			return res, fmt.Errorf("combat: attack roll: %w", err)
		}
		g, err := rules.Classify(roll.Total, eqAttack)
		if err != nil {
			return res, fmt.Errorf("combat: classify attack: %w", err)
		}
		res.AttackRoll = roll
		res.AttackSkill = eqAttack
		res.AttackOutcome = rules.Outcome{Roll: roll.Total, Skill: eqAttack, Grade: g}
	}

	// Reaction roll (or automatic Failure).
	if res.ReactionAutoFail {
		res.ReactionOutcome = rules.Outcome{Roll: 0, Skill: plan.skill, Grade: rules.GradeFailure}
	} else {
		roll, err := r.Roll("1d100")
		if err != nil {
			return res, fmt.Errorf("combat: reaction roll: %w", err)
		}
		g, err := rules.Classify(roll.Total, eqReaction)
		if err != nil {
			return res, fmt.Errorf("combat: classify reaction: %w", err)
		}
		res.ReactionRoll = roll
		res.ReactionSkill = eqReaction
		res.ReactionOutcome = rules.Outcome{Roll: roll.Total, Skill: eqReaction, Grade: g}
	}

	// Step 3: Differential Roll — equal levels and mutual failure grant
	// no benefit (Skills › Differential Rolls L893-906).
	res.Contest = rules.Differential(res.AttackOutcome, res.ReactionOutcome)
	switch res.Contest.Winner {
	case 1:
		res.Winner = WinnerAttacker
	case 2:
		res.Winner = WinnerDefender
	default:
		res.Winner = WinnerNone
	}
	// Step 4: difference grants the superior roller Special Effects.
	res.SpecialEffects = res.Contest.Margin

	attSuccess := res.AttackOutcome.Grade >= rules.GradeSuccess
	defSuccess := res.ReactionOutcome.Grade >= rules.GradeSuccess
	res.Hit = attSuccess
	if res.Winner == WinnerNone {
		res.Facts = append(res.Facts, "neither side gains a benefit from the differential.")
	} else {
		res.Facts = append(res.Facts, fmt.Sprintf("%s wins %d Level(s) of Success and may pick that many Special Effects.", res.Winner, res.SpecialEffects))
	}

	// Damage path. A winning dodge avoids the blow completely; a parry
	// reduces by comparative Sizes when it succeeds; no reaction reduces
	// nothing (How Combat Works steps 5-7).
	damageAllowed := attSuccess
	switch {
	case p.Reaction == ReactionDodge && res.Winner == WinnerDefender:
		// Evading: "If the defender wins, damage is completely avoided"
		// (L1778-1784).
		damageAllowed = false
		res.Facts = append(res.Facts, "the dodge completely avoids the blow.")
	case p.Reaction == ReactionDodge:
		// [AMBIGUOUS: Evading says the attacker's win inflicts damage as
		// per normal and the defender's win avoids it, but does not name
		// the tie case; the general How Combat Works step 5 applies — an
		// attacking Success lands, with no Size reduction (Evade carries
		// none). digest §5, Evading.]
		res.Facts = append(res.Facts, "Evade does not reduce damage by Size — only a winning dodge avoids the blow.")
	}
	if !damageAllowed {
		if p.Reaction == ReactionNone || res.ReactionAutoFail {
			res.Facts = append(res.Facts, "the attack misses and no reaction is taken: the exchange ends.")
		} else {
			res.Facts = append(res.Facts, "the attack does not land: no damage is rolled.")
		}
	}

	if damageAllowed {
		// Hit location: 1d20 through the defender's catalog. Overlapping
		// ranges (e.g. the shark's 17-18 Left Fin / Head) resolve to the
		// first matching catalog entry in table order — deterministic.
		locRoll, err := r.Roll("1d20")
		if err != nil {
			return res, fmt.Errorf("combat: hit location: %w", err)
		}
		loc, err := rollLocation(p.Defender, locRoll.Total)
		if err != nil {
			return res, err
		}
		res.LocationRoll = locRoll
		res.Location = loc

		// Step 5: weapon damage + Damage Modifier.
		dmgRoll, err := r.Roll(wi.damage)
		if err != nil {
			return res, fmt.Errorf("combat: weapon damage %q: %w", wi.damage, err)
		}
		res.DamageRoll = dmgRoll
		res.Damaged = true
		pre := dmgRoll.Total
		if wi.dmAdds {
			dmExpr, err := damageModifierExpr(p.Attacker, p)
			if err != nil {
				return res, err
			}
			dmRoll, err := r.Roll(dmExpr)
			if err != nil {
				return res, fmt.Errorf("combat: damage modifier %q: %w", dmExpr, err)
			}
			res.DamageModifierRoll = dmRoll
			pre += dmRoll.Total
		}
		// "If a negative DM takes damage to zero or below, no damage at
		// all is inflicted" (Step 3, Damage Modifier).
		if pre < 0 {
			pre = 0
		}
		// Long Range: damage halved (rounding up — Introduction › Rounding).
		if res.LongRange && pre > 0 {
			pre = ceilHalf(pre)
			res.Facts = append(res.Facts, "Long Range: the damage is halved and Force −1 step.")
		}
		res.DamagePre = pre
		res.AttackSizeUsed = effectiveAttackSize(wi, p)

		// Knockback trigger hint (the mechanics live in Knockback).
		if pre > 0 && pre > p.Defender.Chars.SIZ {
			res.Facts = append(res.Facts, fmt.Sprintf("damage %d exceeds the target's SIZ %d: knockback applies.", pre, p.Defender.Chars.SIZ))
		}

		// Step 6: parry Size reduction when the defender succeeded.
		afterParry := pre
		if p.Reaction == ReactionParry && defSuccess && pre > 0 {
			red := parryReduction(pre, res.AttackSizeUsed, plan.weapon.size)
			if red > 0 {
				res.ParryApplied = true
				res.ParryReduction = red
				afterParry = pre - red
				res.Facts = append(res.Facts, fmt.Sprintf("the parry deflects %d of %d damage by comparative Size.", red, pre))
			} else {
				res.Facts = append(res.Facts, "the parrying weapon's Size deflects none of the damage.")
			}
		}

		// Step 7: armour.
		comps, err := armorComponents(p.Defender, loc)
		if err != nil {
			return res, err
		}
		res.ArmorComponents = comps
		for _, c := range comps {
			res.ArmorPoints += c.AP
		}
		absorbed := 0
		if afterParry > 0 && res.ArmorPoints > 0 {
			absorbed = min(afterParry, res.ArmorPoints)
		}
		res.ArmorAbsorbed = absorbed
		applied := afterParry - absorbed
		if applied < 0 {
			applied = 0
		}
		res.DamageApplied = applied

		if applied > 0 {
			// Snapshot the defender's pre-wound conditions and life flag so
			// a later Special Effect can reverse the exchange exactly
			// (Enhance Parry deflects the entire blow).
			res.preCond = &conditionSnapshot{
				Conditions: append([]char.Condition(nil), p.Defender.Conditions...),
				Dead:       p.Defender.Dead,
				WoundsLen:  len(p.Defender.Wounds),
			}
			sev, effects, err := p.Defender.ApplyWound(loc, applied)
			if err != nil {
				return res, fmt.Errorf("combat: apply wound: %w", err)
			}
			res.Severity = sev
			res.WoundEffects = effects
			res.Facts = append(res.Facts, fmt.Sprintf("%s takes %d damage to the %s (%s).", idOf(p.Defender), applied, loc, sev))
			// A fresh injury reopens First Aid's once-per-injury use.
			removeConditionAt(p.Defender, CondFirstAid, loc)

			// The Serious-Wound attack ban is a rolled 1d3 Rounds.
			if sev == char.SeveritySerious {
				ban, err := r.Roll("1d3")
				if err != nil {
					return res, fmt.Errorf("combat: serious wound ban: %w", err)
				}
				res.NoAttacksRoll = ban
				p.Defender.AddCondition(char.Condition{Name: CondNoAttacks, Rounds: ban.Total})
				res.Facts = append(res.Facts, fmt.Sprintf("Serious Wound: no attacks or spellcasting for %d Rounds.", ban.Total))
			}
			// Endurance-opposed wound consequences (after effects, per
			// L889 — effects are applied by the GM afterwards; the roll
			// is made here so the audit is complete).
			cons, err := rollConsequences(r, &res, p.Defender, sev, loc, applied)
			if err != nil {
				return res, err
			}
			res.Consequences = cons
			for _, c := range cons {
				res.Facts = append(res.Facts, fmt.Sprintf("Endurance %s vs the attack roll: %s", c.EnduranceOutcome, consequenceText(c)))
			}
		} else {
			res.Facts = append(res.Facts, fmt.Sprintf("the blow lands but the armour absorbs all %d damage.", absorbed))
		}
	}

	// Fumbling with Firearms: "whomever was the intended target is allowed
	// to select a fumble specific special effect, which the shooter then
	// suffers" (L1923-1925). The pick is offered through
	// AvailableSpecialEffects; applying Weapon Malfunction is what actually
	// jams the weapon (see computeAvailable).
	if wi.firearm && res.AttackOutcome.Grade == rules.GradeFumble {
		res.Malfunction = true
		res.Facts = append(res.Facts, wi.name+" fumbles: the intended target selects a fumble-specific Special Effect for the shooter to suffer.")
	}

	// Surprise: the first successful attack on a surprised target gains a
	// bonus Special Effect (tables.md §4.12).
	if attSuccess && p.Defender.HasCondition(CondSurprised) {
		if cond := findCondition(p.Defender, CondSurprised); cond != nil && !strings.Contains(cond.Notes, "bonus spent") {
			res.SpecialEffects++
			marked := *cond
			marked.Notes = strings.TrimSuffix(marked.Notes, "; bonus spent") + "; bonus spent"
			p.Defender.AddCondition(marked)
			res.Facts = append(res.Facts, "the first attack on a surprised target gains a bonus Special Effect.")
		}
	}

	// A dodge leaves the defender prone whatever the outcome (Evading).
	if p.Reaction == ReactionDodge && res.ReactionAPSpent > 0 {
		p.Defender.AddCondition(char.Condition{Name: char.CondProne})
		res.Facts = append(res.Facts, "Evade leaves the defender prone.")
	}

	// Fire the shot last so an earlier failure wastes no ammunition.
	if wi.ammo {
		if err := p.Attacker.ConsumeAmmo(1); err != nil {
			return res, fmt.Errorf("combat: consume ammunition: %w", err)
		}
		res.AmmoConsumed = 1
	}

	res.AvailableSpecialEffects = computeAvailable(&res, p.StationaryOrUnaware)
	return res, nil
}

// reactionPlan is the resolved reaction: which skill rolls, with which
// weapon, or why the reaction is impossible (and therefore an automatic
// Failure).
type reactionPlan struct {
	reaction Reaction
	weapon   weaponInfo
	skill    int
	skillRef string // "Evade" or the combat style name, for audit
	autoFail bool
	reason   string
}

// action returns the Combat Action the reaction pays for.
func (pl reactionPlan) action() string {
	if pl.reaction == ReactionDodge {
		return "Evade"
	}
	return "Parry"
}

// planReaction resolves the defender's reaction before any AP is spent.
// A parry needs something to parry with (a shield against missile fire;
// firearms cannot be actively parried at all — Passive Blocking only); a
// dodge needs the Evade skill. Impossibilities become automatic Failures
// rather than errors, per Combat › Unable or Unwilling to Parry.
func planReaction(def *char.Character, reaction Reaction, ranged, firearm bool) (reactionPlan, error) {
	plan := reactionPlan{reaction: reaction}
	if reaction == ReactionNone {
		plan.autoFail = true
		plan.reason = "defender does not react"
		return plan, nil
	}
	if reaction == ReactionDodge {
		s, ok := def.Skill("Evade")
		if !ok {
			return plan, fmt.Errorf("combat: evade with %s: %w", idOf(def), char.ErrUnknownSkill)
		}
		plan.skill = s.Value
		plan.skillRef = "Evade"
		return plan, nil
	}
	// Parry.
	weapon, canParry, reason := selectParryWeapon(def, ranged, firearm)
	if !canParry {
		plan.autoFail = true
		plan.reason = reason
		return plan, nil
	}
	plan.weapon = weapon
	skill, ref, outOfStyle, err := combatSkill(def, weapon.name, weapon.unarmed)
	if err != nil {
		return plan, err
	}
	if outOfStyle {
		// The defender's out-of-style penalty mirrors the attacker's.
		if modified, err := rules.DifficultyHard.Modify(skill); err == nil {
			skill = modified
		}
	}
	plan.skill = skill
	plan.skillRef = ref
	return plan, nil
}

// reactionBlocked reports whether the reaction must be an automatic Failure
// and why: flat-footed under Surprise, or any CanAct blocker (no AP,
// unconscious, and so on).
func reactionBlocked(e *Encounter, def *char.Character, plan reactionPlan) (string, bool) {
	if plan.autoFail {
		return plan.reason, true
	}
	if cond := findCondition(def, CondSurprised); cond != nil && !strings.Contains(cond.Notes, "defence restored") {
		return "flat-footed until their initiative arrives (Surprise): they cannot defend", true
	}
	action := plan.action()
	if ok, blockers := CanAct(e, def, action); !ok {
		return strings.Join(blockers, "; "), true
	}
	return "", false
}

// combatSkill resolves the skill used to attack or parry with a weapon:
// the Combat Style whose weapon list contains it; otherwise the character's
// first style (alphabetically — maps must not decide order) with
// outOfStyle set, so ResolveAttack charges one difficulty grade; with no
// styles at all, the Unarmed skill (digest L472: Unarmed is the universal
// combat skill). Unarmed strikes always use the Unarmed skill directly.
func combatSkill(c *char.Character, weaponName string, unarmed bool) (value int, ref string, outOfStyle bool, err error) {
	if c == nil {
		return 0, "", false, errors.New("combat: nil character")
	}
	if !unarmed {
		if names := styleNames(c); len(names) > 0 {
			for _, n := range names {
				if styleHasWeapon(c.CombatStyles[n], weaponName) {
					return c.CombatStyles[n].Value, "Combat Style (" + n + ")", false, nil
				}
			}
			// Weapon outside every style: use the first style and charge
			// one difficulty grade ("one or more grades", L1317).
			return c.CombatStyles[names[0]].Value, "Combat Style (" + names[0] + ")", true, nil
		}
	}
	s, ok := c.Skill("Unarmed")
	if !ok {
		return 0, "", false, fmt.Errorf("combat: %s: %w: Unarmed", idOf(c), char.ErrUnknownSkill)
	}
	return s.Value, "Unarmed", len(styleNames(c)) > 0, nil
}

// styleNames returns the character's combat style names in sorted order so
// every lookup is deterministic (Go map iteration is randomised).
func styleNames(c *char.Character) []string {
	names := make([]string, 0, len(c.CombatStyles))
	for n := range c.CombatStyles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// styleHasWeapon reports whether the style lists the weapon (the printed
// style weapons are matched case-insensitively).
func styleHasWeapon(cs *char.CombatStyle, weapon string) bool {
	if cs == nil || weapon == "" {
		return false
	}
	for _, w := range cs.Weapons {
		if strings.EqualFold(strings.TrimSpace(w), weapon) {
			return true
		}
	}
	return false
}

// selectParryWeapon picks the weapon the defender parries with: a shield
// against missile fire (nothing else parries arrows — Ranged Combat); for
// firearms, nothing at all (active parry impossible, Passive Blocking
// only). In melee the main-hand weapon is preferred, then a shield, then
// any equipped weapon; barehanded, the Unarmed skill defends with no Size
// reduction. An impaled weapon cannot parry (Impale), so it is skipped.
func selectParryWeapon(def *char.Character, ranged, firearm bool) (weaponInfo, bool, string) {
	if ranged && firearm {
		return weaponInfo{}, false, "firearms cannot be actively parried — shields work by Passive Blocking only (Combat › Parrying Weapon Fire with Shields)"
	}
	if ranged {
		for _, it := range def.Inventory {
			if it != nil && it.Equipped && it.Shield != nil {
				info, err := infoFromItem(it)
				if err == nil {
					return info, true, ""
				}
			}
		}
		return weaponInfo{}, false, "only shields normally parry ranged fire (Combat › Ranged Combat)"
	}
	candidates := []*char.Item{}
	for _, it := range def.Inventory {
		if it != nil && it.Equipped && it.Slot == "main" && (it.Weapon != nil || it.Shield != nil) {
			candidates = append(candidates, it)
		}
	}
	for _, it := range def.Inventory {
		if it != nil && it.Equipped && it.Shield != nil && it.Slot != "main" {
			candidates = append(candidates, it)
		}
	}
	for _, it := range def.Inventory {
		if it != nil && it.Equipped && it.Weapon != nil && it.Slot != "main" {
			candidates = append(candidates, it)
		}
	}
	for _, it := range candidates {
		if impaledHolds(def, it.Name) {
			continue // an impaled weapon cannot parry
		}
		info, err := infoFromItem(it)
		if err != nil {
			continue
		}
		return info, true, ""
	}
	// Nothing suitable (barehanded, or every candidate impaled): parry
	// with the body — the Unarmed skill, with no Size reduction.
	info, err := resolveWeapon(def, "")
	if err == nil {
		return info, true, ""
	}
	return weaponInfo{}, false, "the defender has nothing to parry with"
}

// resolveWeapon resolves a weapon name to its stats: first an inventory
// item by display name (equipped items first, so a held weapon beats the
// stowed copy), then the sample weapon tables (melee first, per
// char.WeaponByName). "" or "unarmed" means an unarmed strike using the
// Fist/Kick row. Errors wrap [ErrUnknownWeapon].
func resolveWeapon(c *char.Character, weapon string) (weaponInfo, error) {
	trimmed := strings.TrimSpace(weapon)
	if trimmed == "" || strings.EqualFold(trimmed, "unarmed") {
		trimmed = "Fist/Kick"
	}
	for _, pass := range []bool{true, false} {
		for _, it := range c.Inventory {
			if it == nil || !strings.EqualFold(it.Name, trimmed) {
				continue
			}
			if pass != it.Equipped {
				continue
			}
			if it.Weapon == nil && it.Shield == nil {
				continue
			}
			return infoFromItem(it)
		}
	}
	entry, err := char.WeaponByName(trimmed)
	if err != nil {
		return weaponInfo{}, fmt.Errorf("%w: %s cannot use %q", ErrUnknownWeapon, idOf(c), weapon)
	}
	info := infoFromStats(entry.Name, entry.Stats, entry.Tags)
	info.unarmed = strings.EqualFold(entry.Name, "Fist/Kick") && (strings.TrimSpace(weapon) == "" || strings.EqualFold(strings.TrimSpace(weapon), "unarmed"))
	return info, nil
}

// infoFromItem builds a weaponInfo from an inventory item.
func infoFromItem(it *char.Item) (weaponInfo, error) {
	if it.Weapon != nil {
		return infoFromStats(it.Name, *it.Weapon, it.Tags), nil
	}
	if it.Shield != nil {
		stats := char.WeaponStats{
			Damage: it.Shield.Damage,
			Size:   it.Shield.Size,
			DMAdds: true, // shields are melee tools: the DM applies
			AP:     it.Shield.AP,
			HP:     it.Shield.HP,
			Notes:  "shield",
		}
		return infoFromStats(it.Name, stats, it.Tags), nil
	}
	return weaponInfo{}, fmt.Errorf("%w: item %q carries no weapon statistics", ErrUnknownWeapon, it.Name)
}

// infoFromStats classifies resolved weapon statistics: ranged by tag or a
// printed Range; firearms by the sample firearm rows or a "firearm" note;
// high-tech firearms by their note; ammunition consumption by a printed
// Load (thrown weapons print "—" and consume nothing).
func infoFromStats(name string, stats char.WeaponStats, tags []string) weaponInfo {
	ranged := stats.Range != ""
	for _, t := range tags {
		if t == char.TagRanged {
			ranged = true
		}
	}
	lowerNotes := strings.ToLower(stats.Notes)
	firearm := firearmNames[strings.ToLower(name)] || strings.Contains(lowerNotes, "firearm")
	info := weaponInfo{
		name:       name,
		damage:     stats.Damage,
		size:       stats.Size,
		force:      stats.Force,
		rangeField: stats.Range,
		impaleSize: stats.ImpaleSize,
		dmAdds:     stats.DMAdds,
		load:       stats.Load,
		ranged:     ranged,
		firearm:    firearm && ranged,
		highTech:   strings.Contains(lowerNotes, "high tech") || strings.Contains(lowerNotes, "high-tech"),
		shield:     hasTag(tags, char.TagShield) || stats.Notes == "shield",
		unarmed:    strings.EqualFold(name, "Fist/Kick"),
		ammo:       ranged && stats.Load > 0,
		notes:      stats.Notes,
		damageType: stats.DamageType,
		ap:         stats.AP,
		hp:         stats.HP,
	}
	if stats.Damage == "" {
		info.damage = "1d3" // weapons without a printed damage roll with fists
	}
	return info
}

// firearmNames are the sample firearm rows (tables.md §5.4). Custom
// firearms are recognised by a "firearm" note on the weapon — the sample
// tables print no firearm column.
var firearmNames = map[string]bool{
	"pistol":       true,
	"rifle":        true,
	"shotgun":      true,
	"laser rifle":  true,
	"plasma rifle": true,
	"gauss rifle":  true,
}

// weaponInfo is the internal, fully classified weapon of one exchange.
type weaponInfo struct {
	name       string
	damage     string
	size       string
	force      string
	rangeField string
	impaleSize string
	dmAdds     bool
	load       int
	ranged     bool
	firearm    bool
	highTech   bool
	shield     bool
	unarmed    bool
	ammo       bool
	notes      string
	damageType string
	ap         int
	hp         int
}

// hasTag reports whether tags contains want.
func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

// factsOf projects a weaponInfo onto its audit record.
func factsOf(w weaponInfo) WeaponFacts {
	return WeaponFacts{
		Name:       w.name,
		Damage:     w.damage,
		DamageType: w.damageType,
		Size:       w.size,
		Force:      w.force,
		ImpaleSize: w.impaleSize,
		Notes:      w.notes,
		Ranged:     w.ranged,
		Firearm:    w.firearm,
		HighTech:   w.highTech,
		Shield:     w.shield,
		Unarmed:    w.unarmed,
		DMAdds:     w.dmAdds,
		AP:         w.ap,
		HP:         w.hp,
	}
}

// rangeBand maps a distance in metres onto the weapon's printed
// Close/Effective/Long bands ("15/100/200"). Melee weapons, distances of 0
// and weapons without a printed Range report "". A distance beyond the
// printed Long band reports "long":
// [AMBIGUOUS: the source defines no band beyond Long — the engine applies
// the Long effects and records a fact, leaving any extra grade to the GM
// via AttackParams.Grade — Combat › Ranged Combat L1833-1851.]
func rangeBand(w weaponInfo, distance int) (band string, long bool) {
	if !w.ranged || distance <= 0 {
		return "", false
	}
	band = bandForRange(distance, parseRangeBands(w.rangeField))
	return band, band == "long"
}

// parseRangeBands splits the printed "Close/Effective/Long" metres.
func parseRangeBands(s string) [3]int {
	var out [3]int
	if s == "" {
		return out
	}
	parts := strings.Split(s, "/")
	for i, p := range parts {
		if i > 2 {
			break
		}
		var n int
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				continue
			}
			n = n*10 + int(ch-'0')
		}
		out[i] = n
	}
	return out
}

// bandForRange classifies a distance against the three printed bands.
func bandForRange(distance int, bands [3]int) string {
	if bands[0] == 0 && bands[1] == 0 && bands[2] == 0 {
		return ""
	}
	switch {
	case distance <= bands[0]:
		return "close"
	case distance <= bands[1]:
		return "effective"
	default:
		return "long"
	}
}

// rangedSituationalModifiers transcribes the Ranged Situational Modifiers
// table (tables.md §4.14; Mythras-Imperative.md, Combat › Ranged Combat
// L1853-1879), keyed by the printed situation name. The footnotes travel
// with the rows: * the wind rows assume wind-vulnerable ammunition; ** the
// "Impossible" row's location-known footnote is handled in
// ParseRangedGrade; *** Attacker Prone is negated by a crossbow or
// firearm from a prepared position — the caller then passes the easier
// grade instead.
var rangedSituationalModifiers = []struct {
	situation string
	grade     rules.DifficultyGrade
}{
	{"light wind", rules.DifficultyHard},
	{"moderate wind", rules.DifficultyFormidable},
	{"strong wind", rules.DifficultyHerculean},
	{"gale, storm, or worse", rules.DifficultyHopeless},
	{"target is running", rules.DifficultyHard},
	{"target is sprinting", rules.DifficultyFormidable},
	{"target obscured by mist", rules.DifficultyHard},
	{"target obscured by partial darkness", rules.DifficultyHard},
	{"target obscured by thick smoke", rules.DifficultyFormidable},
	{"target obscured by darkness", rules.DifficultyFormidable},
	{"target completely obscured", rules.DifficultyHerculean},
	{"target completely invisible", rules.DifficultyHerculean}, // location known — see ParseRangedGrade
	{"target prone", rules.DifficultyFormidable},
	{"attacker prone", rules.DifficultyHerculean},
	{"attacker on unstable ground", rules.DifficultyHard},
}

// ParseRangedGrade maps a situation name from the Ranged Situational
// Modifiers table (tables.md §4.14) — or a plain difficulty-grade name —
// onto a Difficulty Grade. The table's "Impossible" row (a completely
// invisible target) is not in the Difficulty Grade Table; its footnote
// says to treat it as Herculean when the target's general location is
// known.
// [AMBIGUOUS: when the location is not known the source prints no grade at
// all — the engine refuses the attempt as Hopeless ("no attempt can be
// made"), the nearest defined outcome — digest §9.3, Combat › Ranged
// Combat.]
func ParseRangedGrade(name string, locationKnown bool) (rules.DifficultyGrade, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "impossible" {
		if locationKnown {
			return rules.DifficultyHerculean, nil
		}
		return rules.DifficultyHopeless, errors.New("combat: Impossible without the target's general location — no attempt can be made")
	}
	for _, row := range rangedSituationalModifiers {
		if n == row.situation {
			return row.grade, nil
		}
	}
	return rules.ParseDifficulty(name)
}

// damageModifierExpr returns the Damage Modifier expression to roll,
// stepping it up one row for a charge (+1 step biped, +2 quadruped;
// riders may substitute the mount's — that choice comes in as the
// character's own modifier).
// [AMBIGUOUS: the table's "Continue Progression" row beyond STR+SIZ 120
// prints no terms (digest §9.1); a step that would leave the printed rows
// keeps the original expression and records the limit in Facts.]
func damageModifierExpr(c *char.Character, p AttackParams) (string, error) {
	d, err := char.Derive(c)
	if err != nil {
		return "", fmt.Errorf("combat: derive %s: %w", idOf(c), err)
	}
	expr := d.DamageModifier
	if !p.Charging {
		return expr, nil
	}
	steps := 1
	if p.Quadruped {
		steps = 2
	}
	for i := 0; i < steps; i++ {
		next, ok := stepUpDamageModifier(expr)
		if !ok {
			return expr, nil // beyond the printed table: keep the original
		}
		expr = next
	}
	return expr, nil
}

// stepUpDamageModifier climbs one row of the Damage Modifier table
// (tables.md §1.2), in order from −1d8 to +2d10+1d2.
func stepUpDamageModifier(expr string) (string, bool) {
	for i, e := range damageModifierLadder {
		if e == expr {
			if i+1 >= len(damageModifierLadder) {
				return "", false
			}
			return damageModifierLadder[i+1], true
		}
	}
	return "", false
}

// damageModifierLadder is the Damage Modifier table (tables.md §1.2) in
// ascending step order — used for charge steps.
var damageModifierLadder = []string{
	"-1d8", "-1d6", "-1d4", "-1d2", "0", "1d2", "1d4", "1d6", "1d8",
	"1d10", "1d12", "2d6", "1d8+1d6", "2d8", "1d10+1d8", "2d10", "2d10+1d2",
}

// effectiveAttackSize computes the attacker's Size for parry comparison:
// melee uses Size (+1 step for a charge); ranged uses Force, stepped down
// one at Long Range and +1 step for a charge's momentum where Force
// applies (tables.md §4.9, §4.13).
func effectiveAttackSize(w weaponInfo, p AttackParams) string {
	s := w.size
	if w.ranged {
		s = w.force
	}
	rank, ok := sizeRank(s)
	if !ok {
		return "" // no printed Size/Force: nothing to compare
	}
	if w.ranged && p.Distance > 0 {
		if band, _ := rangeBand(w, p.Distance); band == "long" {
			rank--
		}
	}
	if p.Charging && !w.ranged {
		rank++ // weapon Size +1 step while charging
	}
	if rank < 1 {
		rank = 1
	}
	if rank > 5 {
		rank = 5
	}
	return sizeName(rank)
}

// sizeRank maps a Size/Force letter onto its step (S < M < L < H < E).
func sizeRank(s string) (int, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "S":
		return 1, true
	case "M":
		return 2, true
	case "L":
		return 3, true
	case "H":
		return 4, true
	case "E":
		return 5, true
	}
	return 0, false
}

// sizeName is the inverse of sizeRank.
func sizeName(rank int) string {
	switch rank {
	case 1:
		return "S"
	case 2:
		return "M"
	case 3:
		return "L"
	case 4:
		return "H"
	case 5:
		return "E"
	}
	return ""
}

// parryReduction returns how much damage a successful parry deflects
// (tables.md §4.5): equal or greater Size deflects all, one Size less
// deflects half (rounded up per Introduction › Rounding), two or more less
// deflects none. A barehanded or sizeless parry deflects nothing.
func parryReduction(dmg int, attackSize, parrySize string) int {
	if dmg <= 0 {
		return 0
	}
	a, aok := sizeRank(attackSize)
	d, dok := sizeRank(parrySize)
	if !aok || !dok {
		return 0
	}
	switch {
	case d >= a:
		return dmg
	case d == a-1:
		return dmg - ceilHalf(dmg)
	default:
		return 0
	}
}

// ceilHalf rounds a positive half up, per the Introduction's rounding rule
// ("whenever a division creates a fraction, round up").
func ceilHalf(n int) int {
	return (n + 1) / 2
}

// rollLocation resolves a 1d20 hit-location roll through the defender's
// catalog. Overlapping ranges — the shark's 17-18 appears in both Left Fin
// (17-18) and Head (17-20) — resolve to the first matching entry in catalog
// order, which is deterministic for every catalog (digest §9.10; char.Catalog
// returns the table as printed).
func rollLocation(def *char.Character, roll int) (string, error) {
	catalogName := def.HitLocationCatalog
	if catalogName == "" {
		catalogName = char.DefaultCatalog
	}
	defs, err := char.Catalog(catalogName)
	if err != nil {
		return "", fmt.Errorf("combat: hit location: %w", err)
	}
	for _, d := range defs {
		if roll >= d.RollRange[0] && roll <= d.RollRange[1] {
			return d.Name, nil
		}
	}
	return "", fmt.Errorf("combat: hit location roll %d matches no entry of catalog %q", roll, catalogName)
}

// armorComponents lists every protection element covering a hit location:
// the catalog's natural armour first, then each equipped item contributing
// AP (the "*" key covers every location). This mirrors char.ArmorAt but
// keeps the parts separate so Bypass Armor and Sunder can act on them.
func armorComponents(def *char.Character, location string) ([]ArmorComponent, error) {
	d, err := char.Derive(def)
	if err != nil {
		return nil, fmt.Errorf("combat: derive %s: %w", idOf(def), err)
	}
	var comps []ArmorComponent
	// Natural armour: the catalog's AP, reduced by a Sunder record when
	// one is present (CondNaturalArmor carries the remaining value).
	naturalAP := -1
	for _, loc := range d.Locations {
		if loc.Name == location {
			naturalAP = loc.AP
			break
		}
	}
	if naturalAP >= 0 {
		if cond := findCondition(def, CondNaturalArmor); cond != nil && cond.Location == location {
			if v, ok := parseArmorRemainder(cond.Notes); ok {
				naturalAP = v
			}
		}
		if naturalAP > 0 {
			comps = append(comps, ArmorComponent{Name: "natural", Label: ("natural armour (" + location + ")"), AP: naturalAP})
		}
	}
	found := false
	for _, loc := range d.Locations {
		if loc.Name == location {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("combat: %w: %q", char.ErrInvalidLocation, location)
	}
	for _, it := range def.Inventory {
		if it == nil || !it.Equipped || it.Armor == nil {
			continue
		}
		ap, ok := it.Armor.Points[location]
		if !ok {
			ap, ok = it.Armor.Points["*"]
		}
		if ok && ap > 0 {
			comps = append(comps, ArmorComponent{Name: it.ID, Label: it.Name, AP: ap})
		}
	}
	return comps, nil
}

// parseArmorRemainder reads a CondNaturalArmor "ap: N" note.
func parseArmorRemainder(notes string) (int, bool) {
	rest, ok := strings.CutPrefix(notes, "ap: ")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// rollConsequences resolves the Endurance-opposed consequences of a Serious
// or Major wound (tables.md §3.9): a rolled 1d3-round attack ban has
// already been applied for Serious wounds; here the victim rolls Endurance
// against the attacker's original roll. On failure a Serious limb goes
// useless (leg: prone), a Serious torso/head victim sleeps for minutes
// equal to the damage, a Major limb victim is knocked unconscious, and a
// Major torso/head victim dies instantly — survivors instead start the
// 2 × Healing Rounds treatment timer.
func rollConsequences(r *dice.Rand, res *AttackResult, def *char.Character, sev char.Severity, location string, dmg int) ([]ConsequenceRoll, error) {
	if sev != char.SeveritySerious && sev != char.SeverityMajor {
		return nil, nil
	}
	end, ok := def.Skill("Endurance")
	if !ok {
		return nil, fmt.Errorf("combat: wound consequence with %s: %w: Endurance", idOf(def), char.ErrUnknownSkill)
	}
	eqEnd, eqAtk := rules.EqualiseSkillsOver100(end.Value, res.AttackSkill)
	endRoll, err := r.Roll("1d100")
	if err != nil {
		return nil, fmt.Errorf("combat: endurance roll: %w", err)
	}
	endGrade, err := rules.Classify(endRoll.Total, eqEnd)
	if err != nil {
		return nil, fmt.Errorf("combat: classify endurance: %w", err)
	}
	endOut := rules.Outcome{Roll: endRoll.Total, Skill: eqEnd, Grade: endGrade}
	atkOut := res.AttackOutcome
	if res.AttackOutcome.Roll != 0 {
		// An Automatic attack rolled no die (Roll 0): its recorded
		// Success stands as the resistance target; every rolled attack is
		// re-equalised and reclassified against Endurance instead.
		atkGrade, err := rules.Classify(res.AttackOutcome.Roll, eqAtk)
		if err != nil {
			return nil, fmt.Errorf("combat: reclassify attack: %w", err)
		}
		atkOut = rules.Outcome{Roll: res.AttackOutcome.Roll, Skill: eqAtk, Grade: atkGrade}
	}

	cons := ConsequenceRoll{
		Location:         location,
		Severity:         sev,
		EnduranceRoll:    endRoll,
		EnduranceOutcome: endOut,
		AttackOutcome:    atkOut,
	}
	if res.ForcedFailure {
		// Force Failure: "the opponent auto-fails the resistance".
		cons.Forced = true
		cons.Resisted = false
		cons.Opposed = rules.OpposedResult{A: endOut, B: atkOut, Winner: 2}
	} else {
		cons.Opposed = rules.Opposed(endOut, atkOut)
		cons.Resisted = cons.Opposed.Winner == 1
	}

	class := locationClass(location)
	switch sev {
	case char.SeveritySerious:
		if class == locLimb || class == locWholeBody {
			if !cons.Resisted {
				cond := char.Condition{Name: CondUselessLimb, Location: location, Notes: "until the location returns to positive HP"}
				def.AddCondition(cond)
				if isLegLocation(location) {
					def.AddCondition(char.Condition{Name: char.CondProne})
					cons.Effect = "the limb fails: the victim drops prone and the limb is useless until it heals."
				} else {
					cons.Effect = "the limb fails: it is useless until it heals (a held object drops unless strapped)."
				}
			}
		} else if !cons.Resisted {
			def.AddCondition(char.Condition{Name: char.CondUnconscious, Notes: fmt.Sprintf("%d minutes", dmg)})
			cons.Effect = fmt.Sprintf("unconscious for %d minutes (First Aid or Healing can rouse).", dmg)
		}
	case char.SeverityMajor:
		switch {
		case class == locLimb || class == locWholeBody:
			if !cons.Resisted {
				def.AddCondition(char.Condition{Name: char.CondUnconscious, Notes: "Major Wound"})
				cons.Effect = "unconscious from the Major Wound."
			} else {
				d, err := char.Derive(def)
				if err != nil {
					return nil, fmt.Errorf("combat: derive %s: %w", idOf(def), err)
				}
				cons.Effect = fmt.Sprintf("the maimed limb kills if untreated after %d minutes (= 5 × Healing Rate).", 5*d.HealingRate)
			}
		case !cons.Resisted:
			def.Dead = true
			cons.Effect = "instant death."
		default:
			d, err := char.Derive(def)
			if err != nil {
				return nil, fmt.Errorf("combat: derive %s: %w", idOf(def), err)
			}
			rounds := 2 * d.HealingRate
			def.AddCondition(char.Condition{Name: CondUntreatedMajorWound, Rounds: rounds})
			cons.Effect = fmt.Sprintf("survives, but dies unless treated within %d Combat Rounds (= 2 × Healing Rate).", rounds)
		}
	}
	return []ConsequenceRoll{cons}, nil
}

// consequenceText renders one consequence for the audit facts.
func consequenceText(c ConsequenceRoll) string {
	if c.Resisted {
		return "resisted."
	}
	if c.Effect == "" {
		return "failed."
	}
	return "failed: " + c.Effect
}

// blindedGrade returns the difficulty grade carried by the Blinded
// condition (the Blind Opponent effect records "hard" or "formidable");
// a condition without a recorded grade falls back to the Close Combat
// table's "Blinded ... Herculean".
func blindedGrade(c *char.Character) rules.DifficultyGrade {
	cond := findCondition(c, CondBlinded)
	if cond != nil {
		if g, ok := parseConditionGrade(cond.Notes); ok {
			return g
		}
	}
	return rules.DifficultyHerculean
}

// parseConditionGrade extracts a "grade=<name>" pair from condition notes.
func parseConditionGrade(notes string) (rules.DifficultyGrade, bool) {
	idx := strings.Index(notes, "grade=")
	if idx < 0 {
		return rules.DifficultyStandard, false
	}
	rest := notes[idx+len("grade="):]
	if end := strings.IndexAny(rest, ";, "); end >= 0 {
		rest = rest[:end]
	}
	g, err := rules.ParseDifficulty(rest)
	if err != nil {
		return rules.DifficultyStandard, false
	}
	return g, true
}

// malfunctionFor returns the attacker's Weapon Malfunction condition for
// this weapon ("" notes blocks any weapon), or nil.
func malfunctionFor(c *char.Character, weapon string) *char.Condition {
	for i := range c.Conditions {
		cond := &c.Conditions[i]
		if cond.Name != CondMalfunction {
			continue
		}
		if cond.Notes == "" || cond.Notes == "weapon="+weapon {
			return cond
		}
	}
	return nil
}

// impaledHolds reports whether an Impaled condition pins this weapon in a
// wound ("weapon=<name>; ..." notes), making it unusable for parrying.
func impaledHolds(c *char.Character, weapon string) bool {
	for _, cond := range c.Conditions {
		if cond.Name == CondImpaled && strings.HasPrefix(cond.Notes, "weapon="+weapon+";") {
			return true
		}
	}
	return false
}

// removeConditionAt drops a named condition scoped to one hit location.
func removeConditionAt(c *char.Character, name, location string) {
	kept := c.Conditions[:0]
	for _, cond := range c.Conditions {
		if cond.Name == name && cond.Location == location {
			continue
		}
		kept = append(kept, cond)
	}
	c.Conditions = kept
}

// harder reports whether candidate is harder than current (grades never
// stack — the hardest applies; DifficultyGrade orders easiest to hardest).
func harder(candidate, current rules.DifficultyGrade) bool {
	return candidate > current
}

// ceilInt rounds a fraction up (Introduction › Rounding), used by effect
// maths that divides damage.
func ceilInt(n, d int) int {
	if d <= 0 {
		return 0
	}
	return int(math.Ceil(float64(n) / float64(d)))
}

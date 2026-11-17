package combat

import (
	"errors"
	"fmt"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
	"github.com/maxwelljens/mythras-ultra/internal/engine/rules"
)

// Sentinels returned by the Special Effects operations. Callers branch on
// them with errors.Is to turn a refusal into a structured IMPOSSIBLE result.
var (
	// ErrUnknownEffect reports a name the Special Effects Table does not
	// print (data_effects.go).
	ErrUnknownEffect = errors.New("combat: unknown special effect")
	// ErrEffectUnavailable reports a printed effect that this exchange
	// does not offer: wrong side of the Differential, a Specific Roll or
	// Special Weapon Type gate not met, no Special Effects left to spend,
	// or a non-stackable repeat.
	ErrEffectUnavailable = errors.New("combat: special effect not available")
	// ErrInvalidChoice reports an EffectOptions value the chosen effect
	// needs but does not accept (unknown hit location, missing bystander,
	// missing Prepare Counter nomination, non-adjoining Marksman shift).
	ErrInvalidChoice = errors.New("combat: invalid effect choice")
)

// The two sides of the Special Effects Table, reported in
// EffectResult.Side: an Offensive row is picked by the winner of the
// Differential Roll as attacker, a Defensive row as defender.
const (
	SideOffensive = "offensive"
	SideDefensive = "defensive"
)

// NamedRoll pairs a dice.Roll with the name of what was rolled, so an
// EffectResult audit reads ("endurance", 37), ("impale damage", 11), ...
type NamedRoll struct {
	// Name identifies what was rolled ("endurance", "location", ...).
	Name string `json:"name"`
	// Roll is the audit record of the dice.
	Roll dice.Roll `json:"roll"`
}

// EffectOptions carries the choices individual Special Effects demand.
// Every field is optional; an effect that needs one and does not get it
// errors wrapping [ErrInvalidChoice].
type EffectOptions struct {
	// Location is the hit location Choose Location selects, the adjoining
	// location Marksman shifts to, and the location a defensive Entangle
	// immobilises.
	Location string `json:"location,omitempty"`
	// Target is the second victim of Overpenetration or the bystander a
	// fumbled blow is deflected onto with Select Target; both require an
	// encounter participant other than the two combatants.
	Target *char.Character `json:"-"`
	// ArmorComponent names (by ID or label, case-insensitive) the armour
	// Bypass Armor skips or Sunder hammers; "" picks the first worn item,
	// falling back to natural armour.
	ArmorComponent string `json:"armorComponent,omitempty"`
	// CounterAgainst and CounterSubstitute are the Prepare Counter
	// nomination: the effect held ready in secret, and the effect that
	// replaces it when it fires ("" means the counter only neutralises).
	CounterAgainst    string `json:"counterAgainst,omitempty"`
	CounterSubstitute string `json:"counterSubstitute,omitempty"`
	// Scar is the description recorded in the Scar Foe condition; "" uses
	// a stock line.
	Scar string `json:"scar,omitempty"`
	// BlindGrade constrains Blind Opponent's duration grade to the two the
	// rules print: Formidable selects the harsher, anything else Hard.
	BlindGrade rules.DifficultyGrade `json:"blindGrade,omitempty"`
	// ResistSkill names the skill Trip Opponent's target resists with
	// ("Brawn", "Evade" — the default, or "Acrobatics").
	ResistSkill string `json:"resistSkill,omitempty"`
	// Quadruped marks a Trip Opponent target with even more legs: they use
	// Athletics and treat the roll one Difficulty Grade easier.
	Quadruped bool `json:"quadruped,omitempty"`
}

// EffectResult is the complete audit of one applied Special Effect: what
// was picked, which side picked it, every roll made while applying it, the
// damage and distance it moved, conditions it added, and present-tense
// Facts for the GM. Adjudicated reports an effect the engine has no
// mechanics for (no cover model, no engagement model): the Facts are
// narration hooks and no game state changed.
type EffectResult struct {
	// Round, Effect, Side, Actor and Target locate the pick: the round,
	// the printed effect name, SideOffensive or SideDefensive, the
	// character who won and applied it, and its recipient.
	Round  int    `json:"round"`
	Effect string `json:"effect"`
	Side   string `json:"side"`
	Actor  string `json:"actor"`
	Target string `json:"target"`
	// Adjudicated reports that the effect produced narration only.
	Adjudicated bool `json:"adjudicated,omitempty"`
	// Counter reports that a secret Prepare Counter intercepted the effect.
	Counter bool `json:"counter,omitempty"`
	// Location is the hit location in effect after the pick (Choose
	// Location, Marksman and Select Target move it).
	Location string `json:"location,omitempty"`
	// DamageApplied is the damage standing on the recipient after the
	// pick; Delta is the net damage this pick added (0 when none).
	DamageApplied int `json:"damageApplied,omitempty"`
	Delta         int `json:"delta,omitempty"`
	// Distance is metres knocked back (Bash) or flung (Disarm Opponent).
	Distance int `json:"distance,omitempty"`
	// Severity is the recipient's wound level after the pick.
	Severity char.Severity `json:"severity,omitempty"`
	// Rolls carries every dice roll the pick made, in order.
	Rolls []NamedRoll `json:"rolls,omitempty"`
	// Conditions lists the conditions the pick added (names only).
	Conditions []string `json:"conditions,omitempty"`
	// Facts are human-readable, present-tense audit lines for the GM.
	Facts []string `json:"facts,omitempty"`
}

// ApplySpecialEffect applies one Special Effects Table row to the exchange
// recorded in res: it validates the pick against the effect's side, the
// Specific Roll and Special Weapon Type gates and the number of effects
// won (res.SpecialEffects), resolves a secret Prepare Counter that
// intercepts it, then runs the row's mechanics deterministically — every
// roll lands in EffectResult.Rolls and every state change is announced in
// EffectResult.Facts.
//
// att and def must be the exchange's attacker and defender (as recorded in
// res); on the offensive side att acts and def receives, on the defensive
// side the roles swap. Effects whose rules are pure narration (Circumvent
// Cover, Duck Back, Withdraw) return Adjudicated: true without inventing
// mechanics.
//
// The rules require effects to be chosen before damage and Hit Location
// are rolled (L1647-1649); ResolveAttack is atomic, so effects adjust the
// already-rolled outcome afterwards and record every change in
// AttackResult.Adjustments. Errors wrap [ErrUnknownEffect],
// [ErrEffectUnavailable], [ErrInvalidChoice], [ErrNotRunning], or whatever
// the dice, catalog and wound operations return.
func ApplySpecialEffect(r *dice.Rand, e *Encounter, effect string, att, def *char.Character, res *AttackResult, opts EffectOptions) (EffectResult, error) {
	if r == nil {
		return EffectResult{}, errors.New("combat: nil rand")
	}
	fx, ok := EffectByName(effect)
	if !ok {
		return EffectResult{}, fmt.Errorf("%w: %q", ErrUnknownEffect, effect)
	}
	if err := e.running(); err != nil {
		return EffectResult{}, err
	}
	if res == nil {
		return EffectResult{}, errors.New("combat: special effects need the originating attack result")
	}
	if att == nil || def == nil {
		return EffectResult{}, errors.New("combat: nil combatant")
	}
	if res.Attacker != idOf(att) || res.Defender != idOf(def) {
		return EffectResult{}, fmt.Errorf("combat: %q belongs to the exchange %s vs %s, not %s vs %s",
			fx.Name, res.Attacker, res.Defender, idOf(att), idOf(def))
	}
	firearmFumble := res.AttackOutcome.Grade == rules.GradeFumble && res.WeaponFacts.Firearm
	side, okSide := effectSide(fx, res, firearmFumble)
	if !okSide {
		return EffectResult{}, fmt.Errorf("%w: %q may not be applied by that side in this exchange (winner %q)",
			ErrEffectUnavailable, fx.Name, res.Winner)
	}
	if !containsString(res.AvailableSpecialEffects, fx.Name) {
		return EffectResult{}, fmt.Errorf("%w: %q fails its gate in this exchange", ErrEffectUnavailable, fx.Name)
	}
	if len(res.AppliedEffects) >= res.SpecialEffects {
		return EffectResult{}, fmt.Errorf("%w: %s has %d Special Effect(s) left to spend in this exchange",
			ErrEffectUnavailable, idOf(att), res.SpecialEffects-len(res.AppliedEffects))
	}
	if !fx.Stackable && containsString(res.AppliedEffects, fx.Name) {
		return EffectResult{}, fmt.Errorf("%w: %q was already applied and is not stackable", ErrEffectUnavailable, fx.Name)
	}

	actor, victim := att, def
	if side == SideDefensive {
		actor, victim = def, att
	}
	out := EffectResult{
		Round:  e.Round,
		Effect: fx.Name,
		Side:   side,
		Actor:  idOf(actor),
		Target: idOf(victim),
	}

	// A secret Prepare Counter nominated against this effect fires here:
	// it neutralises the pick, or substitutes the counter-holder's own
	// effect, which succeeds automatically (Special Effect Descriptions ›
	// Prepare Counter).
	if idx, found := findCounter(e, victim, fx.Name); found {
		ctr := e.Counters[idx]
		var sub SpecialEffect
		if ctr.Substitute != "" {
			var ok bool
			sub, ok = EffectByName(ctr.Substitute)
			if !ok {
				return EffectResult{}, fmt.Errorf("%w: prepare counter substitute %q", ErrUnknownEffect, ctr.Substitute)
			}
		}
		res.AppliedEffects = append(res.AppliedEffects, fx.Name)
		e.Counters = append(e.Counters[:idx], e.Counters[idx+1:]...)
		out.Counter = true
		if ctr.Substitute == "" {
			out.Facts = append(out.Facts,
				fmt.Sprintf("%s's prepared counter fires: %s is neutralised and nothing happens.", out.Target, fx.Name))
			return out, nil
		}
		subOut, err := dispatch(r, e, res, sub, flipSide(side), victim, actor, opts)
		if err != nil {
			return EffectResult{}, fmt.Errorf("combat: prepare counter substitute %q: %w", sub.Name, err)
		}
		out.Facts = append(out.Facts,
			fmt.Sprintf("%s's prepared counter fires: %s is substituted with %s, which succeeds automatically.", out.Target, fx.Name, sub.Name))
		out.Facts = append(out.Facts, subOut.Facts...)
		out.Rolls = append(out.Rolls, subOut.Rolls...)
		out.Conditions = append(out.Conditions, subOut.Conditions...)
		out.Location, out.Delta, out.Distance = subOut.Location, subOut.Delta, subOut.Distance
		out.DamageApplied, out.Severity = subOut.DamageApplied, subOut.Severity
		out.Adjudicated = subOut.Adjudicated
		return out, nil
	}

	// The pick is spent whether or not the effect's own preconditions
	// hold — in the real sequence it is chosen before damage is known.
	res.AppliedEffects = append(res.AppliedEffects, fx.Name)
	final, err := dispatch(r, e, res, fx, side, actor, victim, opts)
	if err != nil {
		return EffectResult{}, err
	}
	return final, nil
}

// dispatch routes one validated Special Effect to its mechanics. The
// switch is the Special Effects Table itself (data_effects.go): every row
// either runs its printed mechanic or returns narration-only facts.
func dispatch(r *dice.Rand, e *Encounter, res *AttackResult, fx SpecialEffect, side string, actor, victim *char.Character, opts EffectOptions) (EffectResult, error) {
	out := EffectResult{
		Round:  e.Round,
		Effect: fx.Name,
		Side:   side,
		Actor:  idOf(actor),
		Target: idOf(victim),
	}
	var err error
	switch fx.Name {
	case "Accidental Injury":
		err = fxAccidental(r, res, &out, victim)
	case "Arise":
		actor.RemoveCondition(char.CondProne)
		out.Facts = append(out.Facts, out.Actor+" rolls back to their feet.")
	case "Bash":
		fxBash(res, &out, actor, victim)
	case "Bleed":
		err = fxBleed(r, res, &out, victim)
	case "Blind Opponent":
		err = fxBlind(r, res, &out, victim, opts)
	case "Bypass Armor":
		err = fxBypassArmor(r, res, &out, victim, opts)
	case "Choose Location":
		err = fxRelocate(r, res, victim, opts, "Choose Location")
	case "Circumvent Cover":
		out.Adjudicated = true
		out.Facts = append(out.Facts,
			(out.Actor + " circumvents the cover with high-tech fire; which protection is defeated is GM-adjudicated (the engine models no cover)."))
	case "Circumvent Parry":
		err = fxCircumventParry(r, res, &out, victim)
	case "Damage Weapon":
		err = fxDamageWeapon(r, e, res, &out, side, actor, victim)
	case "Disarm Opponent":
		err = fxDisarm(r, res, &out, actor, victim)
	case "Drop Foe":
		err = fxDropFoe(r, res, &out, victim)
	case "Duck Back":
		out.Adjudicated = true
		out.Facts = append(out.Facts,
			(out.Actor + " ducks straight back into cover, skipping the Take Cover action; cover placement is GM-adjudicated."))
	case "Enhance Parry":
		// The parry belongs to the applier: on the defensive side that is
		// the defender, whose exchanged wound is the one reversed.
		err = fxEnhanceParry(res, &out, actor)
	case "Entangle":
		fxEntangle(res, &out, side, victim, opts)
	case "Force Failure":
		res.ForcedFailure = true
		out.Facts = append(out.Facts,
			(out.Actor + " forces the failure: every resistance roll in this exchange auto-fails."))
	case "Grip":
		victim.AddCondition(char.Condition{Name: CondGripped, Notes: "escape: opposed Brawn or Unarmed against the gripper's choice"})
		out.Conditions = append(out.Conditions, CondGripped)
		out.Facts = append(out.Facts, fmt.Sprintf("%s grips %s: they cannot disengage until they break free.", out.Actor, out.Target))
	case "Impale":
		err = fxImpale(r, res, &out, actor, victim)
	case "Marksman":
		err = fxMarksman(r, res, &out, victim, opts)
	case "Maximize Damage":
		err = fxMaximise(r, res, &out, victim)
	case "Overpenetration":
		err = fxOverpenetration(r, e, res, &out, opts)
	case "Pin Down":
		err = fxPinDown(r, res, &out, victim)
	case "Prepare Counter":
		err = fxPrepareCounter(e, &out, actor, opts)
	case "Rapid Reload":
		rapidReload(actor)
		out.Conditions = append(out.Conditions, CondRapidReload)
		out.Facts = append(out.Facts, (out.Actor + "'s next reload takes one Turn less (stacking)."))
	case "Scar Foe":
		scar := opts.Scar
		if scar == "" {
			scar = "a lasting scar"
		}
		victim.AddCondition(char.Condition{Name: CondScarred, Notes: scar})
		out.Conditions = append(out.Conditions, CondScarred)
		out.Facts = append(out.Facts, fmt.Sprintf("%s leaves %s with %s.", out.Actor, out.Target, scar))
	case "Select Target":
		err = fxSelectTarget(r, e, res, &out, actor, opts)
	case "Slip Free":
		// "the defender can automatically escape being Entangled, Gripped,
		// or Pinned" — the holder of the effect is the one who slips free.
		for _, name := range []string{CondEntangled, CondGripped, CondPinned} {
			actor.RemoveCondition(name)
		}
		out.Facts = append(out.Facts, (out.Actor + " slips free of every hold."))
	case "Stun Location":
		err = fxStun(r, res, &out, victim)
	case "Sunder":
		err = fxSunder(res, &out, victim, opts)
	case "Trip Opponent":
		err = fxTrip(r, res, &out, victim, opts)
	case "Weapon Malfunction":
		if cond := malfunctionFor(victim, res.Weapon); cond == nil {
			victim.AddCondition(char.Condition{Name: CondMalfunction, Notes: "weapon=" + res.Weapon})
		}
		out.Conditions = append(out.Conditions, CondMalfunction)
		out.Facts = append(out.Facts, fmt.Sprintf("%s's %s malfunctions: it is useless until field-stripped and cleaned.", out.Target, res.Weapon))
	case "Withdraw":
		out.Adjudicated = true
		out.Facts = append(out.Facts, (out.Actor + " withdraws out of reach, breaking engagement automatically; positioning is GM-adjudicated."))
	default:
		// Unreachable: EffectByName only returns printed rows and every
		// printed row is switched above.
		return EffectResult{}, fmt.Errorf("%w: no mechanics wired for %q", ErrUnknownEffect, fx.Name)
	}
	if err != nil {
		return EffectResult{}, err
	}
	// Fill the exchange's standing state for handlers that left the
	// defaults alone (the pick changed no wound).
	if out.DamageApplied == 0 {
		out.DamageApplied = res.DamageApplied
	}
	if out.Severity == char.SeverityNone {
		out.Severity = res.Severity
	}
	if out.Location == "" {
		out.Location = res.Location
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// availability
// ---------------------------------------------------------------------------

// computeAvailable returns the Special Effects Table rows this exchange
// offers the winner, in printed order — the list AttackResult carries for
// the GM's pick menu. firearmFumble grants the intended target the
// fumble-specific rows even when the Differential Roll itself grants no
// benefit: gunfire has no opposing roll, so there is nothing to win
// against (Combat › Fumbling with Firearms L1923-1925).
func computeAvailable(res *AttackResult, stationaryOrUnaware bool) []string {
	if res.AttackOutcome.Grade == rules.GradeFumble && res.WeaponFacts.Firearm && res.SpecialEffects == 0 {
		res.SpecialEffects = 1
		res.Facts = append(res.Facts,
			"Fumbling with Firearms: with no opposing roll the intended target gains one pick from the fumble-specific Special Effects.")
	}
	var out []string
	for _, fx := range specialEffects {
		side, ok := effectSide(fx, res, res.AttackOutcome.Grade == rules.GradeFumble && res.WeaponFacts.Firearm)
		if !ok {
			continue
		}
		if !effectGates(fx, res, side, stationaryOrUnaware) {
			continue
		}
		out = append(out, fx.Name)
	}
	return out
}

// effectSide resolves which side of the Differential Roll may pick fx:
// the winner picks rows of their own column, and the intended target of a
// firearm fumble may always pick the "Attacker Fumbles" rows.
func effectSide(fx SpecialEffect, res *AttackResult, firearmFumble bool) (string, bool) {
	if fx.Offensive && res.Winner == WinnerAttacker {
		return SideOffensive, true
	}
	if fx.Defensive && res.Winner == WinnerDefender {
		return SideDefensive, true
	}
	if fx.Defensive && firearmFumble && fx.SpecificRoll == "Attacker Fumbles" {
		return SideDefensive, true
	}
	return "", false
}

// effectGates enforces the printed Specific Roll and Special Weapon Type
// columns plus the two gates only the effect descriptions state (Enhance
// Parry needs a real parry, Damage Weapon against a firearm needs a
// Critical). w is the applying side's weapon: the attacker's for
// offensive rows, the defender's parrying weapon for defensive rows.
func effectGates(fx SpecialEffect, res *AttackResult, side string, stationaryOrUnaware bool) bool {
	switch fx.SpecificRoll {
	case "Attacker Criticals":
		if res.AttackOutcome.Grade != rules.GradeCritical {
			return false
		}
	case "Defender Criticals":
		if res.ReactionOutcome.Grade != rules.GradeCritical {
			return false
		}
	case "Attacker Fumbles":
		if res.AttackOutcome.Grade != rules.GradeFumble {
			return false
		}
	case "Opponent Fumbles":
		// The opponent of the applying side is the one who must fumble.
		if side == SideOffensive {
			if res.ReactionOutcome.Grade != rules.GradeFumble {
				return false
			}
		} else if res.AttackOutcome.Grade != rules.GradeFumble {
			return false
		}
	case "Firearms: Attacker Criticals":
		// The column means "firearms restrict this to a Critical"; other
		// weapons carry no Specific Roll gate here.
		if res.WeaponFacts.Firearm && res.AttackOutcome.Grade != rules.GradeCritical {
			return false
		}
	case "See Description":
		if !chooseLocationGate(res, stationaryOrUnaware) {
			return false
		}
	}
	// Special Weapon Type, judged on the applying side's weapon — except
	// the "Attacker Fumbles" rows, which are about the shooter's own
	// weapon (Weapon Malfunction jams *their* firearm even when they
	// could not be parried and the defender's weapon facts are empty).
	w := res.WeaponFacts
	if side == SideDefensive && fx.SpecificRoll != "Attacker Fumbles" {
		w = res.ReactionWeapon
	}
	switch fx.WeaponType {
	case "Shields or Bludgeoning":
		if !w.Shield && !isBludgeoning(w) {
			return false
		}
	case "Cutting Weapons":
		if isBludgeoning(w) || isEntangling(w) {
			return false
		}
	case "High Tech Firearms Only":
		if !w.Firearm || !w.HighTech {
			return false
		}
	case "Entangling Weapons":
		if !isEntangling(w) {
			return false
		}
	case "Firearms Only":
		if !w.Firearm {
			return false
		}
	case "Unarmed":
		if !w.Unarmed {
			return false
		}
	case "Impaling Weapons":
		if !impaling(w) {
			return false
		}
	case "Ranged Weapons":
		if !w.Ranged {
			return false
		}
	case "Bludgeoning Weapons":
		if !isBludgeoning(w) {
			return false
		}
	case "Two Handed Weapons":
		if !strings.Contains(strings.ToLower(w.Notes), "two handed") {
			return false
		}
	}
	switch fx.Name {
	case "Enhance Parry":
		return res.Reaction == ReactionParry
	case "Damage Weapon":
		// Offensively against melee the row needs a parrying weapon to
		// strike; firearms aim at whatever the victim holds (there is no
		// parry against a bullet), and "for firearms ... this should
		// generally be reserved for Critical Successes only" (Special
		// Effect Descriptions).
		if side == SideOffensive && !w.Firearm &&
			(res.ParriedWith == "" || res.ReactionWeapon.Name == "") {
			return false
		}
		return !w.Firearm || res.AttackOutcome.Grade == rules.GradeCritical
	}
	return true
}

// chooseLocationGate implements Choose Location's printed gate: free in
// melee; ranged and firearm attacks only on a Critical, unless the target
// is within Close Range and stationary or unaware (Special Effect
// Descriptions › Choose Location).
func chooseLocationGate(res *AttackResult, stationaryOrUnaware bool) bool {
	if !res.Ranged {
		return true
	}
	if res.AttackOutcome.Grade == rules.GradeCritical {
		return true
	}
	return stationaryOrUnaware && res.RangeBand == "close"
}

// isBludgeoning reports whether the weapon records the bludgeoning damage
// type (the gate behind Bash and Stun Location, and the exclusion for
// cutting and impaling picks). The sample tables print no damage-type
// column, so an untyped weapon passes every damage-type gate and the GM
// may refuse the pick
// [AMBIGUOUS: Combat › Arms and Armor vs. Combat › Special Effects].
func isBludgeoning(w WeaponFacts) bool {
	return strings.EqualFold(strings.TrimSpace(w.DamageType), "bludgeoning")
}

// isEntangling reports whether the weapon's printed notes mark it as an
// entangling weapon ("Can entangle", "Entrapping, Thrown", "Entangling").
func isEntangling(w WeaponFacts) bool {
	n := strings.ToLower(w.Notes)
	return strings.Contains(n, "entangl") || strings.Contains(n, "entrap")
}

// impaling reports whether a weapon can pick Impale. Melee and thrown
// weapons lodge unless they are blunt, entangling, shields or unarmed;
// "the ammunition of most projectile-based firearms will be able to
// impale", and thrown weapons that print an Impale Size lodge (tables.md
// §5.4); bullets and flechettes inflict no secondary impaling effects, so
// firearms with no printed Impale Size only get the doubled damage roll.
// [AMBIGUOUS: the sample tables print no "impaling" column — Special
// Effect Descriptions › Impale.]
func impaling(w WeaponFacts) bool {
	switch {
	case w.Name == "" || w.Unarmed || w.Shield:
		return false
	case isBludgeoning(w), isEntangling(w):
		return false
	case w.Firearm:
		return true
	case w.Ranged:
		return w.ImpaleSize != ""
	default:
		return true
	}
}

// flipSide returns the opposite side (a Prepare Counter substitutes the
// counter-holder's own effect, picked from the other column).
func flipSide(side string) string {
	if side == SideOffensive {
		return SideDefensive
	}
	return SideOffensive
}

// findCounter returns the index of a Prepare Counter held by c against
// effect, and whether one exists.
func findCounter(e *Encounter, c *char.Character, effect string) (int, bool) {
	idx, err := e.indexOf(c)
	if err != nil {
		return 0, false
	}
	for i, ctr := range e.Counters {
		if ctr.Index == idx && strings.EqualFold(ctr.Against, effect) {
			return i, true
		}
	}
	return 0, false
}

// containsString reports whether haystack contains want (exact).
func containsString(haystack []string, want string) bool {
	for _, s := range haystack {
		if s == want {
			return true
		}
	}
	return false
}

// rapidReload adds one Rapid Reload stack (Rounds carries the stack count
// — tickExempt keeps NextRound from consuming it).
func rapidReload(c *char.Character) {
	if prev := findCondition(c, CondRapidReload); prev != nil {
		c.AddCondition(char.Condition{Name: CondRapidReload, Rounds: prev.Rounds + 1})
		return
	}
	c.AddCondition(char.Condition{Name: CondRapidReload, Rounds: 1})
}

// ---------------------------------------------------------------------------
// effect mechanics
// ---------------------------------------------------------------------------

// fxAccidental rolls the attacker's own weapon damage against them at a
// random Hit Location; an unarmed fumble tears something internal and
// ignores armour (Special Effect Descriptions › Accidental Injury).
// [AMBIGUOUS: whether the self-inflicted blow carries the attacker's
// Damage Modifier is never stated; the engine applies the damage
// application order in full (tables.md §3.9), so it does.]
func fxAccidental(r *dice.Rand, res *AttackResult, out *EffectResult, self *char.Character) error {
	w := res.WeaponFacts
	expr := w.Damage
	if expr == "" {
		expr = "1d3"
	}
	dmg, err := r.Roll(expr)
	if err != nil {
		return fmt.Errorf("combat: accidental damage %q: %w", expr, err)
	}
	out.Rolls = append(out.Rolls, NamedRoll{"damage", dmg})
	total := dmg.Total
	if w.DMAdds {
		d, err := char.Derive(self)
		if err != nil {
			return fmt.Errorf("combat: derive %s: %w", idOf(self), err)
		}
		if d.DamageModifier != "" && d.DamageModifier != "0" {
			dm, err := r.Roll(d.DamageModifier)
			if err != nil {
				return fmt.Errorf("combat: accidental damage modifier %q: %w", d.DamageModifier, err)
			}
			out.Rolls = append(out.Rolls, NamedRoll{"damage modifier", dm})
			total += dm.Total
		}
	}
	if total < 0 {
		total = 0
	}
	locRoll, err := r.Roll("1d20")
	if err != nil {
		return fmt.Errorf("combat: accidental location: %w", err)
	}
	out.Rolls = append(out.Rolls, NamedRoll{"location", locRoll})
	loc, err := rollLocation(self, locRoll.Total)
	if err != nil {
		return err
	}
	absorbed := 0
	if !w.Unarmed {
		comps, err := armorComponents(self, loc)
		if err != nil {
			return err
		}
		for _, c := range comps {
			absorbed += c.AP
		}
		if absorbed > total {
			absorbed = total
		}
	} else {
		out.Facts = append(out.Facts, "unarmed: the internal tear ignores armour.")
	}
	applied := total - absorbed
	if applied < 0 {
		applied = 0
	}
	out.Location = loc
	out.DamageApplied = applied
	out.Delta = applied
	out.Facts = append(out.Facts, fmt.Sprintf("%s's fumbled %s strikes themself: %d damage to the %s (%d absorbed by armour).",
		out.Target, w.Name, total, loc, absorbed))
	if applied == 0 {
		return nil
	}
	sev, fxList, err := self.ApplyWound(loc, applied)
	if err != nil {
		return fmt.Errorf("combat: accidental wound: %w", err)
	}
	out.Severity = sev
	out.Facts = append(out.Facts, fmt.Sprintf("%s takes %d damage (%s).", out.Target, applied, sev))
	_ = fxList
	// Endurance resists the wounding: the attacker's own fumbled roll is
	// the attack roll the consequence opposes (L1649).
	if sev == char.SeveritySerious || sev == char.SeverityMajor {
		cons, err := rollConsequences(r, res, self, sev, loc, applied)
		if err != nil {
			return err
		}
		for _, c := range cons {
			out.Facts = append(out.Facts, "Endurance "+c.EnduranceOutcome.String()+": "+consequenceText(c))
		}
	}
	return nil
}

// fxBash applies the printed Bash knockback: shields push one metre per 2
// points of damage rolled, bludgeons one metre per 3, only against
// creatures up to twice the attacker's SIZ (fractions round up —
// Introduction › Rounding). The obstacle clause needs terrain the engine
// does not model, so it is announced as a GM fact.
func fxBash(res *AttackResult, out *EffectResult, actor, victim *char.Character) {
	w := res.WeaponFacts
	// Brace: the recipient's SIZ counts as doubled vs Bash, which can put
	// them outside the "twice the attacker's SIZ" cap the effect works on.
	siz := victim.Chars.SIZ
	if victim.HasCondition(CondBraced) {
		siz *= 2
		out.Facts = append(out.Facts, fmt.Sprintf("%s braces: SIZ counts as %d vs the bash.", out.Target, siz))
	}
	if siz > 2*actor.Chars.SIZ {
		out.Facts = append(out.Facts, (out.Target + " is too large to bash (creatures up to twice the attacker's SIZ only)."))
		return
	}
	if res.DamagePre <= 0 {
		out.Facts = append(out.Facts, "no damage was rolled: the bash accomplishes nothing.")
		return
	}
	div := 3
	if w.Shield {
		div = 2
	}
	metres := ceilInt(res.DamagePre, div)
	out.Distance = metres
	out.Facts = append(out.Facts, fmt.Sprintf("%s bashes %s back %d metre(s) (%d pre-reduction damage ÷ %d).",
		out.Actor, out.Target, metres, res.DamagePre, div))
	out.Facts = append(out.Facts, "GM: driven into an obstacle, the recipient needs a Hard Athletics or Acrobatics roll or falls and trips.")
}

// fxBleed applies the Bleed effect: if the blow overcame armour and
// injured, the victim opposes Endurance against the original attack roll
// and, on failure, loses one Fatigue level at the start of every Combat
// Round until staunched (Special Effect Descriptions › Bleed; Blood Loss,
// Game System).
func fxBleed(r *dice.Rand, res *AttackResult, out *EffectResult, victim *char.Character) error {
	if res.DamageApplied <= 0 {
		out.Facts = append(out.Facts, "the blow did not overcome armour and injure: no bleed.")
		return nil
	}
	opp, err := opposeOriginal(r, res, victim, "Endurance", out, "endurance")
	if err != nil {
		return err
	}
	if opp.Winner == 1 {
		out.Facts = append(out.Facts, (out.Target + "'s Endurance holds: no bleeding."))
		return nil
	}
	if findCondition(victim, char.CondBleeding) == nil {
		victim.AddCondition(char.Condition{Name: char.CondBleeding, Notes: "loss: 0"})
		out.Conditions = append(out.Conditions, char.CondBleeding)
	}
	out.Facts = append(out.Facts,
		(out.Target + " fails the opposed Endurance and bleeds: 1 Fatigue level at the start of each Combat Round until First Aid staunches the wound."))
	return nil
}

// fxBlind applies Blind Opponent: the blinded attacker opposes their
// Evade (or weapon skill when fighting with a shield) against the
// defender's original parry roll; failure blinds them Hard or Formidable
// for 1d3 Turns [AMBIGUOUS: a Turn is an action slot — context implies
// Rounds, digest §9.5].
func fxBlind(r *dice.Rand, res *AttackResult, out *EffectResult, victim *char.Character, opts EffectOptions) error {
	skillName := "Evade"
	skill := 0
	if res.WeaponFacts.Shield {
		v, ref, _, err := combatSkill(victim, res.Weapon, false)
		if err != nil {
			return err
		}
		skill, skillName = v, ref
	} else {
		s, ok := victim.Skill("Evade")
		if !ok {
			return fmt.Errorf("combat: blind: %w: Evade", char.ErrUnknownSkill)
		}
		skill = s.Value
	}
	eqAtk, eqDef := rules.EqualiseSkillsOver100(skill, res.ReactionSkill)
	roll, err := r.Roll("1d100")
	if err != nil {
		return fmt.Errorf("combat: blind roll: %w", err)
	}
	out.Rolls = append(out.Rolls, NamedRoll{strings.ToLower(skillName), roll})
	g, err := rules.Classify(roll.Total, eqAtk)
	if err != nil {
		return fmt.Errorf("combat: classify blind: %w", err)
	}
	atkOut := rules.Outcome{Roll: roll.Total, Skill: eqAtk, Grade: g}
	defOut := res.ReactionOutcome
	if res.ReactionRoll.Total != 0 {
		dg, err := rules.Classify(res.ReactionRoll.Total, eqDef)
		if err != nil {
			return fmt.Errorf("combat: reclassify parry: %w", err)
		}
		defOut = rules.Outcome{Roll: res.ReactionRoll.Total, Skill: eqDef, Grade: dg}
	}
	opp := rules.Opposed(atkOut, defOut)
	if res.ForcedFailure {
		opp = rules.OpposedResult{A: atkOut, B: defOut, Winner: 2, Margin: int(defOut.Grade - atkOut.Grade)}
	}
	if opp.Winner == 1 {
		out.Facts = append(out.Facts, (out.Target + " keeps their vision: the blinding attempt fails."))
		return nil
	}
	grade := rules.DifficultyHard
	if opts.BlindGrade == rules.DifficultyFormidable {
		grade = rules.DifficultyFormidable
	}
	turns, err := r.Roll("1d3")
	if err != nil {
		return fmt.Errorf("combat: blind duration: %w", err)
	}
	out.Rolls = append(out.Rolls, NamedRoll{"duration", turns})
	victim.AddCondition(char.Condition{
		Name:   CondBlinded,
		Rounds: turns.Total,
		Notes:  "grade=" + grade.String(),
	})
	out.Conditions = append(out.Conditions, CondBlinded)
	out.Facts = append(out.Facts, fmt.Sprintf("%s is blinded (%s) for %d Rounds.", out.Target, grade, turns.Total))
	return nil
}

// fxBypassArmor removes one armour component from the damage calculation
// and re-applies the difference: the pick was made before damage was
// known, so the engine adjusts the wound it already recorded.
func fxBypassArmor(r *dice.Rand, res *AttackResult, out *EffectResult, victim *char.Character, opts EffectOptions) error {
	if res.Location == "" || res.DamagePre == 0 {
		out.Facts = append(out.Facts, "no blow landed: there is no armour to bypass.")
		return nil
	}
	comps, err := armorComponents(victim, res.Location)
	if err != nil {
		return err
	}
	pick, err := pickComponent(comps, opts.ArmorComponent)
	if err != nil {
		return err
	}
	res.BypassedArmor = append(res.BypassedArmor, pick.Label)
	afterParry := res.DamagePre - res.ParryReduction
	remaining := 0
	for _, c := range comps {
		bypassed := false
		for _, name := range res.BypassedArmor {
			if c.Label == name || c.Name == name {
				bypassed = true
				break
			}
		}
		if !bypassed {
			remaining += c.AP
		}
	}
	absorbed := remaining
	if afterParry < absorbed {
		absorbed = afterParry
	}
	newApplied := afterParry - absorbed
	delta := newApplied - res.DamageApplied
	res.ArmorAbsorbed = absorbed
	if delta <= 0 {
		out.Facts = append(out.Facts, fmt.Sprintf("%s bypasses %s; the remaining armour already absorbs nothing extra.", out.Actor, pick.Label))
		return nil
	}
	out.Delta = delta
	if err := addWoundDelta(r, res, victim, delta, "Bypass Armor"); err != nil {
		return err
	}
	out.Facts = append(out.Facts, fmt.Sprintf("%s bypasses %s: %d extra damage reaches the %s.", out.Actor, pick.Label, delta, res.Location))
	return nil
}

// pickComponent selects an armour component for Bypass Armor or Sunder: a
// named match, else the first worn item, else natural armour.
func pickComponent(comps []ArmorComponent, name string) (ArmorComponent, error) {
	if name != "" {
		for _, c := range comps {
			if strings.EqualFold(c.Name, name) || strings.EqualFold(c.Label, name) {
				return c, nil
			}
		}
		return ArmorComponent{}, fmt.Errorf("%w: no armour component %q at the struck location", ErrInvalidChoice, name)
	}
	for _, c := range comps {
		if c.Name != "natural" {
			return c, nil
		}
	}
	if len(comps) > 0 {
		return comps[0], nil
	}
	return ArmorComponent{}, fmt.Errorf("%w: the struck location carries no armour", ErrInvalidChoice)
}

// fxRelocate moves the exchange's wound to another hit location (Choose
// Location, and Marksman after its adjoining-step check): the blow is
// re-applied at the new location, location-scoped conditions travel with
// it, and a severity escalation rolls the new level's consequence.
// [AMBIGUOUS: consequences already rolled at the old location stay as
// recorded — they are the audit of rolls actually made.]
func fxRelocate(r *dice.Rand, res *AttackResult, victim *char.Character, opts EffectOptions, what string) error {
	if opts.Location == "" {
		return fmt.Errorf("%w: %s needs a location", ErrInvalidChoice, what)
	}
	return relocateWound(r, res, victim, opts.Location, what)
}

// fxMarksman shifts the hit location one step to an immediately adjoining
// area (Special Effect Descriptions › Marksman).
func fxMarksman(r *dice.Rand, res *AttackResult, out *EffectResult, victim *char.Character, opts EffectOptions) error {
	if opts.Location == "" {
		return fmt.Errorf("%w: Marksman needs the adjoining location to shift to", ErrInvalidChoice)
	}
	if res.Location == "" {
		out.Facts = append(out.Facts, "no hit location to shift: no blow landed.")
		return nil
	}
	ok, err := adjoining(victim, res.Location, opts.Location)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %q is not an area adjoining %q", ErrInvalidChoice, opts.Location, res.Location)
	}
	return relocateWound(r, res, victim, opts.Location, "Marksman")
}

// fxCircumventParry nullifies a successful parry and re-applies the
// damage it deflected (Special Effect Descriptions › Circumvent Parry).
func fxCircumventParry(r *dice.Rand, res *AttackResult, out *EffectResult, victim *char.Character) error {
	if res.Reaction != ReactionParry || !res.ParryApplied {
		out.Facts = append(out.Facts, "there is no successful parry to circumvent.")
		return nil
	}
	if res.Location == "" || res.DamagePre == 0 {
		res.ParryApplied, res.ParryReduction = false, 0
		out.Facts = append(out.Facts, "the parry is circumvented.")
		return nil
	}
	afterParry := res.DamagePre
	absorbed := afterParry
	if res.ArmorPoints < absorbed {
		absorbed = res.ArmorPoints
	}
	newApplied := afterParry - absorbed
	delta := newApplied - res.DamageApplied
	res.ParryApplied = false
	res.ParryReduction = 0
	res.ArmorAbsorbed = absorbed
	if delta > 0 {
		out.Delta = delta
		if err := addWoundDelta(r, res, victim, delta, "Circumvent Parry"); err != nil {
			return err
		}
	}
	out.Facts = append(out.Facts, fmt.Sprintf("%s circumvents the parry: %d damage comes through untouched.", out.Actor, res.DamagePre))
	return nil
}

// fxDamageWeapon applies a swing's damage roll against the struck weapon,
// which resists with its own Armor Points; at 0 Hit Points it breaks
// (Special Effect Descriptions › Damage Weapon). The struck weapon is the
// recipient's — their parrying weapon when the attacker picks the row, the
// attacking weapon on the defensive side. The exchange's own damage roll
// is reused when it happened; the defender's swing is rolled fresh.
func fxDamageWeapon(r *dice.Rand, e *Encounter, res *AttackResult, out *EffectResult, side string, actor, victim *char.Character) error {
	var swing, struck WeaponFacts
	if side == SideOffensive {
		swing, struck = res.WeaponFacts, res.ReactionWeapon
		if struck.Name == "" {
			// No parry to strike at: a firearm (or any blow met without a
			// weapon) aims at what the victim actually holds.
			if held := mainHandWeapon(victim); held != nil {
				if info, err := infoFromItem(held); err == nil {
					struck = factsOf(info)
				}
			}
		}
		if struck.Name == "" {
			out.Facts = append(out.Facts, "there is no weapon in the victim's hands to strike.")
			return nil
		}
	} else {
		swing, struck = res.ReactionWeapon, res.WeaponFacts
		if swing.Name == "" || struck.Name == "" {
			out.Facts = append(out.Facts, "there is no weapon left in the exchange to damage.")
			return nil
		}
	}
	if struck.HP <= 0 {
		out.Facts = append(out.Facts, ("the tables print no Hit Points for " + struck.Name + "; the damage it takes is GM-adjudicated."))
		return nil
	}
	// The attacker's successful strike already rolled its damage; any
	// other swing rolls weapon damage plus the swinger's Damage Modifier.
	if side == SideOffensive && res.DamageRoll.Total != 0 {
		total := res.DamageRoll.Total + res.DamageModifierRoll.Total
		return recordWeaponDamage(e, out, victim, struck, total)
	}
	expr := swing.Damage
	if expr == "" {
		expr = "1d3"
	}
	dmg, err := r.Roll(expr)
	if err != nil {
		return fmt.Errorf("combat: weapon damage %q: %w", expr, err)
	}
	out.Rolls = append(out.Rolls, NamedRoll{"weapon damage", dmg})
	total := dmg.Total
	if swing.DMAdds {
		d, err := char.Derive(actor)
		if err != nil {
			return fmt.Errorf("combat: derive %s: %w", idOf(actor), err)
		}
		if d.DamageModifier != "" && d.DamageModifier != "0" {
			dm, err := r.Roll(d.DamageModifier)
			if err != nil {
				return fmt.Errorf("combat: damage modifier %q: %w", d.DamageModifier, err)
			}
			out.Rolls = append(out.Rolls, NamedRoll{"damage modifier", dm})
			total += dm.Total
		}
	}
	return recordWeaponDamage(e, out, victim, struck, total)
}

// mainHandWeapon returns the character's equipped main-hand weapon or
// shield, or nil when they hold nothing.
func mainHandWeapon(c *char.Character) *char.Item {
	for _, it := range c.Inventory {
		if it != nil && it.Equipped && it.Slot == "main" && (it.Weapon != nil || it.Shield != nil) {
			return it
		}
	}
	return nil
}

// recordWeaponDamage subtracts the struck weapon's own Armor Points and
// records the residual in the encounter's weapon-HP ledger.
func recordWeaponDamage(e *Encounter, out *EffectResult, holder *char.Character, target WeaponFacts, total int) error {
	if total < 0 {
		total = 0
	}
	absorbed := target.AP
	if absorbed > total {
		absorbed = total
	}
	pierced := total - absorbed
	out.Facts = append(out.Facts, fmt.Sprintf("damage %d vs the %s's %d Armor Points: %d gets through.", total, target.Name, target.AP, pierced))
	if pierced == 0 {
		return nil
	}
	idx, err := e.indexOf(holder)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%d:%s", idx, weaponKey(holder, target.Name))
	cur, ok := e.WeaponHP[key]
	if !ok {
		cur = target.HP
	}
	if e.WeaponHP == nil {
		e.WeaponHP = make(map[string]int)
	}
	cur -= pierced
	e.WeaponHP[key] = cur
	if cur <= 0 {
		holder.AddCondition(char.Condition{Name: CondBrokenWeapon, Notes: "item=" + target.Name})
		out.Conditions = append(out.Conditions, CondBrokenWeapon)
		out.Facts = append(out.Facts, (target.Name + " breaks: it is reduced to 0 Hit Points."))
		return nil
	}
	out.Facts = append(out.Facts, fmt.Sprintf("%s takes %d damage: %d of %d Hit Points left.", target.Name, pierced, cur, target.HP))
	return nil
}

// weaponKey names the struck weapon in the WeaponHP ledger: the inventory
// item ID when the holder carries it, else its printed name.
func weaponKey(holder *char.Character, name string) string {
	for _, it := range holder.Inventory {
		if it != nil && strings.EqualFold(it.Name, name) {
			return it.ID
		}
	}
	return name
}

// fxDisarm resolves the printed disarm contest: the recipient resists
// with their Combat Style against the disarmer's original roll, shifted
// one Difficulty Grade per Size step between the two weapons; on failure
// the weapon is flung the disarmer's Damage Modifier metres, and works
// only on creatures up to twice the disarmer's STR (Special Effect
// Descriptions › Disarm Opponent).
func fxDisarm(r *dice.Rand, res *AttackResult, out *EffectResult, actor, victim *char.Character) error {
	if victim.Chars.SIZ > 2*actor.Chars.STR {
		out.Facts = append(out.Facts, (out.Target + " is too large to disarm (creatures up to twice the attacker's STR only)."))
		return nil
	}
	orig := originalRoll(res, out.Side)
	// The weapon knocked loose: the recipient's parrying weapon when the
	// attacker picks this row, the attacking weapon defensively.
	var loose WeaponFacts
	if out.Side == SideOffensive {
		loose = res.ReactionWeapon
		if loose.Name == "" {
			out.Facts = append(out.Facts, "the recipient holds nothing to disarm.")
			return nil
		}
	} else {
		loose = res.WeaponFacts
	}
	vSkill, skillRef, _, err := combatSkill(victim, loose.Name, loose.Unarmed)
	if err != nil {
		return err
	}
	// One grade per Size step between the disarming and disarmed weapons.
	grade := rules.DifficultyStandard
	if a, ok := sizeRank(actorWeapon(res, out.Side).Size); ok {
		if b, ok2 := sizeRank(loose.Size); ok2 {
			steps := a - b
			g := rules.DifficultyGrade(int(rules.DifficultyStandard) + steps)
			if g < rules.DifficultyVeryEasy {
				g = rules.DifficultyVeryEasy
			}
			if g > rules.DifficultyHopeless {
				g = rules.DifficultyHopeless
			}
			grade = g
		}
	}
	modified, err := grade.Modify(vSkill)
	if err != nil {
		if errors.Is(err, rules.ErrHopeless) {
			// No attempt can be made: the weapon comes loose.
			return disarmFlung(r, out, actor, victim, loose)
		}
		if errors.Is(err, rules.ErrAutomatic) {
			out.Facts = append(out.Facts, (out.Target + " cannot resist the disarm: automatic — but the roll is refused (Automatic means no roll)."))
			return nil
		}
		return fmt.Errorf("combat: disarm difficulty: %w", err)
	}
	roll, err := r.Roll("1d100")
	if err != nil {
		return fmt.Errorf("combat: disarm roll: %w", err)
	}
	out.Rolls = append(out.Rolls, NamedRoll{"disarm resistance", roll})
	g, err := rules.Classify(roll.Total, modified)
	if err != nil {
		return fmt.Errorf("combat: classify disarm: %w", err)
	}
	vOut := rules.Outcome{Roll: roll.Total, Skill: modified, Grade: g}
	opp := rules.Opposed(vOut, orig)
	if res.ForcedFailure {
		opp = rules.OpposedResult{A: vOut, B: orig, Winner: 2}
	}
	if opp.Winner == 1 {
		out.Facts = append(out.Facts, fmt.Sprintf("%s holds on (%s vs the disarmer's original roll).", out.Target, skillRef))
		return nil
	}
	return disarmFlung(r, out, actor, victim, loose)
}

// disarmFlung resolves a lost disarm: the weapon flies the disarmer's
// Damage Modifier metres, or drops at the feet when there is none.
func disarmFlung(r *dice.Rand, out *EffectResult, actor, victim *char.Character, loose WeaponFacts) error {
	metres := 0
	d, err := char.Derive(actor)
	if err != nil {
		return fmt.Errorf("combat: derive %s: %w", idOf(actor), err)
	}
	if d.DamageModifier != "" && d.DamageModifier != "0" {
		dm, err := r.Roll(d.DamageModifier)
		if err != nil {
			return fmt.Errorf("combat: fling distance %q: %w", d.DamageModifier, err)
		}
		out.Rolls = append(out.Rolls, NamedRoll{"fling distance", dm})
		metres = dm.Total
		if metres < 0 {
			metres = 0
		}
	}
	out.Distance = metres
	dropped := false
	for _, it := range victim.Inventory {
		if it != nil && it.Equipped && strings.EqualFold(it.Name, loose.Name) {
			it.Equipped = false
			it.Slot = ""
			dropped = true
			break
		}
	}
	if dropped {
		out.Facts = append(out.Facts, fmt.Sprintf("%s's %s is knocked from their hands and lands %d metre(s) away.", out.Target, loose.Name, metres))
	} else {
		out.Facts = append(out.Facts, fmt.Sprintf("%s loses their grip on %s; it drops %d metre(s) away (GM places it).", out.Target, loose.Name, metres))
	}
	return nil
}

// actorWeapon returns the disarmer's own weapon facts.
func actorWeapon(res *AttackResult, side string) WeaponFacts {
	if side == SideOffensive {
		return res.WeaponFacts
	}
	return res.ReactionWeapon
}

// originalRoll returns the applier's original roll — the value every
// "opposed vs the original roll" effect resists against.
func originalRoll(res *AttackResult, side string) rules.Outcome {
	if side == SideOffensive {
		return res.AttackOutcome
	}
	return res.ReactionOutcome
}

// fxDropFoe resolves the firearm Drop Foe: a wounded target opposes
// Endurance against the hit roll and, on failure, is incapacitated for
// one hour divided by their Healing Rate (First Aid or a booster can
// rouse them).
func fxDropFoe(r *dice.Rand, res *AttackResult, out *EffectResult, victim *char.Character) error {
	if res.DamageApplied <= 0 {
		out.Facts = append(out.Facts, "the shot inflicts no wound: Drop Foe needs at least a Minor Wound.")
		return nil
	}
	opp, err := opposeOriginal(r, res, victim, "Endurance", out, "endurance")
	if err != nil {
		return err
	}
	if opp.Winner == 1 {
		out.Facts = append(out.Facts, (out.Target + " shakes off the shock."))
		return nil
	}
	d, err := char.Derive(victim)
	if err != nil {
		return fmt.Errorf("combat: derive %s: %w", idOf(victim), err)
	}
	hr := d.HealingRate
	if hr < 1 {
		hr = 1
	}
	minutes := (60 + hr - 1) / hr
	victim.AddCondition(char.Condition{Name: char.CondIncapacitated, Notes: fmt.Sprintf("Drop Foe: incapacitated for %d minutes", minutes)})
	out.Conditions = append(out.Conditions, char.CondIncapacitated)
	out.Facts = append(out.Facts, fmt.Sprintf("%s succumbs: incapacitated for %d minutes (1 hour ÷ Healing Rate %d).", out.Target, minutes, hr))
	return nil
}

// fxEnhanceParry deflects the entire force of the blow regardless of
// Size: the exchange's wound is reversed exactly — Hit Points restored,
// the wound record dropped, conditions the wound path added removed and
// the defender's pre-wound state restored (Special Effect Descriptions ›
// Enhance Parry).
func fxEnhanceParry(res *AttackResult, out *EffectResult, victim *char.Character) error {
	if res.Reaction != ReactionParry {
		out.Facts = append(out.Facts, "Enhance Parry applies only to a parry.")
		return nil
	}
	if !res.Hit || res.DamagePre == 0 {
		out.Facts = append(out.Facts, (out.Actor + "'s critical parry absorbs the whole attack — nothing landed."))
		return nil
	}
	if res.DamageApplied > 0 || res.Severity != char.SeverityNone {
		if err := revertExchangeWound(res, victim); err != nil {
			return err
		}
		out.Facts = append(out.Facts, fmt.Sprintf("%s's enhanced parry deflects all %d damage: the wound and its consequences are reversed.",
			out.Actor, res.DamagePre))
	} else {
		out.Facts = append(out.Facts, fmt.Sprintf("%s's enhanced parry deflects all %d damage.", out.Actor, res.DamagePre))
	}
	res.ParryApplied = true
	res.ParryReduction = res.DamagePre
	return nil
}

// woundConditionNames are the conditions the wound path (ApplyWound,
// rollConsequences, the Serious-Wound ban, Impale and Bleed — all of
// which hang on the wound existing) may add: a reversed wound takes them
// back. Effects that stand independently of damage (Blind, Trip, Grip,
// Entangle, Pin Down) keep theirs.
var woundConditionNames = []string{
	CondNoAttacks, CondUselessLimb, CondImpaled, CondUntreatedMajorWound, CondFirstAid,
	char.CondStunned, char.CondBleeding,
	char.CondProne, char.CondIncapacitated, char.CondUnconscious,
}

// revertExchangeWound restores the defender to their pre-wound state:
// Hit Points healed, the wound records the exchange appended dropped,
// wound-path conditions returned to their pre-exchange shape, and the
// life flag rolled back (a consequence's instant death is undone too).
func revertExchangeWound(res *AttackResult, victim *char.Character) error {
	if res.DamageApplied <= 0 {
		return nil
	}
	if err := victim.Heal(res.Location, res.DamageApplied); err != nil {
		return fmt.Errorf("combat: reverse wound: %w", err)
	}
	if res.preCond != nil {
		if len(victim.Wounds) >= res.preCond.WoundsLen {
			victim.Wounds = victim.Wounds[:res.preCond.WoundsLen]
		}
		snapshot := make(map[string]bool, len(res.preCond.Conditions))
		for _, cond := range res.preCond.Conditions {
			snapshot[cond.Name+"|"+cond.Location] = true
		}
		for _, name := range woundConditionNames {
			kept := victim.Conditions[:0]
			for _, cond := range victim.Conditions {
				if cond.Name == name && !snapshot[name+"|"+cond.Location] {
					continue
				}
				kept = append(kept, cond)
			}
			victim.Conditions = kept
			// restore anything the snapshot held under this name that the
			// wound path replaced or dropped
			for _, cond := range res.preCond.Conditions {
				if cond.Name != name {
					continue
				}
				present := false
				for _, have := range victim.Conditions {
					if have.Name == cond.Name && have.Location == cond.Location {
						present = true
						break
					}
				}
				if !present {
					victim.AddCondition(cond)
				}
			}
		}
		victim.Dead = res.preCond.Dead
	}
	res.DamageApplied = 0
	res.ArmorAbsorbed = 0
	res.Severity = char.SeverityNone
	res.Adjustments = append(res.Adjustments, "Enhance Parry: the exchange's wound is reversed.")
	return nil
}

// fxEntangle immobilises the struck location with an entangling weapon;
// escape is opposed Brawn on a Struggle action, or a won Damage Weapon,
// Disarm or Slip Free (Special Effect Descriptions › Entangle).
func fxEntangle(res *AttackResult, out *EffectResult, side string, victim *char.Character, opts EffectOptions) {
	loc := opts.Location
	if side == SideOffensive {
		if res.Location == "" {
			out.Facts = append(out.Facts, "no blow landed: there is no location to entangle.")
			return
		}
		loc = res.Location
	}
	victim.AddCondition(char.Condition{Name: CondEntangled, Location: loc, Notes: "escape: opposed Brawn, or win Damage Weapon, Disarm Opponent or Slip Free"})
	out.Conditions = append(out.Conditions, CondEntangled)
	switch locationClass(loc) {
	case locHead, locTorso:
		out.Facts = append(out.Facts, fmt.Sprintf("%s is entangled at the %s: all skill rolls one grade harder.", out.Target, loc))
	case locWholeBody:
		out.Facts = append(out.Facts, (out.Target + " is entangled: escape with opposed Brawn."))
	default:
		if isLegLocation(loc) {
			out.Facts = append(out.Facts, fmt.Sprintf("%s's entangled %s pins them in place: no movement.", out.Target, loc))
		} else {
			out.Facts = append(out.Facts, fmt.Sprintf("%s's entangled %s cannot use what it holds.", out.Target, loc))
		}
	}
	out.Facts = append(out.Facts, "the wielder may spend 1 AP on their next Turn for an automatic Trip Opponent attempt.")
}

// fxImpale rerolls the weapon damage (the exchange's roll was the first
// of the two; the attacker takes the better result), then — when the
// wound is real and the weapon can lodge — leaves the weapon in the
// wound with the Impale Effects Table penalty (Special Effect
// Descriptions › Impale; tables.md §4.7).
func fxImpale(r *dice.Rand, res *AttackResult, out *EffectResult, actor, victim *char.Character) error {
	if res.Location == "" || res.DamageRoll.Total == 0 {
		out.Facts = append(out.Facts, "no blow landed: there is nothing to impale.")
		return nil
	}
	expr := res.WeaponFacts.Damage
	if expr == "" {
		expr = "1d3"
	}
	extra, err := r.Roll(expr)
	if err != nil {
		return fmt.Errorf("combat: impale damage %q: %w", expr, err)
	}
	out.Rolls = append(out.Rolls, NamedRoll{"impale damage", extra})
	chosen := extra.Total
	if res.DamageRoll.Total > chosen {
		chosen = res.DamageRoll.Total
	}
	out.Facts = append(out.Facts, fmt.Sprintf("damage rolled twice: %d and %d — %d taken.", res.DamageRoll.Total, extra.Total, chosen))
	delta := chosen - res.DamageRoll.Total
	if delta > 0 {
		out.Delta = delta
		if err := applyExtraWound(r, res, victim, delta, "Impale"); err != nil {
			return err
		}
	}
	if res.DamageApplied <= 0 {
		out.Facts = append(out.Facts, "the armour keeps the lodged weapon from taking hold: no Impale penalty.")
		return nil
	}
	w := res.WeaponFacts
	lodge := false
	switch {
	case !w.Ranged:
		lodge = true // a melee weapon left in the wound
	case w.ImpaleSize != "":
		lodge = true // an arrow or bolt stays behind
	}
	if !lodge {
		out.Facts = append(out.Facts, "the projectile leaves no secondary impaling effect (no printed Impale Size).")
		return nil
	}
	sizeForTable := w.Size
	if w.Ranged {
		sizeForTable = w.ImpaleSize
	}
	grade := impalePenaltyGrade(victim.Chars.SIZ, sizeForTable)
	victim.AddCondition(char.Condition{
		Name:     CondImpaled,
		Location: res.Location,
		Notes:    fmt.Sprintf("weapon=%s; grade=%s", res.Weapon, strings.ToLower(grade)),
	})
	out.Conditions = append(out.Conditions, CondImpaled)
	if !w.Ranged {
		// The attacker's own weapon is stuck in the wound: it cannot
		// parry while it remains (selectParryWeapon checks this).
		actor.AddCondition(char.Condition{Name: CondImpaled, Notes: fmt.Sprintf("weapon=%s; lodged in %s", res.Weapon, out.Target)})
		out.Conditions = append(out.Conditions, CondImpaled)
	}
	if grade == "No Effect" {
		out.Facts = append(out.Facts, fmt.Sprintf("%s's %s lodges in the %s: no Impale penalty at SIZ %d.", out.Actor, w.Name, res.Location, victim.Chars.SIZ))
	} else {
		out.Facts = append(out.Facts, fmt.Sprintf("%s's %s lodges in the %s: future skill attempts are %s until withdrawn (Impale Effects Table).",
			out.Actor, w.Name, res.Location, strings.ToLower(grade)))
	}
	out.Facts = append(out.Facts, "withdrawal: Ready Weapon action, unopposed Brawn (opposed if resisted) — half the normal damage, no Damage Modifier, armour does not reduce it.")
	return nil
}

// impalePenaltyGrade reads the Impale Effects Table (tables.md §4.7):
// the penalty climbs with weapon Size and falls as the creature's SIZ
// band rises (each +10 SIZ steps the whole row one place easier, clamped
// at "No Effect"). Sizes beyond the printed columns follow the same
// progression.
func impalePenaltyGrade(siz int, size string) string {
	// severity ladder, easiest to hardest, as printed down a column
	ladder := []string{"No Effect", "Hard", "Formidable", "Herculean", "Incapacitated"}
	rank, ok := sizeRank(size)
	if !ok {
		rank = 2 // Medium is the unarmed/default column
	}
	band := (max(siz, 1) - 1) / 10
	idx := 2 + (rank - 1) - band // SIZ 1-10, Small = Formidable
	if idx < 0 {
		idx = 0
	}
	if idx > len(ladder)-1 {
		idx = len(ladder) - 1
	}
	return ladder[idx]
}

// fxMaximise substitutes one weapon damage die for its full value; the
// Damage Modifier is never affected (Special Effect Descriptions ›
// Maximise Damage). Each pick maximises the next un-maximised die, so
// stacking works.
func fxMaximise(r *dice.Rand, res *AttackResult, out *EffectResult, victim *char.Character) error {
	if res.Location == "" || res.DamageRoll.Total == 0 {
		out.Facts = append(out.Facts, "no damage was rolled: there is nothing to maximise.")
		return nil
	}
	k := res.Maximised
	if k >= len(res.DamageRoll.Dice) {
		out.Facts = append(out.Facts, ("every die of " + res.DamageRoll.Expr + " is already maximised."))
		return nil
	}
	die := res.DamageRoll.Dice[k]
	res.Maximised++
	delta := die.Sides - die.Value
	out.Facts = append(out.Facts, fmt.Sprintf("one %dd%d die becomes %d (was %d).", die.Sides, die.Sides, die.Sides, die.Value))
	if delta <= 0 {
		return nil
	}
	out.Delta = delta
	if err := applyExtraWound(r, res, victim, delta, "Maximise Damage"); err != nil {
		return err
	}
	out.Facts = append(out.Facts, fmt.Sprintf("+%d damage: %d now stands on the %s.", delta, res.DamageApplied, res.Location))
	return nil
}

// fxOverpenetration sends the shot through the first victim into a second
// behind them for half damage, rolled through the second target's own
// Hit Location and armour; the first victim's Special Effects do not
// carry over (Special Effect Descriptions › Over-penetration).
// [AMBIGUOUS: whether the second victim's armour applies is unstated —
// the engine runs the normal damage application, so it does.]
func fxOverpenetration(r *dice.Rand, e *Encounter, res *AttackResult, out *EffectResult, opts EffectOptions) error {
	if opts.Target == nil {
		return fmt.Errorf("%w: Overpenetration needs a second target", ErrInvalidChoice)
	}
	second := opts.Target
	if _, err := e.indexOf(second); err != nil {
		return fmt.Errorf("combat: overpenetration: %w", err)
	}
	if res.DamageApplied <= 0 {
		out.Facts = append(out.Facts, "the shot does not overcome the first target's armour: it never reaches anyone behind.")
		return nil
	}
	half := ceilInt(res.DamagePre, 2)
	locRoll, err := r.Roll("1d20")
	if err != nil {
		return fmt.Errorf("combat: overpenetration location: %w", err)
	}
	out.Rolls = append(out.Rolls, NamedRoll{"location", locRoll})
	loc, err := rollLocation(second, locRoll.Total)
	if err != nil {
		return err
	}
	comps, err := armorComponents(second, loc)
	if err != nil {
		return err
	}
	ap := 0
	for _, c := range comps {
		ap += c.AP
	}
	absorbed := ap
	if half < absorbed {
		absorbed = half
	}
	applied := half - absorbed
	out.Location = loc
	out.DamageApplied = applied
	out.Delta = applied
	out.Facts = append(out.Facts, fmt.Sprintf("the shot penetrates %s and strikes %s: half damage (%d) at the %s, %d absorbed by armour.",
		out.Target, idOf(second), half, loc, absorbed))
	if applied == 0 {
		return nil
	}
	sev, _, err := second.ApplyWound(loc, applied)
	if err != nil {
		return fmt.Errorf("combat: overpenetration wound: %w", err)
	}
	out.Severity = sev
	out.Facts = append(out.Facts, fmt.Sprintf("%s takes %d damage (%s); no Special Effects carry to them.", idOf(second), applied, sev))
	if sev == char.SeveritySerious || sev == char.SeverityMajor {
		cons, err := rollConsequences(r, res, second, sev, loc, applied)
		if err != nil {
			return err
		}
		for _, c := range cons {
			out.Facts = append(out.Facts, "Endurance "+c.EnduranceOutcome.String()+": "+consequenceText(c))
		}
	}
	return nil
}

// fxPinDown opposes the target's Willpower against the hit roll; failure
// pins them down — no return fire next Turn. It works even when no
// damage was inflicted (Special Effect Descriptions › Pin Down).
func fxPinDown(r *dice.Rand, res *AttackResult, out *EffectResult, victim *char.Character) error {
	opp, err := opposeOriginal(r, res, victim, "Willpower", out, "willpower")
	if err != nil {
		return err
	}
	if opp.Winner == 1 {
		out.Facts = append(out.Facts, (out.Target + " keeps their nerve."))
		return nil
	}
	victim.AddCondition(char.Condition{Name: CondPinned, Rounds: 2})
	out.Conditions = append(out.Conditions, CondPinned)
	out.Facts = append(out.Facts, (out.Target + " is pinned down: no return fire on their next Turn (crawling, reloading and talking stay allowed)."))
	return nil
}

// fxPrepareCounter records the defender's secret nomination in encounter
// state so it survives a save (Special Effect Descriptions › Prepare
// Counter).
func fxPrepareCounter(e *Encounter, out *EffectResult, actor *char.Character, opts EffectOptions) error {
	if opts.CounterAgainst == "" {
		return fmt.Errorf("%w: Prepare Counter needs the effect held ready", ErrInvalidChoice)
	}
	if _, ok := EffectByName(opts.CounterAgainst); !ok {
		return fmt.Errorf("%w: prepare counter against unknown effect %q", ErrUnknownEffect, opts.CounterAgainst)
	}
	if opts.CounterSubstitute != "" {
		if _, ok := EffectByName(opts.CounterSubstitute); !ok {
			return fmt.Errorf("%w: prepare counter substitute %q", ErrUnknownEffect, opts.CounterSubstitute)
		}
	}
	idx, err := e.indexOf(actor)
	if err != nil {
		return err
	}
	entry := CounterEntry{Index: idx, Against: opts.CounterAgainst, Substitute: opts.CounterSubstitute}
	for i, ctr := range e.Counters {
		if ctr.Index == idx && strings.EqualFold(ctr.Against, opts.CounterAgainst) {
			e.Counters[i] = entry
			out.Facts = append(out.Facts, fmt.Sprintf("%s quietly readies a counter to %s.", out.Actor, opts.CounterAgainst))
			return nil
		}
	}
	e.Counters = append(e.Counters, entry)
	out.Facts = append(out.Facts, fmt.Sprintf("%s quietly readies a counter to %s.", out.Actor, opts.CounterAgainst))
	return nil
}

// fxSelectTarget redirects a fumbled blow onto an adjacent bystander,
// who is struck automatically and suffers no Special Effect (Special
// Effect Descriptions › Select Target).
func fxSelectTarget(r *dice.Rand, e *Encounter, res *AttackResult, out *EffectResult, actor *char.Character, opts EffectOptions) error {
	if opts.Target == nil {
		return fmt.Errorf("%w: Select Target needs the bystander struck instead", ErrInvalidChoice)
	}
	bystander := opts.Target
	if _, err := e.indexOf(bystander); err != nil {
		return fmt.Errorf("combat: select target: %w", err)
	}
	// The blow moves off its original recipient (the defensive side's
	// actor is the defender who was struck).
	if res.DamageApplied > 0 {
		if err := revertExchangeWound(res, actor); err != nil {
			return err
		}
	}
	// The blow no longer stands on the defender: later picks must not
	// re-wound them at the old location.
	res.Location = ""
	res.Severity = char.SeverityNone
	// The misdirected strike is not subject to the defender's parry or
	// armour: it lands with the fumbled blow's original damage.
	half := res.DamagePre
	if half <= 0 {
		out.Facts = append(out.Facts, "there is no damage to redirect.")
		return nil
	}
	locRoll, err := r.Roll("1d20")
	if err != nil {
		return fmt.Errorf("combat: select target location: %w", err)
	}
	out.Rolls = append(out.Rolls, NamedRoll{"location", locRoll})
	loc, err := rollLocation(bystander, locRoll.Total)
	if err != nil {
		return err
	}
	comps, err := armorComponents(bystander, loc)
	if err != nil {
		return err
	}
	ap := 0
	for _, c := range comps {
		ap += c.AP
	}
	absorbed := ap
	if half < absorbed {
		absorbed = half
	}
	applied := half - absorbed
	out.Location = loc
	out.DamageApplied = applied
	out.Delta = applied
	out.Facts = append(out.Facts, fmt.Sprintf("%s's fumbled blow is deflected onto %s: %d damage at the %s, %d absorbed by armour.",
		out.Actor, idOf(bystander), half, loc, absorbed))
	if applied == 0 {
		return nil
	}
	sev, _, err := bystander.ApplyWound(loc, applied)
	if err != nil {
		return fmt.Errorf("combat: select target wound: %w", err)
	}
	out.Severity = sev
	out.Facts = append(out.Facts, fmt.Sprintf("%s takes %d damage (%s), struck automatically and immune to Special Effects.", idOf(bystander), applied, sev))
	if sev == char.SeveritySerious || sev == char.SeverityMajor {
		cons, err := rollConsequences(r, res, bystander, sev, loc, applied)
		if err != nil {
			return err
		}
		for _, c := range cons {
			out.Facts = append(out.Facts, "Endurance "+c.EnduranceOutcome.String()+": "+consequenceText(c))
		}
	}
	return nil
}

// fxStun resolves Stun Location: a wounding bludgeoning blow opposes
// Endurance against the attack roll; failure incapacitates the location
// for turns equal to the damage inflicted — torso staggered (defends
// only), head insensible (Special Effect Descriptions › Stun Location).
func fxStun(r *dice.Rand, res *AttackResult, out *EffectResult, victim *char.Character) error {
	if res.DamageApplied <= 0 {
		out.Facts = append(out.Facts, "the blow did not overcome armour and injure: no stun.")
		return nil
	}
	opp, err := opposeOriginal(r, res, victim, "Endurance", out, "endurance")
	if err != nil {
		return err
	}
	if opp.Winner == 1 {
		out.Facts = append(out.Facts, (out.Target + " shrugs off the staggering blow."))
		return nil
	}
	victim.AddCondition(char.Condition{Name: char.CondStunned, Location: res.Location, Rounds: res.DamageApplied})
	out.Conditions = append(out.Conditions, char.CondStunned)
	switch locationClass(res.Location) {
	case locHead:
		out.Facts = append(out.Facts, fmt.Sprintf("the head shot leaves %s insensible: incapacitated for %d Rounds.", out.Target, res.DamageApplied))
	case locTorso:
		out.Facts = append(out.Facts, fmt.Sprintf("the torso shot staggers %s: they may only defend, for %d Rounds.", out.Target, res.DamageApplied))
	default:
		out.Facts = append(out.Facts, fmt.Sprintf("the %s is incapacitated for %d Rounds.", res.Location, res.DamageApplied))
	}
	return nil
}

// fxSunder applies the blow against one armour component: damage after
// the parry reduces the component's Armor Points (to zero, ripping
// straps and bursting rings), and any surplus — already standing in the
// wound — carries to the location's Hit Points (Special Effect
// Descriptions › Sunder).
func fxSunder(res *AttackResult, out *EffectResult, victim *char.Character, opts EffectOptions) error {
	if res.Location == "" || res.DamagePre == 0 {
		out.Facts = append(out.Facts, "no blow landed: there is no armour to sunder.")
		return nil
	}
	afterParry := res.DamagePre - res.ParryReduction
	comps, err := armorComponents(victim, res.Location)
	if err != nil {
		return err
	}
	if len(comps) == 0 {
		out.Facts = append(out.Facts, ("the " + out.Target + " wears no armour worth sundering."))
		return nil
	}
	pick, err := pickComponent(comps, opts.ArmorComponent)
	if err != nil {
		return err
	}
	taken := pick.AP
	if afterParry < taken {
		taken = afterParry
	}
	if taken == 0 {
		out.Facts = append(out.Facts, ("nothing of " + out.Target + "'s protection gives way."))
		return nil
	}
	remaining := pick.AP - taken
	if pick.Name == "natural" {
		victim.AddCondition(char.Condition{Name: CondNaturalArmor, Location: res.Location, Notes: fmt.Sprintf("ap: %d", remaining)})
		out.Conditions = append(out.Conditions, CondNaturalArmor)
	} else {
		item, err := victim.FindItem(pick.Name)
		if err != nil {
			return fmt.Errorf("combat: sunder: %w", err)
		}
		if item.Armor == nil {
			return fmt.Errorf("combat: sunder: %w: item %q carries no armour statistics", ErrInvalidChoice, item.Name)
		}
		key := res.Location
		if _, ok := item.Armor.Points[key]; !ok {
			if _, okStar := item.Armor.Points["*"]; !okStar && remaining > 0 {
				key = "*"
			}
		}
		if item.Armor.Points == nil {
			item.Armor.Points = make(map[string]int)
		}
		item.Armor.Points[key] = remaining
	}
	out.Facts = append(out.Facts, fmt.Sprintf("%s's sundering wreck removes %d Armor Points from %s: %d left at the %s.",
		out.Actor, taken, pick.Label, remaining, res.Location))
	out.Facts = append(out.Facts, "any surplus damage was already carried to the location's Hit Points.")
	return nil
}

// fxTrip resolves Trip Opponent: the target resists with Brawn, Evade or
// Acrobatics against the applier's original roll — quadrupeds use
// Athletics one grade easier — and falls prone on failure (Special
// Effect Descriptions › Trip Opponent).
func fxTrip(r *dice.Rand, res *AttackResult, out *EffectResult, victim *char.Character, opts EffectOptions) error {
	skillName := opts.ResistSkill
	if skillName == "" {
		skillName = "Evade"
	}
	grade := rules.DifficultyStandard
	if opts.Quadruped {
		skillName = "Athletics"
		grade = rules.DifficultyEasy // one grade easier
	}
	s, err := rules.SkillValue(victim, skillName)
	if err != nil {
		return fmt.Errorf("combat: trip resistance: %w", err)
	}
	modified, err := grade.Modify(s)
	if err != nil {
		return fmt.Errorf("combat: trip difficulty: %w", err)
	}
	roll, err := r.Roll("1d100")
	if err != nil {
		return fmt.Errorf("combat: trip roll: %w", err)
	}
	out.Rolls = append(out.Rolls, NamedRoll{strings.ToLower(skillName), roll})
	g, err := rules.Classify(roll.Total, modified)
	if err != nil {
		return fmt.Errorf("combat: classify trip: %w", err)
	}
	vOut := rules.Outcome{Roll: roll.Total, Skill: modified, Grade: g}
	orig := originalRoll(res, out.Side)
	opp := rules.Opposed(vOut, orig)
	if res.ForcedFailure {
		opp = rules.OpposedResult{A: vOut, B: orig, Winner: 2}
	}
	if opp.Winner == 1 {
		out.Facts = append(out.Facts, fmt.Sprintf("%s keeps their feet (%s vs the original roll).", out.Target, skillName))
		return nil
	}
	victim.AddCondition(char.Condition{Name: char.CondProne})
	out.Conditions = append(out.Conditions, char.CondProne)
	out.Facts = append(out.Facts, fmt.Sprintf("%s fails the %s contest and falls prone.", out.Target, skillName))
	return nil
}

// ---------------------------------------------------------------------------
// shared wound and resistance helpers
// ---------------------------------------------------------------------------

// opposeOriginal rolls victim's skill against the exchange's original
// attack roll — the contest every "opposed vs. the attack/hit roll" effect
// prints (Bleed, Stun Location, Drop Foe, Pin Down). The victim's roll
// lands in out.Rolls; a recorded Force Failure auto-fails it.
func opposeOriginal(r *dice.Rand, res *AttackResult, victim *char.Character, skill string, out *EffectResult, rollName string) (rules.OpposedResult, error) {
	s, ok := victim.Skill(skill)
	if !ok {
		return rules.OpposedResult{}, fmt.Errorf("combat: %s resistance: %s: %w", idOf(victim), skill, char.ErrUnknownSkill)
	}
	eqVictim, eqAtk := rules.EqualiseSkillsOver100(s.Value, res.AttackSkill)
	roll, err := r.Roll("1d100")
	if err != nil {
		return rules.OpposedResult{}, fmt.Errorf("combat: %s roll: %w", skill, err)
	}
	out.Rolls = append(out.Rolls, NamedRoll{rollName, roll})
	g, err := rules.Classify(roll.Total, eqVictim)
	if err != nil {
		return rules.OpposedResult{}, fmt.Errorf("combat: classify %s: %w", skill, err)
	}
	vOut := rules.Outcome{Roll: roll.Total, Skill: eqVictim, Grade: g}
	aOut := res.AttackOutcome
	if res.AttackOutcome.Roll != 0 {
		// Automatic attacks rolled no die: their recorded Success is the
		// resistance target (see rollConsequences).
		aGrade, err := rules.Classify(res.AttackOutcome.Roll, eqAtk)
		if err != nil {
			return rules.OpposedResult{}, fmt.Errorf("combat: reclassify attack: %w", err)
		}
		aOut = rules.Outcome{Roll: res.AttackOutcome.Roll, Skill: eqAtk, Grade: aGrade}
	}
	opp := rules.Opposed(vOut, aOut)
	if res.ForcedFailure {
		opp = rules.OpposedResult{A: vOut, B: aOut, Winner: 2, Margin: int(aOut.Grade - vOut.Grade), TieBreak: "forced"}
	}
	return opp, nil
}

// applyExtraWound applies damage added after the attack pipeline ran
// (Impale's second roll, Maximise Damage, Bypass Armor, Circumvent
// Parry): the armour's unused share still absorbs, the wound is
// re-applied, and a severity escalation rolls the new level's
// Endurance-opposed consequence and Serious-Wound attack ban.
func applyExtraWound(r *dice.Rand, res *AttackResult, victim *char.Character, delta int, what string) error {
	if delta <= 0 {
		return nil
	}
	unused := res.ArmorPoints - res.ArmorAbsorbed
	if unused < 0 {
		unused = 0
	}
	absorbed := unused
	if delta < absorbed {
		absorbed = delta
	}
	res.ArmorAbsorbed += absorbed
	applied := delta - absorbed
	if applied == 0 {
		res.Adjustments = append(res.Adjustments, fmt.Sprintf("%s: %d damage absorbed by armour.", what, delta))
		return nil
	}
	if err := addWoundDelta(r, res, victim, applied, what); err != nil {
		return err
	}
	res.Adjustments = append(res.Adjustments, fmt.Sprintf("%s: +%d damage (%d applied after armour) to the %s.", what, delta, applied, res.Location))
	return nil
}

// addWoundDelta subtracts applied Hit Points from the exchange's struck
// location and escalates the recorded severity, rolling the escalated
// level's consequence and attack ban.
func addWoundDelta(r *dice.Rand, res *AttackResult, victim *char.Character, applied int, what string) error {
	if applied <= 0 {
		return nil
	}
	prev := res.Severity
	sev, fxList, err := victim.ApplyWound(res.Location, applied)
	if err != nil {
		return fmt.Errorf("combat: %s: %w", what, err)
	}
	removeConditionAt(victim, CondFirstAid, res.Location)
	res.DamageApplied += applied
	if sev <= prev {
		return nil
	}
	res.Severity = sev
	res.WoundEffects = fxList
	if sev >= char.SeveritySerious && prev < char.SeveritySerious {
		ban, err := r.Roll("1d3")
		if err != nil {
			return fmt.Errorf("combat: serious wound ban: %w", err)
		}
		res.NoAttacksRoll = ban
		victim.AddCondition(char.Condition{Name: CondNoAttacks, Rounds: ban.Total})
	}
	if sev >= char.SeveritySerious {
		cons, err := rollConsequences(r, res, victim, sev, res.Location, res.DamageApplied)
		if err != nil {
			return err
		}
		res.Consequences = append(res.Consequences, cons...)
	}
	return nil
}

// relocateWound moves the exchange's wound to newLoc: Hit Points are
// restored at the old location, the damage is re-applied at the new one,
// and location-scoped conditions travel with the blow.
func relocateWound(r *dice.Rand, res *AttackResult, victim *char.Character, newLoc, what string) error {
	old := res.Location
	if old == "" {
		return fmt.Errorf("%w: %s has no hit location to move", ErrInvalidChoice, what)
	}
	if old == newLoc {
		return nil
	}
	d, err := char.Derive(victim)
	if err != nil {
		return fmt.Errorf("combat: derive %s: %w", idOf(victim), err)
	}
	if _, ok := d.LocationHP[newLoc]; !ok {
		return fmt.Errorf("%w: %q is not a hit location of %s", ErrInvalidChoice, newLoc, idOf(victim))
	}
	if res.DamageApplied > 0 {
		if err := victim.Heal(old, res.DamageApplied); err != nil {
			return fmt.Errorf("combat: relocate wound: %w", err)
		}
		if res.preCond != nil && len(victim.Wounds) >= res.preCond.WoundsLen {
			victim.Wounds = victim.Wounds[:res.preCond.WoundsLen]
		}
		for i := range victim.Conditions {
			if victim.Conditions[i].Location == old && locationScoped(victim.Conditions[i].Name) {
				victim.Conditions[i].Location = newLoc
			}
		}
		prev := res.Severity
		sev, fxList, err := victim.ApplyWound(newLoc, res.DamageApplied)
		if err != nil {
			return fmt.Errorf("combat: relocate wound: %w", err)
		}
		res.Severity = sev
		res.WoundEffects = fxList
		if sev > prev && sev >= char.SeveritySerious {
			cons, err := rollConsequences(r, res, victim, sev, newLoc, res.DamageApplied)
			if err != nil {
				return err
			}
			res.Consequences = append(res.Consequences, cons...)
		}
	}
	res.Location = newLoc
	res.Adjustments = append(res.Adjustments, fmt.Sprintf("%s: the hit location moves from %s to %s.", what, old, newLoc))
	return nil
}

// locationScoped reports whether a condition follows its hit location.
func locationScoped(name string) bool {
	for _, n := range []string{CondUselessLimb, CondImpaled, char.CondStunned, CondFirstAid, CondEntangled} {
		if name == n {
			return true
		}
	}
	return false
}

// adjoining reports whether b is an immediately adjoining hit location of
// a — the one-step Marksman shift. Physiology decides what adjoins what:
// Head meets Chest; the abdomen meets the legs and the chest; the chest
// meets the head, the arms and the abdomen (the rulebook's own example
// moves a leg shot "up to the abdomen"); creature stat-block locations
// fall back to head/torso/limb adjacency. [AMBIGUOUS: the source only
// says "an immediately adjoining body area" and leaves the physiology to
// common sense — Special Effect Descriptions › Marksman.]
func adjoining(c *char.Character, a, b string) (bool, error) {
	name := c.HitLocationCatalog
	if name == "" {
		name = char.DefaultCatalog
	}
	defs, err := char.Catalog(name)
	if err != nil {
		return false, fmt.Errorf("combat: adjoining locations: %w", err)
	}
	var candidates []string
	switch {
	case locationClass(a) == locHead:
		for _, d := range defs {
			if locationClass(d.Name) == locTorso {
				candidates = append(candidates, d.Name)
			}
		}
	case a == "Abdomen":
		// the abdomen adjoins the legs and the chest
		for _, d := range defs {
			if isLegLocation(d.Name) || locationClass(d.Name) == locTorso && d.Name != a {
				candidates = append(candidates, d.Name)
			}
		}
	case a == "Chest":
		// the chest adjoins the head, the arms and the abdomen
		for _, d := range defs {
			if d.Name == a {
				continue
			}
			if locationClass(d.Name) == locHead ||
				(locationClass(d.Name) == locLimb && !isLegLocation(d.Name)) ||
				locationClass(d.Name) == locTorso {
				candidates = append(candidates, d.Name)
			}
		}
	case locationClass(a) == locTorso:
		// other creature torsos: head and every limb
		for _, d := range defs {
			if locationClass(d.Name) == locHead || locationClass(d.Name) == locLimb {
				candidates = append(candidates, d.Name)
			}
		}
	default:
		// a limb reaches its torso: abdomen for legs, chest otherwise
		want := "Chest"
		if isLegLocation(a) {
			want = "Abdomen"
		}
		found := false
		for _, d := range defs {
			if d.Name == want {
				found = true
				break
			}
		}
		if !found {
			want = ""
			for _, d := range defs {
				if locationClass(d.Name) == locTorso {
					want = d.Name
					break
				}
			}
		}
		if want != "" {
			candidates = append(candidates, want)
		}
	}
	return containsString(candidates, b), nil
}

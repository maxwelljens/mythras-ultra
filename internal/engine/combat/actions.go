package combat

import (
	"errors"
	"fmt"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
)

// Sentinels returned by SpendAP and CanAct. Callers branch on them with
// errors.Is to turn a refusal into a structured IMPOSSIBLE result.
var (
	// ErrUnknownAction reports a name the Combat Actions table does not
	// print (data_actions.go).
	ErrUnknownAction = errors.New("combat: unknown action")
	// ErrUnknownReaction reports a Reaction value outside "parry", "dodge"
	// and "none".
	ErrUnknownReaction = errors.New("combat: unknown reaction")
	// ErrUnknownWeapon reports a weapon name that is neither wielded by the
	// attacker, in their inventory, nor in the sample weapon tables.
	ErrUnknownWeapon = errors.New("combat: unknown weapon")
	// ErrMalfunction reports an attack made with a firearm that carries the
	// Weapon Malfunction condition ("rendered useless until time can be
	// spent field stripping and cleaning it" — Special Effect Descriptions).
	ErrMalfunction = errors.New("combat: weapon malfunctioning")
)

// Combat-owned condition names. The rulebook gives these states durations
// and locations but the char package only carries the table conditions, so
// combat adds its own; resolve and the harness read them by these names.
// Round-limited names tick down in Encounter.NextRound.
const (
	// CondNoAttacks is the Serious-Wound ban: "cannot attack or start to
	// cast spells for 1d3 Turns (may Parry/Evade)" (tables.md §3.9).
	// Rounds carries the rolled 1d3.
	CondNoAttacks = "No Attacks"
	// CondUselessLimb scopes a failed limb Endurance roll: the location is
	// useless until it returns to positive HP (tables.md §3.9).
	CondUselessLimb = "Useless Limb"
	// CondBlinded carries the Blind Opponent effect; Notes holds the
	// difficulty grade ("hard" or "formidable") for 1d3 Rounds.
	CondBlinded = "Blinded"
	// CondImpaled records an impaled weapon: Notes holds the Impale Effects
	// Table penalty grade and the weapon name (the weapon cannot parry
	// while it remains).
	CondImpaled = "Impaled"
	// CondEntangled is the Entangle effect (Rounds 0 until escaped).
	CondEntangled = "Entangled"
	// CondGripped is the Grip effect (Rounds 0 until escaped).
	CondGripped = "Gripped"
	// CondPinned is the Pin Down effect; Rounds 2 covers the target's next
	// Turn ("cannot return fire on their next Turn").
	CondPinned = "Pinned Down"
	// CondMalfunction is the Weapon Malfunction effect; Notes names the
	// firearm.
	CondMalfunction = "Weapon Malfunction"
	// CondBrokenWeapon records a weapon broken by the Damage Weapon effect
	// (0 HP); Notes names the item.
	CondBrokenWeapon = "Broken Weapon"
	// CondSurprised is the Surprise state: flat-footed (cannot defend) and
	// no offensive Actions for the rest of the Round (tables.md §4.12).
	// Rounds is 1. Notes records whether the first-attack bonus Special
	// Effect has been spent.
	CondSurprised = "Surprised"
	// CondBraced is the Brace action: SIZ +50% vs Knockback/Leaping, ×2 vs
	// Bash, until the character moves (Rounds 0 — removed by the GM or
	// resolve when the braced character moves).
	CondBraced = "Braced"
	// CondScarred records the Scar Foe effect; Notes describes the scar.
	CondScarred = "Scarred"
	// CondRapidReload is the Rapid Reload effect. Rounds is its stack
	// counter (−1 reload per stack), not a duration — exempt from round
	// ticking (tickExempt).
	CondRapidReload = "Rapid Reload"
	// CondUntreatedMajorWound is the Major-Wound treatment timer: the
	// character dies when it expires — "survivors die unless treated within
	// Combat Rounds = 2 × Healing Rate" (tables.md §3.9).
	CondUntreatedMajorWound = "Untreated Major Wound"
	// CondNaturalArmor records natural protection reduced by Sunder: the
	// catalog prints the base value, this condition carries the remaining
	// Armor Points at the location ("ap: N"). Worn armour is degraded in
	// place on the inventory item instead (armorComponents reads both).
	CondNaturalArmor = "Natural Armor"
	// CondFirstAid marks an injury already treated by First Aid ("First Aid
	// may be applied only once per specific injury"); scoped to the hit
	// location. A new wound to the same location clears it.
	CondFirstAid = "First Aid"
)

// actBlocker is one reason an actor cannot perform an action: a
// human-readable message for CanAct and the sentinel SpendAP wraps.
type actBlocker struct {
	msg      string
	sentinel error
}

// actBlockers validates action, encounter and actor state and returns the
// action row plus every reason the actor cannot perform it. The error result
// is for inputs no blocker list can express: an unknown action or an
// encounter that is not running.
func actBlockers(e *Encounter, actor *char.Character, action string) (Action, []actBlocker, error) {
	act, ok := ActionByName(action)
	if !ok {
		return Action{}, nil, fmt.Errorf("%w: %q", ErrUnknownAction, action)
	}
	if err := e.running(); err != nil {
		return act, nil, err
	}
	if actor == nil {
		return act, []actBlocker{{"no actor given", ErrCannotAct}}, nil
	}

	var blockers []actBlocker
	cannot := func(reason string) {
		blockers = append(blockers, actBlocker{reason, ErrCannotAct})
	}
	// A character outside the encounter is a distinct sentinel, so it is
	// checked before any state reason.
	if _, err := e.indexOf(actor); err != nil {
		blockers = append(blockers, actBlocker{
			idOf(actor) + " is not a participant in this encounter",
			ErrNotInEncounter,
		})
		return act, blockers, nil
	}

	if actor.Dead {
		cannot(idOf(actor) + " is dead")
	}
	if actor.HasCondition(char.CondUnconscious) {
		cannot(idOf(actor) + " is unconscious")
	}
	if actor.HasCondition(char.CondIncapacitated) {
		// A Major Wound "immediately Incapacitated, drops prone"
		// (tables.md §3.9): no actions until recovered.
		cannot(idOf(actor) + " is incapacitated")
	}
	for _, cond := range actor.Conditions {
		if cond.Name != char.CondStunned {
			continue
		}
		// Stun Location incapacitates the struck location: a head is
		// insensible (no action), the torso staggered (may only defend),
		// an arm cannot attack, a leg cannot move. Whole-body stun blocks
		// everything. [AMBIGUOUS: the effect prints outcomes for torso and
		// head only (Stun Location); limb mappings follow "the Hit Location
		// is Incapacitated".] The engine cannot know which limb wields the
		// weapon, so a stunned arm blocks Attack conservatively.
		switch locationClass(cond.Location) {
		case locHead:
			cannot(idOf(actor) + " is insensible (stunned head)")
		case locTorso:
			if act.Type == Proactive {
				cannot(idOf(actor) + " is staggered (stunned torso) and may only defend")
			}
		default:
			switch {
			case cond.Location == "":
				cannot(idOf(actor) + " is stunned and cannot act")
			case isLegLocation(cond.Location) && (act.Name == "Move" || act.Name == "Charge"):
				cannot(("stunned location " + cond.Location + " cannot move"))
			case !isLegLocation(cond.Location) && act.Name == "Attack":
				cannot(("stunned location " + cond.Location + " cannot attack"))
			}
		}
	}
	if actor.HasCondition(CondSurprised) && (act.Name == "Attack" || act.Name == "Charge") {
		// Surprise: "For the remainder of the Round, they may not perform
		// any offensive Action" (tables.md §4.12).
		cannot(idOf(actor) + " is surprised: no offensive action for the rest of the Round")
	}
	if actor.HasCondition(char.CondProne) && (act.Name == "Move" || act.Name == "Charge") {
		// A prone character crawls (max 2 metres per Turn) or stands with
		// Regain Footing; there is no standing sprint from prone
		// (Combat › Movement; Regain Footing). Fighting from prone is a
		// Formidable penalty, handled by ResolveAttack, not a block here.
		cannot(idOf(actor) + " is prone: crawl (2 metres per Turn) or stand with Regain Footing")
	}
	if ban := findCondition(actor, CondNoAttacks); ban != nil && (act.Name == "Attack" || act.Name == "Charge" || act.Name == "Cast Magic/Use Power") {
		cannot(fmt.Sprintf("Serious Wound: no attacks or spellcasting for %d more Rounds", ban.Rounds))
	}

	if len(blockers) == 0 {
		if idx, err := e.indexOf(actor); err == nil {
			if have, need := safeAP(e, idx), act.AP; have < need {
				blockers = append(blockers, actBlocker{
					fmt.Sprintf("%s has %d Action Points; %s costs %d", idOf(actor), have, act.Name, need),
					ErrNotEnoughAP,
				})
			}
		}
	}
	return act, blockers, nil
}

// CanAct reports whether the actor may perform the named action in the
// encounter right now, and if not, every blocker as a human-readable reason
// ("X is prone: crawl ... or stand with Regain Footing"). The action name
// must appear in the Combat Actions table; unknown actions and encounters
// that are not running come back as blockers with the sentinel's own
// message. An empty second result with true means the action is legal —
// call SpendAP to pay for it.
func CanAct(e *Encounter, actor *char.Character, action string) (bool, []string) {
	_, blockers, err := actBlockers(e, actor, action)
	if err != nil {
		return false, []string{err.Error()}
	}
	if len(blockers) == 0 {
		return true, nil
	}
	msgs := make([]string, len(blockers))
	for i, b := range blockers {
		msgs[i] = b.msg
	}
	return false, msgs
}

// SpendAP validates and pays for one Combat Action: the name must appear in
// the table, the encounter must be running, the actor must be a participant
// able to act, and their remaining Action Points must cover the cost — only
// then are the points deducted. Errors wrap [ErrUnknownAction],
// [ErrNotRunning], [ErrNotInEncounter], [ErrCannotAct] (message names the
// state reason) or [ErrNotEnoughAP].
func SpendAP(e *Encounter, actor *char.Character, action string) error {
	act, blockers, err := actBlockers(e, actor, action)
	if err != nil {
		return err
	}
	if len(blockers) > 0 {
		return fmt.Errorf("%w: %s", blockers[0].sentinel, blockers[0].msg)
	}
	return e.spend(actor, act.AP)
}

// findCondition returns the first condition with the given name, or nil.
func findCondition(c *char.Character, name string) *char.Condition {
	if c == nil {
		return nil
	}
	for i := range c.Conditions {
		if c.Conditions[i].Name == name {
			return &c.Conditions[i]
		}
	}
	return nil
}

// locationClass buckets a hit-location name for the wound and stun tables,
// which distinguish limbs from Abdomen/Chest/Head (tables.md §3.9). It
// mirrors char's own (unexported) classifier: creature stat-block names
// that read as torso map to torso, "Head" to head, everything else to limb.
// An empty name is treated as whole-body (limb bucket with no location).
func locationClass(name string) locClass {
	switch strings.ToLower(name) {
	case "head":
		return locHead
	case "chest", "abdomen", "thorax", "hindquarters", "forequarters":
		return locTorso
	case "":
		return locWholeBody
	}
	return locLimb
}

// isLegLocation reports whether a hit location is a leg (or analogous
// lower limb — creature tables print Hind Leg, Rear Leg, and so on). Legs
// block movement when stunned; every other limb conservatively blocks
// Attack, because the engine cannot know which limb wields the weapon.
func isLegLocation(name string) bool {
	return strings.Contains(strings.ToLower(name), "leg")
}

// locClass is the limb/torso/head bucketing of hit locations.
type locClass int

const (
	locLimb locClass = iota
	locTorso
	locHead
	locWholeBody
)

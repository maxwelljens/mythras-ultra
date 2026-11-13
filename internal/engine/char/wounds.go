package char

import (
	"errors"
	"fmt"
	"strings"
)

// Severity is a wound level from the Damage and Wound Levels table (Game
// System › Damage and Wound Levels, tables.md §3.9).
type Severity int

const (
	// SeverityNone means no wound was applied (damage was 0 or less).
	SeverityNone Severity = iota
	// SeverityMinor: the location still has positive Hit Points.
	SeverityMinor
	// SeveritySerious: the location is at 0 Hit Points or below.
	SeveritySerious
	// SeverityMajor: the location is at a negative score equal to or
	// greater than its starting Hit Points.
	SeverityMajor
)

// String renders the wound level name.
func (s Severity) String() string {
	switch s {
	case SeverityNone:
		return "None"
	case SeverityMinor:
		return "Minor Wound"
	case SeveritySerious:
		return "Serious Wound"
	case SeverityMajor:
		return "Major Wound"
	}
	return fmt.Sprintf("Severity(%d)", int(s))
}

// Wound is one applied injury in the character's injury log: where the blow
// landed, how much damage it dealt, and the severity at the moment of
// injury. Current state is read from CurrentHP / SeverityAt — the log is
// history, not live state.
type Wound struct {
	// Location is the hit location that was wounded.
	Location string `json:"location"`
	// Damage is the damage the wounding hit dealt after all reductions.
	Damage int `json:"damage"`
	// Severity is the wound level at the moment of injury.
	Severity Severity `json:"severity"`
}

// locationClass buckets a hit location for the wound-effect tables, which
// distinguish limbs from Abdomen/Chest/Head (tables.md §3.9).
type locationClass int

const (
	locLimb locationClass = iota
	locTorso
	locHead
)

// classifyLocation maps a catalog location name onto the wound table's
// limb/torso/head classes. Humanoid names map directly; creature catalogs
// use stat-block names the rulebook never classifies, so recognised torso
// words map to torso, "Head" to head, and anything else is treated as a
// limb — the less immediately lethal reading.
// [AMBIGUOUS: the rulebook gives wound tables only in terms of Limb vs
// Abdomen/Chest/Head; classification of creature stat-block locations
// (Hindquarters, Forequarters, Thorax, Tail, Wings, Fins, ...) is the
// engine's conservative mapping — Game System › Damage and Wound Levels.]
func classifyLocation(name string) locationClass {
	switch strings.ToLower(name) {
	case "head":
		return locHead
	case "chest", "abdomen", "thorax", "hindquarters", "forequarters":
		return locTorso
	}
	return locLimb
}

// severityFor evaluates the wound thresholds (tables.md §3.9): current >
// 0 → Minor; 0 or below → Serious; at or below −(starting HP) → Major.
func severityFor(current, starting int) Severity {
	switch {
	case current > 0:
		return SeverityMinor
	case starting > 0 && current > -starting:
		return SeveritySerious
	default:
		return SeverityMajor
	}
}

// SeverityAt returns the wound level currently in effect at the named hit
// location (the digest's thresholds, recomputed from live HP). Errors wrap
// [ErrInvalidLocation] or whatever Derive returns.
func (c *Character) SeverityAt(location string) (Severity, error) {
	if c == nil {
		return SeverityNone, errors.New("char: nil character")
	}
	d, err := Derive(c)
	if err != nil {
		return SeverityNone, err
	}
	starting, ok := d.LocationHP[location]
	if !ok {
		return SeverityNone, fmt.Errorf("%w: %q", ErrInvalidLocation, location)
	}
	current, ok := c.CurrentHP[location]
	if !ok {
		current = starting
	}
	return severityFor(current, starting), nil
}

// ApplyWound subtracts dmg from the named hit location's current Hit Points
// and returns the resulting wound level plus the effect strings the rules
// attach to it. This is wound bookkeeping only: the damage-application
// pipeline (Damage Modifier, magic, parry reduction, Armor Points) and the
// opposed Endurance rolls whose results the effects announce as "pending"
// are engine/combat's job, executed before calling ApplyWound.
//
// Per the rules (tables.md §3.9): damage ≤ 0 inflicts no wound (SeverityNone,
// nothing changes); a Major Wound immediately imposes Prone and
// Incapacitated — and Unconscious when torso or head — as Conditions.
// Errors wrap [ErrInvalidAmount] (negative damage), [ErrInvalidLocation]
// (unknown hit location) or whatever Derive returns.
func (c *Character) ApplyWound(location string, dmg int) (Severity, []string, error) {
	if c == nil {
		return SeverityNone, nil, errors.New("char: nil character")
	}
	if dmg < 0 {
		return SeverityNone, nil, fmt.Errorf("%w: damage %d to %q", ErrInvalidAmount, dmg, location)
	}
	d, err := Derive(c)
	if err != nil {
		return SeverityNone, nil, err
	}
	starting, ok := d.LocationHP[location]
	if !ok {
		return SeverityNone, nil, fmt.Errorf("%w: %q", ErrInvalidLocation, location)
	}
	if dmg == 0 {
		// "If damage > 0, subtract it ..." — zero damage is not a wound.
		return SeverityNone, nil, nil
	}
	if c.CurrentHP == nil {
		c.CurrentHP = make(map[string]int, len(d.LocationHP))
	}
	for loc, hp := range d.LocationHP {
		if _, exists := c.CurrentHP[loc]; !exists {
			c.CurrentHP[loc] = hp
		}
	}

	current := c.CurrentHP[location] - dmg
	c.CurrentHP[location] = current

	sev := severityFor(current, starting)
	c.Wounds = append(c.Wounds, Wound{Location: location, Damage: dmg, Severity: sev})
	effects := woundEffects(sev, location, d.HealingRate)

	if sev == SeverityMajor {
		// Major Wound: "victim immediately Incapacitated, drops prone";
		// torso/head additionally "drop unconscious".
		c.AddCondition(Condition{Name: CondProne})
		c.AddCondition(Condition{Name: CondIncapacitated})
		if classifyLocation(location) != locLimb {
			c.AddCondition(Condition{Name: CondUnconscious})
		}
	}
	return sev, effects, nil
}

// woundEffects returns the rulebook's effect strings for a wound. The
// opposed-Endurance consequences are announced as pending — combat resolves
// those rolls; char only records what is unconditional.
func woundEffects(sev Severity, location string, healingRate int) []string {
	if sev != SeveritySerious && sev != SeverityMajor {
		return nil
	}
	// [AMBIGUOUS: the source prints "the next 1d3 Turns"; a Turn is one
	// participant's action slot within a Round (Combat › Combat Rounds and
	// Initiative), so context implies Rounds — digest §9.5. The engine
	// states Rounds.]
	const seriousAttackBan = "Serious Wound: no attacks or spellcasting for 1d3 Rounds (Parry and Evade still allowed)."

	switch sev {
	case SeveritySerious:
		if classifyLocation(location) == locLimb {
			return []string{
				seriousAttackBan,
				"limb useless until the location returns to positive HP (leg: drops prone; arm: drops held item unless strapped)",
				"pending opposed Endurance vs. the attack roll (combat): on failure, apply the limb effect above",
			}
		}
		return []string{
			seriousAttackBan,
			"pending opposed Endurance vs. the attack roll (combat): on failure, unconscious for minutes equal to the damage dealt",
		}
	case SeverityMajor:
		if classifyLocation(location) == locLimb {
			return []string{
				"Major Wound: immediately Incapacitated, drops prone",
				fmt.Sprintf("limb severed, transfixed, shattered or ripped off; untreated: death after %d minutes (= 5 × Healing Rate)", 5*healingRate),
				"pending opposed Endurance vs. the attack roll (combat): on failure, unconscious",
			}
		}
		return []string{
			"Major Wound: immediately Incapacitated, drops prone",
			"unconscious, totally Incapacitated",
			"pending opposed Endurance vs. the attack roll (combat): on failure, instant death",
			fmt.Sprintf("survivors die unless treated within %d Combat Rounds (= 2 × Healing Rate)", 2*healingRate),
		}
	}
	return nil
}

// Heal restores n Hit Points to the named hit location, clamped to the
// location's starting Hit Points (a location cannot heal above its
// starting value). Healing bookkeeping only: which healing action is
// available (First Aid's once-per-injury 1d3, the per-day/week/month
// rates of the Healing from Injury table, the successful Healing roll a
// Major Wound needs) is decided by the rules/combat layer, which then calls
// Heal with the number of points to restore.
//
// Errors wrap [ErrInvalidAmount] (negative n) or [ErrInvalidLocation].
func (c *Character) Heal(location string, n int) error {
	if c == nil {
		return errors.New("char: nil character")
	}
	if n < 0 {
		return fmt.Errorf("%w: heal %d to %q", ErrInvalidAmount, n, location)
	}
	d, err := Derive(c)
	if err != nil {
		return err
	}
	starting, ok := d.LocationHP[location]
	if !ok {
		return fmt.Errorf("%w: %q", ErrInvalidLocation, location)
	}
	if c.CurrentHP == nil {
		c.CurrentHP = make(map[string]int, len(d.LocationHP))
	}
	for loc, hp := range d.LocationHP {
		if _, exists := c.CurrentHP[loc]; !exists {
			c.CurrentHP[loc] = hp
		}
	}
	current := c.CurrentHP[location] + n
	if current > starting {
		current = starting
	}
	c.CurrentHP[location] = current
	return nil
}

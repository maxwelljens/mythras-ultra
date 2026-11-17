package combat

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
	"github.com/maxwelljens/mythras-ultra/internal/engine/rules"
)

// Sentinels returned by the damage and healing operations.
var (
	// ErrUnknownHealMethod reports a method name Heal does not print:
	// "first aid", "treat", "day", "week" or "month".
	ErrUnknownHealMethod = errors.New("combat: unknown healing method")
	// ErrHealUnavailable reports a healing action the rules refuse right
	// now: First Aid on a wound that already received it, or natural
	// Major-Wound healing before a successful Healing roll ("treat").
	ErrHealUnavailable = errors.New("combat: healing not available")
)

// Healing method names accepted by Heal (Game System › Healing from
// Injury, L1210-1225; tables.md §3.10; First Aid skill L636).
const (
	// HealFirstAid heals 1d3 Hit Points at one injury, once per specific
	// injury, and staunches an actively bleeding wound on a successful
	// First Aid roll.
	HealFirstAid = "first aid"
	// HealTreat is the successful Healing roll a Major Wound needs before
	// natural healing may start ("treated with a successful Healing roll").
	HealTreat = "treat"
	// HealDay heals every Minor (still positive) location by the Healing
	// Rate, and recovers one Fatigue level once bleeding has stopped.
	HealDay = "day"
	// HealWeek heals every still-Serious location (zero HP or below) by
	// the Healing Rate per week.
	HealWeek = "week"
	// HealMonth heals treated Major Wounds by the Healing Rate per month.
	HealMonth = "month"
	// CondMajorTreated records the successful Healing roll that unblocks
	// natural Major-Wound healing (Healing from Injury).
	CondMajorTreated = "Major Wound Treated"
)

// LocationHeal is one location's healing inside a HealResult.
type LocationHeal struct {
	// Location is the hit location that recovered.
	Location string `json:"location"`
	// Before and After are the location's Hit Points around the healing.
	Before int `json:"before"`
	After  int `json:"after"`
	// Amount is the Hit Points restored.
	Amount int `json:"amount"`
}

// HealResult is the audit of one Heal call: the method, the First Aid or
// Healing roll when one was made, each location that recovered, and
// present-tense Facts for the GM.
type HealResult struct {
	// Method echoes the normalised method name that ran.
	Method string `json:"method"`
	// Rolled reports whether a skill roll was made; SkillRoll and Grade
	// carry it (First Aid to staunch, Healing to treat).
	Rolled    bool        `json:"rolled,omitempty"`
	SkillRoll int         `json:"skillRoll,omitempty"`
	Grade     rules.Grade `json:"grade,omitempty"`
	// Healed lists every location that gained Hit Points.
	Healed []LocationHeal `json:"healed,omitempty"`
	// Total is the Hit Points restored across all locations.
	Total int `json:"total"`
	// FatigueRecovered is the Fatigue level recovered by the Blood Loss
	// rule (1 per day once bleeding has stopped).
	FatigueRecovered int `json:"fatigueRecovered,omitempty"`
	// Facts are human-readable, present-tense audit lines for the GM.
	Facts []string `json:"facts,omitempty"`
}

// Heal runs one healing action on a character:
//
//   - "first aid": 1d3 at the most wounded untreated location (once per
//     injury — First Aid skill L636), plus a First Aid roll to staunch an
//     active bleed (Special Effect Descriptions › Bleed);
//   - "treat": the Healing roll that unblocks natural Major-Wound healing;
//   - "day" / "week" / "month": natural healing at the Healing Rate for
//     Minor / Serious / Major wounds respectively (Healing from Injury).
//
// Digest restrictions the engine does not model: strenuous activity
// reducing the Healing Rate by 1d3 (the GM passes a lower-rank period
// instead), and maiming of Major Wounds untreated for days equal to 1/20th
// of the Healing skill — [AMBIGUOUS: the source never states how many Hit
// Points the maiming removes, and char has no field for a permanent
// starting-HP reduction, so the engine announces the deadline as a fact
// and leaves the maim to the GM]. Errors wrap [ErrUnknownHealMethod],
// [ErrHealUnavailable], or whatever the dice, skill and wound operations
// return.
func Heal(r *dice.Rand, c *char.Character, method string) (HealResult, error) {
	if r == nil {
		return HealResult{}, errors.New("combat: nil rand")
	}
	if c == nil {
		return HealResult{}, errors.New("combat: nil character")
	}
	m := strings.ToLower(strings.TrimSpace(method))
	switch m {
	case HealFirstAid, "firstaid":
		m = HealFirstAid
	case HealTreat, "healing":
		m = HealTreat
	case HealDay, "natural", "natural day":
		m = HealDay
	case HealWeek, "natural week":
		m = HealWeek
	case HealMonth, "natural month":
		m = HealMonth
	default:
		return HealResult{}, fmt.Errorf("%w: %q (want %q, %q, %q, %q or %q)",
			ErrUnknownHealMethod, method, HealFirstAid, HealTreat, HealDay, HealWeek, HealMonth)
	}
	res := HealResult{Method: m}
	d, err := char.Derive(c)
	if err != nil {
		return res, fmt.Errorf("combat: derive %s: %w", idOf(c), err)
	}
	switch m {
	case HealFirstAid:
		err = healFirstAid(r, c, d, &res)
	case HealTreat:
		err = healTreat(r, c, &res)
	case HealDay:
		err = healNatural(c, d, &res, char.SeverityMinor)
		// Blood Loss recovers one Fatigue level per day, starting the day
		// after the bleeding stops (Game System › Blood Loss).
		if !c.HasCondition(char.CondBleeding) {
			if n := recoverFatigueLevel(c); n > 0 {
				res.FatigueRecovered = n
				res.Facts = append(res.Facts, fmt.Sprintf("blood loss receded: %d Fatigue level recovered.", n))
			}
		} else {
			res.Facts = append(res.Facts, "the wound still bleeds: no Fatigue recovery today.")
		}
	case HealWeek:
		err = healNatural(c, d, &res, char.SeveritySerious)
	case HealMonth:
		err = healNatural(c, d, &res, char.SeverityMajor)
	}
	if err != nil {
		return res, err
	}
	return res, nil
}

// healFirstAid runs the First Aid action: staunch an active bleed (needs
// a First Aid roll) and heal 1d3 at the most wounded location that has
// not yet received First Aid.
func healFirstAid(r *dice.Rand, c *char.Character, d char.Derived, res *HealResult) error {
	if cond := findCondition(c, char.CondBleeding); cond != nil {
		sk, err := rules.SkillValue(c, "First Aid")
		if err != nil {
			return fmt.Errorf("combat: first aid: %w", err)
		}
		outcome, _, err := rules.SkillTest(r, sk, rules.DifficultyStandard, false)
		if err != nil {
			return fmt.Errorf("combat: first aid roll: %w", err)
		}
		res.Rolled = true
		res.SkillRoll = outcome.Roll
		res.Grade = outcome.Grade
		if outcome.Grade >= rules.GradeSuccess {
			c.RemoveCondition(char.CondBleeding)
			res.Facts = append(res.Facts, "a successful First Aid roll staunches the bleeding (strenuous action reopens the wound).")
		} else {
			res.Facts = append(res.Facts, "the First Aid roll fails: the wound still bleeds.")
		}
	}
	// Once per specific injury: pick the most wounded untreated location.
	best := ""
	bestCurrent := 0
	for _, loc := range d.Locations {
		cur, ok := c.CurrentHP[loc.Name]
		if !ok {
			cur = d.LocationHP[loc.Name]
		}
		starting := d.LocationHP[loc.Name]
		if cur >= starting {
			continue // not wounded
		}
		if findConditionAt(c, CondFirstAid, loc.Name) != nil {
			continue // this injury already treated
		}
		if best == "" || cur < bestCurrent {
			best, bestCurrent = loc.Name, cur
		}
	}
	if best == "" {
		return fmt.Errorf("%w: %s has no untreated injury for First Aid", ErrHealUnavailable, idOf(c))
	}
	heal, err := r.Roll("1d3")
	if err != nil {
		return fmt.Errorf("combat: first aid heal: %w", err)
	}
	if err := applyHeal(c, res, best, heal.Total); err != nil {
		return err
	}
	c.AddCondition(char.Condition{Name: CondFirstAid, Location: best, Notes: "once per injury"})
	res.Facts = append(res.Facts, fmt.Sprintf("First Aid restores %d Hit Points to the %s.", heal.Total, best))
	return nil
}

// healTreat runs the Healing roll that lets a Major Wound start healing.
func healTreat(r *dice.Rand, c *char.Character, res *HealResult) error {
	sk, err := rules.SkillValue(c, "Healing")
	if err != nil {
		return fmt.Errorf("combat: treat: %w", err)
	}
	outcome, _, err := rules.SkillTest(r, sk, rules.DifficultyStandard, false)
	if err != nil {
		return fmt.Errorf("combat: healing roll: %w", err)
	}
	res.Rolled = true
	res.SkillRoll = outcome.Roll
	res.Grade = outcome.Grade
	if outcome.Grade >= rules.GradeSuccess {
		c.AddCondition(char.Condition{Name: CondMajorTreated})
		res.Facts = append(res.Facts, "the Healing roll succeeds: the Major Wound is treated and may now heal naturally.")
		return nil
	}
	res.Facts = append(res.Facts, "the Healing roll fails: the Major Wound cannot heal naturally yet.")
	return nil
}

// healNatural applies one period of natural healing: every wound of the
// requested severity gains Healing Rate Hit Points (Healing from Injury).
// Major Wounds are refused until CondMajorTreated records a successful
// Healing roll, and each untreated Major location announces the maiming
// deadline as a fact.
func healNatural(c *char.Character, d char.Derived, res *HealResult, sev char.Severity) error {
	for _, loc := range d.Locations {
		cur, ok := c.CurrentHP[loc.Name]
		if !ok {
			cur = d.LocationHP[loc.Name]
		}
		starting := d.LocationHP[loc.Name]
		if cur >= starting {
			continue // nothing to heal at this location
		}
		got, err := c.SeverityAt(loc.Name)
		if err != nil {
			return fmt.Errorf("combat: natural healing: %w", err)
		}
		if got != sev {
			continue
		}
		if sev == char.SeverityMajor && !c.HasCondition(CondMajorTreated) {
			res.Facts = append(res.Facts, ("the Major Wound at the " + loc.Name + " heals no further until a successful Healing roll treats it."))
			if sk, err := rules.SkillValue(c, "Healing"); err == nil && sk > 0 {
				days := (sk + 19) / 20 // one twentieth of the Healing skill
				res.Facts = append(res.Facts, fmt.Sprintf("GM: untreated the %s becomes maimed after %d days (permanent Hit Point loss).", loc.Name, days))
			}
			continue
		}
		if err := applyHeal(c, res, loc.Name, d.HealingRate); err != nil {
			return err
		}
	}
	if res.Total == 0 && len(res.Facts) == 0 {
		res.Facts = append(res.Facts, fmt.Sprintf("no %s wound is ready to heal this period.", severityName(sev)))
	}
	return nil
}

// severityName renders the wound level a healing period targets.
func severityName(sev char.Severity) string { return strings.TrimSuffix(sev.String(), " Wound") }

// applyHeal restores amount Hit Points at location through char.Heal
// (clamped at the starting value) and records it in the result.
func applyHeal(c *char.Character, res *HealResult, location string, amount int) error {
	if amount <= 0 {
		res.Facts = append(res.Facts, "the Healing Rate is zero: no Hit Points are restored.")
		return nil
	}
	before, ok := c.CurrentHP[location]
	if !ok {
		d, err := char.Derive(c)
		if err != nil {
			return fmt.Errorf("combat: derive %s: %w", idOf(c), err)
		}
		before = d.LocationHP[location]
	}
	if err := c.Heal(location, amount); err != nil {
		return fmt.Errorf("combat: heal %s: %w", location, err)
	}
	after := c.CurrentHP[location]
	restored := after - before
	if restored <= 0 {
		return nil
	}
	res.Healed = append(res.Healed, LocationHeal{Location: location, Before: before, After: after, Amount: restored})
	res.Total += restored
	return nil
}

// findConditionAt returns the named condition scoped to one hit location.
func findConditionAt(c *char.Character, name, location string) *char.Condition {
	if c == nil {
		return nil
	}
	for i := range c.Conditions {
		if c.Conditions[i].Name == name && c.Conditions[i].Location == location {
			return &c.Conditions[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// manual damage
// ---------------------------------------------------------------------------

// DamageResult is the audit of one ApplyDamage call: the location rolled,
// the armour that absorbed, what reached the Hit Points, the wound level
// and its unconditional effects.
type DamageResult struct {
	// Location is the hit location damaged (rolled when not given).
	Location string `json:"location,omitempty"`
	// LocationRoll is the 1d20 roll when the engine picked the location.
	LocationRoll dice.Roll `json:"locationRoll,omitempty"`
	// Damage is the raw damage offered (the caller applies the Damage
	// Modifier first, per the damage application order).
	Damage int `json:"damage"`
	// ArmorPoints is the protection at the location, ArmorAbsorbed what
	// it stopped, DamageApplied what reached the Hit Points.
	ArmorPoints   int `json:"armorPoints"`
	ArmorAbsorbed int `json:"armorAbsorbed"`
	DamageApplied int `json:"damageApplied"`
	// Severity and WoundEffects come from char.ApplyWound; NoAttacksRoll
	// is the Serious-Wound 1d3 Rounds ban.
	Severity      char.Severity `json:"severity"`
	WoundEffects  []string      `json:"woundEffects,omitempty"`
	NoAttacksRoll dice.Roll     `json:"noAttacksRoll,omitempty"`
	// Facts are human-readable, present-tense audit lines for the GM.
	Facts []string `json:"facts,omitempty"`
}

// ApplyDamage applies raw damage to a character outside the attack
// sequence — fire, falling, breath weapons, collapsing masonry — through
// the damage application order the digest prints for steps that exist
// without an attacker: armour absorbs first, the rest wounds through
// char.ApplyWound, and the Serious-Wound 1d3-Round attack ban is rolled
// (tables.md §3.9, "Damage application order"; steps 1-2 — Damage Modifier
// and magic — are the caller's to apply before calling, step 3 does not
// apply without a parry).
//
// The Endurance-opposed wound consequences are deliberately not rolled
// here: the rules oppose them against "the enemy's attack roll", which
// environmental damage does not have
// [AMBIGUOUS: Game System › Damage and Wound Levels never names the
// resistance target for non-attack damage — the GM rolls or narrates
// those consequences]. Errors wrap [char.ErrInvalidAmount] (negative
// damage), [ErrInvalidChoice] (unknown location) or the wound operations'.
func ApplyDamage(r *dice.Rand, victim *char.Character, location string, dmg int) (DamageResult, error) {
	res := DamageResult{Damage: dmg}
	if r == nil {
		return res, errors.New("combat: nil rand")
	}
	if victim == nil {
		return res, errors.New("combat: nil character")
	}
	if dmg < 0 {
		return res, fmt.Errorf("combat: apply damage %d to %q: %w", dmg, location, char.ErrInvalidAmount)
	}
	if location == "" {
		roll, err := r.Roll("1d20")
		if err != nil {
			return res, fmt.Errorf("combat: hit location: %w", err)
		}
		res.LocationRoll = roll
		loc, err := rollLocation(victim, roll.Total)
		if err != nil {
			return res, err
		}
		res.Location = loc
	} else {
		d, err := char.Derive(victim)
		if err != nil {
			return res, fmt.Errorf("combat: derive %s: %w", idOf(victim), err)
		}
		if _, ok := d.LocationHP[location]; !ok {
			return res, fmt.Errorf("%w: %q is not a hit location of %s", ErrInvalidChoice, location, idOf(victim))
		}
		res.Location = location
	}
	comps, err := armorComponents(victim, res.Location)
	if err != nil {
		return res, err
	}
	for _, c := range comps {
		res.ArmorPoints += c.AP
	}
	absorbed := res.ArmorPoints
	if dmg < absorbed {
		absorbed = dmg
	}
	res.ArmorAbsorbed = absorbed
	applied := dmg - absorbed
	if applied < 0 {
		applied = 0
	}
	res.DamageApplied = applied
	if applied == 0 {
		res.Facts = append(res.Facts, fmt.Sprintf("the armour absorbs all %d damage at the %s.", absorbed, res.Location))
		return res, nil
	}
	sev, effects, err := victim.ApplyWound(res.Location, applied)
	if err != nil {
		return res, fmt.Errorf("combat: apply wound: %w", err)
	}
	res.Severity = sev
	res.WoundEffects = effects
	removeConditionAt(victim, CondFirstAid, res.Location)
	res.Facts = append(res.Facts, fmt.Sprintf("%s takes %d damage to the %s (%s).", idOf(victim), applied, res.Location, sev))
	if sev == char.SeveritySerious {
		ban, err := r.Roll("1d3")
		if err != nil {
			return res, fmt.Errorf("combat: serious wound ban: %w", err)
		}
		res.NoAttacksRoll = ban
		victim.AddCondition(char.Condition{Name: CondNoAttacks, Rounds: ban.Total})
		res.Facts = append(res.Facts, fmt.Sprintf("Serious Wound: no attacks or spellcasting for %d Rounds.", ban.Total))
	}
	return res, nil
}

// ---------------------------------------------------------------------------
// bleeding and Impale withdrawal
// ---------------------------------------------------------------------------

// addFatigueLevels adds n Fatigue levels to the character's
// rules.FatigueCondition and returns the new total. The "levels: N" note
// format is the seam's documented persistence slot for exactly this kind
// of accrual (rules/fatigue.go — Blood Loss is named as a future source);
// combat never touches the condition's Rounds field.
func addFatigueLevels(c *char.Character, n int) (int, error) {
	levels := 0
	for _, cond := range c.Conditions {
		if cond.Name != rules.FatigueCondition {
			continue
		}
		rest, ok := strings.CutPrefix(cond.Notes, "levels: ")
		if !ok {
			return 0, fmt.Errorf("%w: notes %q", rules.ErrInvalidFatigue, cond.Notes)
		}
		v, err := strconv.Atoi(rest)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("%w: notes %q", rules.ErrInvalidFatigue, cond.Notes)
		}
		levels = v
	}
	levels += n
	c.AddCondition(char.Condition{Name: rules.FatigueCondition, Notes: "levels: " + strconv.Itoa(levels)})
	return levels, nil
}

// recoverFatigueLevel removes one Fatigue level (the Blood Loss daily
// recovery) and reports how many it removed.
func recoverFatigueLevel(c *char.Character) int {
	for _, cond := range c.Conditions {
		if cond.Name != rules.FatigueCondition {
			continue
		}
		rest, ok := strings.CutPrefix(cond.Notes, "levels: ")
		if !ok {
			return 0
		}
		v, err := strconv.Atoi(rest)
		if err != nil || v <= 0 {
			return 0
		}
		if v == 1 {
			c.RemoveCondition(rules.FatigueCondition)
			return 1
		}
		c.AddCondition(char.Condition{Name: rules.FatigueCondition, Notes: "levels: " + strconv.Itoa(v-1)})
		return 1
	}
	return 0
}

// accrueBleeding charges one Fatigue level for a bleeding character at the
// start of a Combat Round ("At the start of each Combat Round, the
// recipient loses one level of Fatigue, until they collapse and possibly
// die" — Special Effect Descriptions › Bleed) and returns the new total.
func accrueBleeding(c *char.Character) (int, error) {
	if !c.HasCondition(char.CondBleeding) {
		return 0, nil
	}
	return addFatigueLevels(c, 1)
}

// ImpaleWithdrawResult is the audit of one withdrawal attempt.
type ImpaleWithdrawResult struct {
	// Round, Weapon and Location locate the attempt.
	Round    int    `json:"round"`
	Weapon   string `json:"weapon"`
	Location string `json:"location"`
	// Resisted reports that the carrier opposed the withdrawal, and
	// Opposed carries that Brawn contest.
	Resisted bool                `json:"resisted,omitempty"`
	Opposed  rules.OpposedResult `json:"opposed,omitempty"`
	// Success reports the weapon came free.
	Success bool `json:"success"`
	// DamageRoll is the withdrawal damage (half the normal roll, or the
	// full roll for a barbed weapon), DamageApplied what it dealt —
	// armour does not reduce it and no Damage Modifier applies.
	DamageRoll    dice.Roll     `json:"damageRoll,omitempty"`
	DamageApplied int           `json:"damageApplied,omitempty"`
	Severity      char.Severity `json:"severity,omitempty"`
	// Facts are human-readable, present-tense audit lines for the GM.
	Facts []string `json:"facts,omitempty"`
}

// WithdrawImpaled resolves pulling an impaled weapon free: the Ready
// Weapon action, an unopposed Brawn roll (opposed when the victim
// resists). Success inflicts extra damage to the same location equal to
// half the normal damage roll — no Damage Modifier, armour does not
// reduce it; barbed weapons inflict the normal damage. Failure leaves the
// weapon stuck (Special Effect Descriptions › Impale; tables.md §4.7).
// The weapon is att's, lodged in def; both Impaled conditions clear on
// success. Errors wrap [ErrNotInEncounter] when either character is
// outside e, or [ErrEffectUnavailable] when the weapon is not impaled.
func WithdrawImpaled(r *dice.Rand, e *Encounter, att, def *char.Character, weapon string, resisted bool) (ImpaleWithdrawResult, error) {
	res := ImpaleWithdrawResult{Weapon: weapon, Resisted: resisted}
	if r == nil {
		return res, errors.New("combat: nil rand")
	}
	if err := e.running(); err != nil {
		return res, err
	}
	if att == nil || def == nil {
		return res, errors.New("combat: nil combatant")
	}
	if _, err := e.indexOf(att); err != nil {
		return res, err
	}
	if _, err := e.indexOf(def); err != nil {
		return res, err
	}
	res.Round = e.Round
	held := impaledHolds(att, weapon)
	var impaled *char.Condition
	for i := range def.Conditions {
		cond := &def.Conditions[i]
		if cond.Name == CondImpaled && (strings.HasPrefix(cond.Notes, "weapon="+weapon+";") || cond.Notes == "weapon="+weapon) {
			impaled = cond
			break
		}
	}
	if !held || impaled == nil {
		return res, fmt.Errorf("%w: %s is not impaled in %s", ErrEffectUnavailable, weapon, idOf(def))
	}
	res.Location = impaled.Location

	// The wielder passes an unopposed Brawn roll, or wins an opposed one
	// when the victim resists.
	skill, ok := att.Skill("Brawn")
	if !ok {
		return res, fmt.Errorf("combat: withdraw impaled: %w: Brawn", char.ErrUnknownSkill)
	}
	roll, err := r.Roll("1d100")
	if err != nil {
		return res, fmt.Errorf("combat: withdrawal roll: %w", err)
	}
	g, err := rules.Classify(roll.Total, skill.Value)
	if err != nil {
		return res, fmt.Errorf("combat: classify withdrawal: %w", err)
	}
	out := rules.Outcome{Roll: roll.Total, Skill: skill.Value, Grade: g}
	facts := []string{fmt.Sprintf("Brawn %s to withdraw the %s.", out, weapon)}
	if resisted {
		vSkill, ok := def.Skill("Brawn")
		if !ok {
			return res, fmt.Errorf("combat: withdraw impaled: %w: Brawn", char.ErrUnknownSkill)
		}
		vRoll, err := r.Roll("1d100")
		if err != nil {
			return res, fmt.Errorf("combat: withdrawal resistance: %w", err)
		}
		vg, err := rules.Classify(vRoll.Total, vSkill.Value)
		if err != nil {
			return res, fmt.Errorf("combat: classify withdrawal resistance: %w", err)
		}
		vOut := rules.Outcome{Roll: vRoll.Total, Skill: vSkill.Value, Grade: vg}
		res.Opposed = rules.Opposed(out, vOut)
		res.Success = res.Opposed.Winner == 1
		facts = append(facts, fmt.Sprintf("opposed by %s: %s.", idOf(def), vOut))
	} else {
		res.Success = g >= rules.GradeSuccess
	}
	res.Facts = append(res.Facts, facts...)
	if !res.Success {
		res.Facts = append(res.Facts, "the weapon stays stuck in the wound; it may be tried again next Turn.")
		return res, nil
	}

	// Withdrawal damage: half the normal roll, rounded up, no Damage
	// Modifier, armour does not reduce it (barbed: the full roll).
	w, err := resolveWeapon(att, weapon)
	if err != nil {
		return res, err
	}
	expr := w.damage
	if expr == "" {
		expr = "1d3"
	}
	dmg, err := r.Roll(expr)
	if err != nil {
		return res, fmt.Errorf("combat: withdrawal damage %q: %w", expr, err)
	}
	res.DamageRoll = dmg
	if strings.Contains(strings.ToLower(w.notes), "barbed") {
		res.Facts = append(res.Facts, fmt.Sprintf("the barbed %s inflicts the full %d damage.", weapon, dmg.Total))
	} else {
		half := ceilInt(dmg.Total, 2)
		dmg = dice.Roll{Expr: "half of " + dmg.Expr, Dice: dmg.Dice, Modifier: dmg.Modifier, Total: half}
		res.DamageRoll = dmg
		res.Facts = append(res.Facts, fmt.Sprintf("the withdrawal deals half the normal roll: %d damage, armour does not reduce it.", half))
	}
	applied := res.DamageRoll.Total
	if applied <= 0 {
		res.Facts = append(res.Facts, "the withdrawal deals no damage.")
	} else {
		sev, _, err := def.ApplyWound(res.Location, applied)
		if err != nil {
			return res, fmt.Errorf("combat: withdrawal wound: %w", err)
		}
		res.DamageApplied = applied
		res.Severity = sev
		res.Facts = append(res.Facts, fmt.Sprintf("%s takes %d extra damage to the %s (%s).", idOf(def), applied, res.Location, sev))
	}
	// The weapon comes free: both Impaled records clear.
	att.RemoveCondition(CondImpaled)
	removeImpaledAt(def, weapon)
	res.Facts = append(res.Facts, fmt.Sprintf("%s pulls the %s free.", idOf(att), weapon))
	return res, nil
}

// removeImpaledAt drops a victim's Impaled record for one weapon.
func removeImpaledAt(c *char.Character, weapon string) {
	kept := c.Conditions[:0]
	for _, cond := range c.Conditions {
		if cond.Name == CondImpaled && strings.HasPrefix(cond.Notes, "weapon="+weapon+";") {
			continue
		}
		kept = append(kept, cond)
	}
	c.Conditions = kept
}

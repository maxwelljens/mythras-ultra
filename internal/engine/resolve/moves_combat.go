package resolve

import (
	"fmt"
	"slices"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/combat"
)

// StartEncounter opens a combat encounter: participants are registered,
// given round 1's Action Points, and the initiative order is rolled
// (tool: combat_start; combat.Start then combat.RollInitiative — resolve
// runs both before the first turn).
type StartEncounter struct {
	// Participants are the fighting characters' IDs; at least one, all
	// known, all distinct.
	Participants []string `json:"participants"`
}

// Kind implements [Move].
func (m StartEncounter) Kind() string { return "combat_start" }

// Apply resolves every participant ID first (unknown IDs wrap
// [ErrUnknownCharacter] with the known list), then builds and rolls the
// encounter completely off-state and installs it only on success. Starting
// over a running encounter refuses wrapping [ErrEncounterActive]; a stale
// ended encounter is replaced. The previous exchange record is cleared.
func (m StartEncounter) Apply(s *State) (Result, error) {
	if len(m.Participants) == 0 {
		return Result{}, fmt.Errorf("%w: no participants", ErrInvalidArgument)
	}
	people := make([]*char.Character, 0, len(m.Participants))
	seen := make(map[string]bool, len(m.Participants))
	for _, id := range m.Participants {
		if seen[id] { // argument validation before any state check
			return Result{}, fmt.Errorf("%w: %q offered twice", combat.ErrDuplicateParticipant, id)
		}
		seen[id] = true
		c, err := s.character(id)
		if err != nil {
			return Result{}, err
		}
		people = append(people, c)
	}
	if s.Encounter != nil && s.Encounter.Started && !s.Encounter.Ended {
		return Result{}, fmt.Errorf("%w: end the current encounter first", ErrEncounterActive)
	}
	e, err := combat.Start(people)
	if err != nil {
		return Result{}, err
	}
	if err := combat.RollInitiative(s.Rand(), e); err != nil {
		return Result{}, err
	}
	s.Encounter = e
	s.LastAttack = nil

	facts := []string{fmt.Sprintf("encounter started (round %d, %d combatants)", e.Round, len(people))}
	changes := []string{fmt.Sprintf("encounter: none → round %d", e.Round)}
	a := Audit{}
	for place, o := range e.Order {
		if o.Index < 0 || o.Index >= len(e.Participants) {
			continue // Normalize refuses such states; stay safe in-session too
		}
		name := idLabel(e.Participants[o.Index].ID, e.Participants[o.Index].Name)
		line := fmt.Sprintf("%d. %s (initiative %d)", place+1, name, o.Score)
		facts = append(facts, line)
		changes = append(changes, "order: "+line)
		a.addRoll(o.Roll)
	}
	for i, c := range people {
		if i < len(e.AP) {
			facts = append(facts, fmt.Sprintf("%s: %d Action Points", c.ID, e.AP[i]))
		}
	}
	a.addCitation("Mythras Imperative, Combat: Combat Rounds and Initiative")
	summary := fmt.Sprintf("combat begins, round %d: %d combatants, initiative rolled", e.Round, len(people))
	return newResult(summary, facts, a), nil
}

// SpendAP pays for one Combat Action from the actor's round allowance
// (tool: spend_ap): the action table names the cost, and combat refuses
// unknown actions, non-participants, states that prevent the action and
// insufficient Action Points.
type SpendAP struct {
	// Actor is the character spending Action Points (ID).
	Actor string `json:"actor"`
	// Action is the printed Combat Action name ("attack", "parry", ...).
	Action string `json:"action"`
}

// Kind implements [Move].
func (m SpendAP) Kind() string { return "spend_ap" }

// Apply validates the encounter, the actor and the action name before the
// points move, then reports Action Points before → after with the action
// row's own rulebook citation. Errors wrap combat sentinels:
// ErrNotRunning, ErrNotInEncounter, ErrUnknownAction, ErrCannotAct and
// ErrNotEnoughAP.
func (m SpendAP) Apply(s *State) (Result, error) {
	e, err := s.encounter()
	if err != nil {
		return Result{}, err
	}
	c, err := s.character(m.Actor)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(m.Action) == "" {
		return Result{}, fmt.Errorf("%w: empty action name", ErrInvalidArgument)
	}
	action, ok := combat.ActionByName(m.Action)
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", combat.ErrUnknownAction, m.Action)
	}
	idx := participantIndex(e, c)
	if idx < 0 {
		return Result{}, fmt.Errorf("%w: %q", combat.ErrNotInEncounter, m.Actor)
	}
	if idx >= len(e.AP) {
		return Result{}, fmt.Errorf("%w: encounter AP missing for %q", ErrInvalidState, m.Actor)
	}
	before := e.AP[idx]
	if err := combat.SpendAP(e, c, m.Action); err != nil {
		return Result{}, err
	}
	after := e.AP[idx]
	facts := []string{
		fmt.Sprintf("action: %s (%d AP)", action.Name, action.AP),
		change("Action Points", before, after),
	}
	a := Audit{Changes: []string{change(m.Actor+" Action Points", before, after)}}
	a.addCitation(action.Citation)
	summary := fmt.Sprintf("%s spends %d AP on %s (%d left)", c.ID, action.AP, action.Name, after)
	return newResult(summary, facts, a), nil
}

// NextRound advances the encounter to the next Combat Round: Action Points
// refresh to full, round-limited conditions tick and wound death timers run
// (combat.Encounter.NextRound).
type NextRound struct{}

// Kind implements [Move].
func (m NextRound) Kind() string { return "next_round" }

// Apply refreshes the round; a nil encounter errors wrapping
// [ErrNoEncounter], a stopped one wrapping combat.ErrNotRunning.
func (m NextRound) Apply(s *State) (Result, error) {
	e, err := s.encounter()
	if err != nil {
		return Result{}, err
	}
	if !e.Started || e.Ended {
		return Result{}, fmt.Errorf("%w: round cannot advance", combat.ErrNotRunning)
	}
	before := e.Round
	if err := e.NextRound(); err != nil {
		return Result{}, err
	}
	facts := []string{change("round", before, e.Round), "Action Points refreshed for every combatant"}
	a := Audit{Changes: []string{change("round", before, e.Round)}}
	a.addCitation("Mythras Imperative, Combat: Combat Rounds and Initiative")
	summary := fmt.Sprintf("round %d begins", e.Round)
	return newResult(summary, facts, a), nil
}

// CombatEnd closes the running encounter (tool: combat_end). The record
// stays in state with Ended set — CombatStatus can still show it — and the
// recorded attack exchange is cleared so effects cannot target a finished
// fight.
type CombatEnd struct{}

// Kind implements [Move].
func (m CombatEnd) Kind() string { return "combat_end" }

// Apply snapshots the final status for the summary, ends the encounter
// through combat.Encounter.End and drops State.LastAttack.
func (m CombatEnd) Apply(s *State) (Result, error) {
	e, err := s.encounter()
	if err != nil {
		return Result{}, err
	}
	if !e.Started || e.Ended {
		return Result{}, fmt.Errorf("%w: encounter is not running", combat.ErrNotRunning)
	}
	status := e.Status()
	e.End()
	s.LastAttack = nil
	alive, dead := 0, 0
	facts := []string{fmt.Sprintf("encounter ended at round %d", e.Round)}
	for _, p := range status.Participants {
		if p.Dead {
			dead++
		} else {
			alive++
		}
		facts = append(facts, fmt.Sprintf("%s: %s", idLabel(p.ID, p.Name), stateLabel(p.Dead)))
	}
	facts = append(facts, fmt.Sprintf("survivors: %d alive, %d dead", alive, dead))
	a := Audit{Changes: []string{fmt.Sprintf("encounter: running → ended (round %d)", e.Round)}}
	summary := fmt.Sprintf("combat ends at round %d (%d alive, %d dead)", e.Round, alive, dead)
	return newResult(summary, facts, a), nil
}

// ResolveAttack runs one atomic attack exchange: attack roll, the
// defender's chosen reaction, Differential Roll, hit location, damage,
// armour, wound severity and Endurance consequences — spending Action
// Points and ammunition as the rules require (tool: resolve_attack;
// combat.ResolveAttack). Special Effects are NOT applied here: they are a
// separate [ApplySpecialEffect] move against the recorded exchange.
type ResolveAttack struct {
	// Attacker is the attacking character (ID, a participant).
	Attacker string `json:"attacker"`
	// Weapon is the weapon name (inventory item or sample table); empty
	// means an unarmed attack.
	Weapon string `json:"weapon,omitempty"`
	// Defender is the defending character (ID, a participant).
	Defender string `json:"defender"`
	// Reaction is the defender's response — "parry", "dodge" or "none";
	// empty means none (an unwilling or unable defender counts as an
	// automatic Failure, granting special effects on a hit).
	Reaction string `json:"reaction,omitempty"`
}

// Kind implements [Move].
func (m ResolveAttack) Kind() string { return "resolve_attack" }

// Apply validates the encounter and both combatants, normalises the
// reaction, runs combat.ResolveAttack on the session RNG, and records the
// exchange as State.LastAttack for the follow-up effects move.
func (m ResolveAttack) Apply(s *State) (Result, error) {
	e, err := s.encounter()
	if err != nil {
		return Result{}, err
	}
	if !e.Started || e.Ended {
		return Result{}, fmt.Errorf("%w: attack needs a running encounter", combat.ErrNotRunning)
	}
	att, err := s.character(m.Attacker)
	if err != nil {
		return Result{}, err
	}
	def, err := s.character(m.Defender)
	if err != nil {
		return Result{}, err
	}
	reaction := combat.Reaction(strings.ToLower(strings.TrimSpace(m.Reaction)))
	if reaction == "" {
		reaction = combat.ReactionNone
	}
	if !reaction.Valid() {
		return Result{}, fmt.Errorf("%w: %q", combat.ErrUnknownReaction, m.Reaction)
	}
	attIdx := participantIndex(e, att)
	if attIdx < 0 {
		return Result{}, fmt.Errorf("%w: attacker %q", combat.ErrNotInEncounter, m.Attacker)
	}
	apBefore := 0
	if attIdx < len(e.AP) {
		apBefore = e.AP[attIdx]
	}
	res, err := combat.ResolveAttack(s.Rand(), e, att, m.Weapon, def, reaction)
	if err != nil {
		return Result{}, err
	}
	s.LastAttack = &res

	facts := slices.Clone(res.Facts)
	apAfter := apBefore
	if attIdx < len(e.AP) {
		apAfter = e.AP[attIdx]
	}
	if apAfter != apBefore {
		facts = append(facts, change(m.Attacker+" Action Points", apBefore, apAfter))
	}
	if len(res.AvailableSpecialEffects) > 0 {
		facts = append(facts, "special effects available: "+strings.Join(res.AvailableSpecialEffects, ", "))
	}
	a := Audit{}
	a.addRoll(res.AttackRoll)
	a.addRoll(res.ReactionRoll)
	a.addRoll(res.LocationRoll)
	a.addRoll(res.DamageRoll)
	a.addRoll(res.DamageModifierRoll)
	a.addRoll(res.NoAttacksRoll)
	for _, cr := range res.Consequences {
		a.addRoll(cr.EnduranceRoll)
	}
	a.addCitation("Mythras Imperative, Combat: How Combat Works")
	summary := attackSummary(res)
	return newResult(summary, facts, a), nil
}

// ApplySpecialEffect applies one Special Effects Table row to the exchange
// [ResolveAttack] recorded — impale, knockback, choose location and the
// rest (tool: apply_special_effect; combat.ApplySpecialEffect). Purely
// narrative rows come back with adjudicated effects and change no
// mechanics.
type ApplySpecialEffect struct {
	// Effect is the printed Special Effect name (required).
	Effect string `json:"effect"`
	// Attacker and Defender optionally restate the exchange's
	// combatants (character IDs); when given they must match the recorded
	// exchange.
	Attacker string `json:"attacker,omitempty"`
	Defender string `json:"defender,omitempty"`
	// Target optionally names the second victim (overpenetration,
	// select target) — an encounter participant other than the two
	// combatants.
	Target string `json:"target,omitempty"`
	// Location optionally names the hit location a location-picking effect
	// selects (choose location, entangle).
	Location string `json:"location,omitempty"`
}

// Kind implements [Move].
func (m ApplySpecialEffect) Kind() string { return "apply_special_effect" }

// Apply validates the running encounter, the recorded exchange, both
// combatants and the effect name before any mechanic runs, then applies
// the row through combat.ApplySpecialEffect against State.LastAttack.
func (m ApplySpecialEffect) Apply(s *State) (Result, error) {
	e, err := s.encounter()
	if err != nil {
		return Result{}, err
	}
	if !e.Started || e.Ended {
		return Result{}, fmt.Errorf("%w: effects need a running encounter", combat.ErrNotRunning)
	}
	res := s.LastAttack
	if res == nil {
		return Result{}, ErrNoAttack
	}
	if strings.TrimSpace(m.Effect) == "" {
		return Result{}, fmt.Errorf("%w: empty special effect name", ErrInvalidArgument)
	}
	row, ok := combat.EffectByName(m.Effect)
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", combat.ErrUnknownEffect, m.Effect)
	}
	if m.Attacker != "" && m.Attacker != res.Attacker {
		return Result{}, fmt.Errorf("%w: attacker %q does not match the recorded exchange (%q)", ErrInvalidArgument, m.Attacker, res.Attacker)
	}
	if m.Defender != "" && m.Defender != res.Defender {
		return Result{}, fmt.Errorf("%w: defender %q does not match the recorded exchange (%q)", ErrInvalidArgument, m.Defender, res.Defender)
	}
	att, err := s.character(res.Attacker)
	if err != nil {
		return Result{}, err
	}
	def, err := s.character(res.Defender)
	if err != nil {
		return Result{}, err
	}
	opts := combat.EffectOptions{Location: m.Location}
	if m.Target != "" {
		target, err := s.character(m.Target)
		if err != nil {
			return Result{}, err
		}
		opts.Target = target
	}
	er, err := combat.ApplySpecialEffect(s.Rand(), e, m.Effect, att, def, res, opts)
	if err != nil {
		return Result{}, err
	}

	facts := slices.Clone(er.Facts)
	if er.Adjudicated {
		facts = append(facts, "adjudicated: true — narration only, no mechanics changed")
	}
	if er.Location != "" {
		facts = append(facts, "hit location: "+er.Location)
	}
	if er.Delta != 0 {
		facts = append(facts, change("damage on "+er.Target, er.DamageApplied-er.Delta, er.DamageApplied))
	}
	if er.Severity != char.SeverityNone {
		facts = append(facts, "wound severity: "+er.Severity.String())
	}
	if len(er.Conditions) > 0 {
		facts = append(facts, "conditions: "+strings.Join(er.Conditions, ", "))
	}
	if er.Distance > 0 {
		facts = append(facts, fmt.Sprintf("distance moved: %d metres", er.Distance))
	}
	a := Audit{}
	for _, nr := range er.Rolls {
		a.addRoll(nr.Roll)
	}
	a.addCitation(row.Citation)
	summary := fmt.Sprintf("%s applied (%s side)", er.Effect, er.Side)
	if er.Adjudicated {
		summary += " — adjudicated"
	}
	return newResult(summary, facts, a), nil
}

// ApplyDamage applies raw damage outside the attack sequence — fire,
// falling, breath, collapsing masonry — through the damage application
// order (tool: apply_damage; combat.ApplyDamage). The engine rolls the hit
// location when one is not given.
type ApplyDamage struct {
	// Actor is the character taking the damage (ID).
	Actor string `json:"actor"`
	// Location is the hit location to damage; empty rolls one (consuming
	// session RNG).
	Location string `json:"location,omitempty"`
	// Amount is the damage after any Damage Modifier; must be positive.
	Amount int `json:"amount"`
}

// Kind implements [Move].
func (m ApplyDamage) Kind() string { return "apply_damage" }

// Apply validates actor and amount, snapshots total Hit Points, then runs
// combat.ApplyDamage. Errors wrap char.ErrInvalidAmount,
// combat.ErrInvalidChoice (unknown location) or the wound operations'.
func (m ApplyDamage) Apply(s *State) (Result, error) {
	c, err := s.character(m.Actor)
	if err != nil {
		return Result{}, err
	}
	if m.Amount <= 0 {
		return Result{}, fmt.Errorf("%w: damage %d", char.ErrInvalidAmount, m.Amount)
	}
	before := totalHP(c)
	res, err := combat.ApplyDamage(s.Rand(), c, m.Location, m.Amount)
	if err != nil {
		return Result{}, err
	}
	after := totalHP(c)
	facts := slices.Clone(res.Facts)
	facts = append(facts,
		change("total hit points", before, after),
		fmt.Sprintf("wound severity at %s: %s", res.Location, res.Severity),
	)
	a := Audit{Changes: []string{change("total hit points", before, after)}}
	a.addRoll(res.LocationRoll)
	a.addRoll(res.NoAttacksRoll)
	a.addCitation("Mythras Imperative, Game System: Damage and Wound Levels")
	summary := fmt.Sprintf("%s takes %d damage at the %s (%s)", c.ID, res.DamageApplied, res.Location, res.Severity)
	return newResult(summary, facts, a), nil
}

// Heal runs one healing action — first aid, treatment, or natural healing
// over a day, week or month (tool: heal; combat.Heal).
type Heal struct {
	// Actor is the character being healed (ID).
	Actor string `json:"actor"`
	// Method is one of "first aid", "treat", "day", "week", "month".
	Method string `json:"method"`
}

// Kind implements [Move].
func (m Heal) Kind() string { return "heal" }

// Apply validates the actor and method, snapshots total Hit Points, then
// runs combat.Heal. Errors wrap combat.ErrUnknownHealMethod or
// combat.ErrHealUnavailable.
func (m Heal) Apply(s *State) (Result, error) {
	c, err := s.character(m.Actor)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(m.Method) == "" {
		return Result{}, fmt.Errorf("%w: %q", combat.ErrUnknownHealMethod, m.Method)
	}
	before := totalHP(c)
	res, err := combat.Heal(s.Rand(), c, m.Method)
	if err != nil {
		return Result{}, err
	}
	after := totalHP(c)
	facts := slices.Clone(res.Facts)
	facts = append(facts, change("total hit points", before, after))
	if res.FatigueRecovered > 0 {
		facts = append(facts, fmt.Sprintf("fatigue recovered: %d", res.FatigueRecovered))
	}
	a := Audit{Changes: []string{change("total hit points", before, after)}}
	if res.Rolled {
		a.addRoll(d100Roll(res.SkillRoll))
	}
	a.addCitation("Mythras Imperative, Game System: Healing from Injury")
	summary := fmt.Sprintf("%s healed %d HP by %s", c.ID, res.Total, res.Method)
	if res.Rolled {
		summary += fmt.Sprintf(" (roll %d — %s)", res.SkillRoll, res.Grade)
	}
	return newResult(summary, facts, a), nil
}

// attackSummary renders the one-line result of an attack exchange.
func attackSummary(res combat.AttackResult) string {
	weapon := res.Weapon
	if weapon == "" {
		weapon = "unarmed"
	}
	switch {
	case res.Damaged:
		return fmt.Sprintf("%s hits %s with %s: %d to the %s (%s)",
			res.Attacker, res.Defender, weapon, res.DamageApplied, res.Location, res.Severity)
	case res.Hit:
		return fmt.Sprintf("%s hits %s with %s (%s) — no damage",
			res.Attacker, res.Defender, weapon, res.AttackOutcome.Grade)
	default:
		return fmt.Sprintf("%s attacks %s with %s — %s",
			res.Attacker, res.Defender, weapon, res.AttackOutcome.Grade)
	}
}

// idLabel prefers the engine ID for a character reference, falling back to
// the name (combat records identifiers the same way).
func idLabel(id, name string) string {
	if id != "" {
		return id
	}
	return name
}

// stateLabel renders the life flag for status facts.
func stateLabel(dead bool) string {
	if dead {
		return "dead"
	}
	return "alive"
}

// totalHP sums the character's current Hit Points across hit locations.
func totalHP(c *char.Character) int {
	total := 0
	for _, hp := range c.CurrentHP {
		total += hp
	}
	return total
}

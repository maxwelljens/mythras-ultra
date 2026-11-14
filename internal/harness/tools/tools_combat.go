package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mythras-ultra/myul/internal/engine/combat"
	"github.com/mythras-ultra/myul/internal/engine/resolve"
)

// combatTools returns the combat tools, in catalogue order.
func combatTools() []Tool {
	return []Tool{
		{
			Name: "combat_start",
			Description: "Open a combat encounter: register the participants, award " +
				"round 1 Action Points and roll initiative. Use the moment fighting " +
				"begins; every later combat tool needs this first." + factsNote,
			Schema: objSchema(map[string]any{
				"participants": strArrProp("Character IDs fighting (at least one, all distinct)."),
			}, "participants"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Participants []string `json:"participants"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireList("participants", len(a.Participants)); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.StartEncounter{Participants: a.Participants})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "combat_status",
			Description: "Report the running (or just-ended) encounter: round, strike " +
				"order, Action Points, conditions and hit locations per combatant. " +
				"Use before and during combat so narration stays accurate." + factsNote,
			Schema: objSchema(nil),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct{}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				report, err := resolve.CombatStatus(s)
				if err != nil {
					return Result{}, err
				}
				return Result{OK: true, Summary: statusSummary(report), Facts: statusFacts(report)}, nil
			},
		},
		{
			Name: "spend_ap",
			Description: "Pay Action Points for one Combat Action from the actor's " +
				"round allowance. Use to record an action's cost before describing it; " +
				"IMPOSSIBLE when the actor lacks the points or may not act." + factsNote,
			Schema: objSchema(map[string]any{
				"actor":  strProp("Character ID spending Action Points."),
				"action": strProp("Printed Combat Action name, e.g. \"Attack\", \"Move\", \"Parry\", \"Take Cover\"."),
			}, "actor", "action"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor  string `json:"actor"`
					Action string `json:"action"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				if err := requireField("action", a.Action); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.SpendAP{Actor: a.Actor, Action: a.Action})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "can_act",
			Description: "Check whether an actor may currently perform a Combat Action " +
				"— advisory, it spends nothing. Use before committing to an action with " +
				"preconditions (prone, pinned, no line of sight); the blockers come back " +
				"as facts." + factsNote,
			Schema: objSchema(map[string]any{
				"actor":  strProp("Character ID being checked."),
				"action": strProp("Printed Combat Action name, e.g. \"Attack\"."),
			}, "actor", "action"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor  string `json:"actor"`
					Action string `json:"action"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				if err := requireField("action", a.Action); err != nil {
					return Result{}, err
				}
				ok, blockers, err := resolve.CanAct(s, a.Actor, a.Action)
				if err != nil {
					return Result{}, err
				}
				facts := []string{fmt.Sprintf("allowed: %t", ok)}
				for _, b := range blockers {
					facts = append(facts, "blocker: "+b)
				}
				summary := fmt.Sprintf("%s may not %s", a.Actor, a.Action)
				if ok {
					summary = fmt.Sprintf("%s may %s", a.Actor, a.Action)
				}
				return Result{OK: true, Summary: summary, Facts: facts}, nil
			},
		},
		{
			Name: "next_round",
			Description: "Advance the encounter to the next Combat Round: Action Points " +
				"refresh for every combatant, round-limited conditions tick and wound " +
				"death timers run. Use when all combatants have acted." + factsNote,
			Schema: objSchema(nil),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct{}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.NextRound{})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "resolve_attack",
			Description: "Run one full attack exchange: attack roll, the defender's " +
				"reaction, Differential Roll, hit location, damage, armour and wound " +
				"severity. Use for each attack; the offered special effects come back as " +
				"facts to apply with apply_special_effect." + factsNote,
			Schema: objSchema(map[string]any{
				"attacker": strProp("Attacking character ID (an encounter participant)."),
				"weapon":   strProp("Weapon name (inventory item or sample table); omit for unarmed."),
				"defender": strProp("Defending character ID (an encounter participant)."),
				"reaction": enumProp("Defender's response; omit for none.", "parry", "dodge", "none"),
			}, "attacker", "defender"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Attacker string `json:"attacker"`
					Weapon   string `json:"weapon"`
					Defender string `json:"defender"`
					Reaction string `json:"reaction"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				for _, f := range []struct{ name, value string }{
					{"attacker", a.Attacker}, {"defender", a.Defender},
				} {
					if err := requireField(f.name, f.value); err != nil {
						return Result{}, err
					}
				}
				res, err := resolve.Apply(s, resolve.ResolveAttack{
					Attacker: a.Attacker, Weapon: a.Weapon, Defender: a.Defender, Reaction: a.Reaction,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "apply_special_effect",
			Description: "Apply one Special Effects Table row to the exchange " +
				"resolve_attack just recorded — impale, knockback, choose location and " +
				"the rest. Use only for an effect that exchange offered in its facts; " +
				"narrative rows come back with adjudicated: true." + factsNote,
			Schema: objSchema(map[string]any{
				"effect":   strProp("Printed Special Effect name, exactly as offered by resolve_attack."),
				"attacker": strProp("Restates the exchange's attacker ID when known (must match)."),
				"defender": strProp("Restates the exchange's defender ID when known (must match)."),
				"target":   strProp("Second victim for overpenetration / select target (an encounter participant)."),
				"location": strProp("Hit location a location-picking effect selects (choose location, entangle)."),
			}, "effect"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Effect   string `json:"effect"`
					Attacker string `json:"attacker"`
					Defender string `json:"defender"`
					Target   string `json:"target"`
					Location string `json:"location"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("effect", a.Effect); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.ApplySpecialEffect{
					Effect: a.Effect, Attacker: a.Attacker, Defender: a.Defender,
					Target: a.Target, Location: a.Location,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "apply_damage",
			Description: "Apply raw damage outside the attack sequence — fire, falling, " +
				"breath, collapsing masonry. Use for environmental and special damage; " +
				"the engine rolls the hit location when one is not given." + factsNote,
			Schema: objSchema(map[string]any{
				"actor":    strProp("Character taking the damage."),
				"location": strProp("Hit location to damage (e.g. \"Chest\"); omit to roll one on the session RNG."),
				"amount":   intProp("Damage after any Damage Modifier; must be positive.", 1),
			}, "actor", "amount"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor    string `json:"actor"`
					Location string `json:"location"`
					Amount   int    `json:"amount"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				if a.Amount < 1 {
					return Result{}, fmt.Errorf("%w: %q must be a positive integer", ErrInvalidArgs, "amount")
				}
				res, err := resolve.Apply(s, resolve.ApplyDamage{
					Actor: a.Actor, Location: a.Location, Amount: a.Amount,
				})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "heal",
			Description: "Heal the actor with first aid, treatment or natural healing " +
				"over a day, week or month. Use when healing is attempted; IMPOSSIBLE " +
				"when no untreated injury remains or the method is wrong." + factsNote,
			Schema: objSchema(map[string]any{
				"actor":  strProp("Character being healed."),
				"method": enumProp("Healing action.", "first aid", "treat", "day", "week", "month"),
			}, "actor", "method"),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct {
					Actor  string `json:"actor"`
					Method string `json:"method"`
				}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				if err := requireField("actor", a.Actor); err != nil {
					return Result{}, err
				}
				if err := requireField("method", a.Method); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.Heal{Actor: a.Actor, Method: a.Method})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
		{
			Name: "combat_end",
			Description: "Close the running encounter and summarise who survived. Use " +
				"the moment fighting stops; combat_status still reads the ended record." + factsNote,
			Schema: objSchema(nil),
			Handler: func(_ context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
				var a struct{}
				if err := decodeArgs(args, &a); err != nil {
					return Result{}, err
				}
				res, err := resolve.Apply(s, resolve.CombatEnd{})
				if err != nil {
					return Result{}, err
				}
				return fromMove(res), nil
			},
		},
	}
}

// statusSummary renders the one-line combat_status summary.
func statusSummary(rep combat.StatusReport) string {
	state := "running"
	switch {
	case rep.Ended:
		state = "ended"
	case !rep.Started:
		state = "not started"
	}
	return fmt.Sprintf("combat %s, round %d, %d combatant(s)", state, rep.Round, len(rep.Participants))
}

// statusFacts renders the full combat picture: round, strike order and one
// line per combatant with Action Points, life flag, conditions and hit
// locations.
func statusFacts(rep combat.StatusReport) []string {
	state := "running"
	if rep.Ended {
		state = "ended"
	}
	facts := []string{
		fmt.Sprintf("round: %d", rep.Round),
		"state: " + state,
		fmt.Sprintf("initiative rolled: %t", rep.InitiativeRolled),
	}
	for place, o := range rep.Order {
		if o.Index >= 0 && o.Index < len(rep.Participants) {
			facts = append(facts, fmt.Sprintf("%d. %s (initiative %d)", place+1, participantLabel(rep.Participants[o.Index]), o.Score))
		}
	}
	for _, p := range rep.Participants {
		line := fmt.Sprintf("%s: %d AP, %s", participantLabel(p), p.AP, lifeFlag(p.Dead))
		if len(p.Conditions) > 0 {
			line += ", conditions: " + strings.Join(p.Conditions, ", ")
		}
		facts = append(facts, line)
		if len(p.Locations) > 0 {
			parts := make([]string, 0, len(p.Locations))
			for _, loc := range p.Locations {
				parts = append(parts, fmt.Sprintf("%s %d/%d (%s, AP %d)",
					loc.Location, loc.Current, loc.Starting, loc.Severity, loc.ArmorPoints))
			}
			facts = append(facts, fmt.Sprintf("%s locations: %s", p.ID, strings.Join(parts, ", ")))
		}
	}
	for _, e := range rep.Errors {
		facts = append(facts, "error: "+e)
	}
	return facts
}

// participantLabel prefers the engine ID, falling back to the name.
func participantLabel(p combat.ParticipantStatus) string {
	if p.ID != "" {
		return p.ID
	}
	return p.Name
}

// lifeFlag renders the life flag for status facts.
func lifeFlag(dead bool) string {
	if dead {
		return "dead"
	}
	return "alive"
}

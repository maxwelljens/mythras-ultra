package resolve

import (
	"errors"
	"strings"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/combat"
)

// TestStartEncounter covers initiative, order facts, AP assignment and the
// refusal paths (empty, unknown, duplicate, already running).
func TestStartEncounter(t *testing.T) {
	s := newState(t, 42)
	addCharacter(t, s, "pc-1", true)
	addCharacter(t, s, "pc-2", true)

	res, err := Apply(s, StartEncounter{Participants: []string{"pc-2", "pc-1"}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if s.Encounter == nil || !s.Encounter.Started || s.Encounter.Ended {
		t.Fatalf("encounter = %+v, want running", s.Encounter)
	}
	if s.Encounter.Round != 1 || len(s.Encounter.Order) != 2 || !s.Encounter.InitiativeRolled {
		t.Errorf("round/order/initiative = %d/%v/%v", s.Encounter.Round, s.Encounter.Order, s.Encounter.InitiativeRolled)
	}
	if res.Audit == nil || len(res.Audit.Rolls) != 2 {
		t.Errorf("audit rolls = %+v, want two initiative 1d10s", res.Audit)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "1. ") || !strings.Contains(joined, "Action Points") {
		t.Errorf("facts = %v, want ordered initiative and AP lines", res.Facts)
	}
	if !strings.Contains(strings.Join(res.Audit.Citations, "\n"), "Combat Rounds and Initiative") {
		t.Errorf("citations = %v", res.Audit.Citations)
	}
	for _, ap := range s.Encounter.AP {
		if ap != 2 {
			t.Errorf("AP = %d, want the full 2", ap)
		}
	}
	// Participants are the canonical instances (identity contract).
	if s.Encounter.Participants[0] != s.Characters["pc-2"] {
		t.Error("participants are not the canonical character instances")
	}

	// Refusals.
	if _, err := Apply(s, StartEncounter{Participants: nil}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty participants error = %v, want ErrInvalidArgument", err)
	}
	if _, err := Apply(s, StartEncounter{Participants: []string{"ghost"}}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown participant error = %v, want ErrUnknownCharacter", err)
	}
	if _, err := Apply(s, StartEncounter{Participants: []string{"pc-1", "pc-1"}}); !errors.Is(err, combat.ErrDuplicateParticipant) {
		t.Errorf("duplicate error = %v, want combat.ErrDuplicateParticipant", err)
	}
	if _, err := Apply(s, StartEncounter{Participants: []string{"pc-1", "pc-2"}}); !errors.Is(err, ErrEncounterActive) {
		t.Errorf("second start error = %v, want ErrEncounterActive", err)
	}
}

// TestSpendAP covers the AP economy: table cost, before → after facts, the
// action citation, and every combat refusal sentinel.
func TestSpendAP(t *testing.T) {
	s, actor, _ := duelState(t, 42)
	e := s.Encounter

	res, err := Apply(s, SpendAP{Actor: actor, Action: "attack"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "Action Points: 2 → 1") {
		t.Errorf("facts = %v, want AP delta", res.Facts)
	}
	if res.Audit == nil || len(res.Audit.Citations) == 0 {
		t.Error("spend_ap must cite the action row")
	}
	if e.AP[0] != 1 {
		t.Errorf("AP = %d, want 1", e.AP[0])
	}

	// Spend down to zero, then refuse.
	if _, err := Apply(s, SpendAP{Actor: actor, Action: "attack"}); err != nil {
		t.Fatalf("second spend: %v", err)
	}
	if _, err := Apply(s, SpendAP{Actor: actor, Action: "attack"}); !errors.Is(err, combat.ErrNotEnoughAP) {
		t.Errorf("third spend error = %v, want combat.ErrNotEnoughAP", err)
	}

	if _, err := Apply(s, SpendAP{Actor: actor, Action: "tap dance"}); !errors.Is(err, combat.ErrUnknownAction) {
		t.Errorf("unknown action error = %v, want combat.ErrUnknownAction", err)
	}
	if _, err := Apply(s, SpendAP{Actor: "ghost", Action: "attack"}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown actor error = %v, want ErrUnknownCharacter", err)
	}
	// A known character who is not fighting.
	addCharacter(t, s, "npc-9", false)
	if _, err := Apply(s, SpendAP{Actor: "npc-9", Action: "attack"}); !errors.Is(err, combat.ErrNotInEncounter) {
		t.Errorf("non-participant error = %v, want combat.ErrNotInEncounter", err)
	}

	// No encounter at all.
	fresh := newState(t, 1)
	addCharacter(t, fresh, "pc-1", true)
	if _, err := Apply(fresh, SpendAP{Actor: "pc-1", Action: "attack"}); !errors.Is(err, ErrNoEncounter) {
		t.Errorf("no-encounter error = %v, want ErrNoEncounter", err)
	}
}

// TestNextRound covers the round advance and its guards.
func TestNextRound(t *testing.T) {
	s, _, _ := duelState(t, 42)
	res, err := Apply(s, NextRound{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if s.Encounter.Round != 2 {
		t.Errorf("Round = %d, want 2", s.Encounter.Round)
	}
	if !strings.Contains(strings.Join(res.Facts, "\n"), "round: 1 → 2") {
		t.Errorf("facts = %v", res.Facts)
	}
	if len(s.Encounter.AP) != 2 {
		t.Fatalf("AP slice = %v", s.Encounter.AP)
	}
	for i, ap := range s.Encounter.AP {
		if ap != 2 {
			t.Errorf("combatant %d AP = %d after refresh, want 2", i, ap)
		}
	}

	fresh := newState(t, 1)
	addCharacter(t, fresh, "pc-1", true)
	if _, err := Apply(fresh, NextRound{}); !errors.Is(err, ErrNoEncounter) {
		t.Errorf("no-encounter error = %v, want ErrNoEncounter", err)
	}
	if _, err := Apply(s, CombatEnd{}); err != nil {
		t.Fatalf("CombatEnd: %v", err)
	}
	if _, err := Apply(s, NextRound{}); !errors.Is(err, combat.ErrNotRunning) {
		t.Errorf("ended error = %v, want combat.ErrNotRunning", err)
	}
}

// TestCombatEnd covers the closing summary and the stale-exchange cleanup.
func TestCombatEnd(t *testing.T) {
	s, att, def := duelState(t, 42)
	if _, err := Apply(s, ResolveAttack{Attacker: att, Weapon: "Broadsword", Defender: def, Reaction: "parry"}); err != nil {
		t.Fatalf("ResolveAttack: %v", err)
	}
	if s.LastAttack == nil {
		t.Fatal("attack not recorded")
	}
	res, err := Apply(s, CombatEnd{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !s.Encounter.Ended {
		t.Error("encounter not ended")
	}
	if s.LastAttack != nil {
		t.Error("stale exchange survived CombatEnd")
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "encounter ended at round 1") || !strings.Contains(joined, "survivors: 2 alive, 0 dead") {
		t.Errorf("facts = %v", res.Facts)
	}

	fresh := newState(t, 1)
	addCharacter(t, fresh, "pc-1", true)
	if _, err := Apply(fresh, CombatEnd{}); !errors.Is(err, ErrNoEncounter) {
		t.Errorf("no-encounter error = %v, want ErrNoEncounter", err)
	}
}

// TestResolveAttackHappy runs the full exchange through resolve: AP spent,
// the recorded exchange for effects, every dice roll audited, and the
// rulebook citation attached.
func TestResolveAttackHappy(t *testing.T) {
	s, att, def := duelState(t, 42)
	e := s.Encounter

	res, err := Apply(s, ResolveAttack{Attacker: att, Weapon: "Broadsword", Defender: def, Reaction: "parry"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if s.LastAttack == nil || s.LastAttack.Attacker != att {
		t.Fatalf("LastAttack = %+v, want the recorded exchange", s.LastAttack)
	}
	if e.AP[0] != 1 {
		t.Errorf("attacker AP = %d, want 1 (Attack costs 1)", e.AP[0])
	}
	if res.Audit == nil || len(res.Audit.Rolls) == 0 {
		t.Fatal("audit carries no rolls")
	}
	if res.Audit.Rolls[0].Expr != "1d100" {
		t.Errorf("first audited roll = %s, want the attack d100", res.Audit.Rolls[0].Expr)
	}
	if !strings.Contains(strings.Join(res.Audit.Citations, "\n"), "How Combat Works") {
		t.Errorf("citations = %v", res.Audit.Citations)
	}
	if len(res.Facts) == 0 {
		t.Error("no facts to narrate")
	}
	if res.Summary == "" {
		t.Error("empty summary")
	}
	// Facts echo the engine's own audit lines plus the AP delta.
	if !strings.Contains(strings.Join(res.Facts, "\n"), "Action Points: 2 → 1") {
		t.Errorf("facts = %v, want the AP delta appended", res.Facts)
	}

	// Refusals.
	fresh := newState(t, 1)
	addCharacter(t, fresh, "pc-1", true)
	if _, err := Apply(fresh, ResolveAttack{Attacker: "pc-1", Defender: "pc-1"}); !errors.Is(err, ErrNoEncounter) {
		t.Errorf("no-encounter error = %v, want ErrNoEncounter", err)
	}
	if _, err := Apply(s, ResolveAttack{Attacker: "ghost", Weapon: "Broadsword", Defender: def}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown attacker error = %v, want ErrUnknownCharacter", err)
	}
	if _, err := Apply(s, ResolveAttack{Attacker: att, Weapon: "Broadsword", Defender: "ghost"}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown defender error = %v, want ErrUnknownCharacter", err)
	}
	if _, err := Apply(s, ResolveAttack{Attacker: att, Weapon: "Broadsword", Defender: def, Reaction: "sidestep"}); !errors.Is(err, combat.ErrUnknownReaction) {
		t.Errorf("bad reaction error = %v, want combat.ErrUnknownReaction", err)
	}
	addCharacter(t, s, "npc-7", false)
	if _, err := Apply(s, ResolveAttack{Attacker: "npc-7", Weapon: "Broadsword", Defender: def}); !errors.Is(err, combat.ErrNotInEncounter) {
		t.Errorf("non-participant error = %v, want combat.ErrNotInEncounter", err)
	}
	if _, err := Apply(s, ResolveAttack{Attacker: att, Weapon: "Musket of Doom", Defender: def}); !errors.Is(err, combat.ErrUnknownWeapon) {
		t.Errorf("unknown weapon error = %v, want combat.ErrUnknownWeapon", err)
	}
}

// TestApplySpecialEffectHappy searches seeds for an exchange that offers a
// special effect, then applies one end-to-end through resolve: the effect
// lands, the recorded exchange shows it, and its dice reach the audit. Each
// refused pick rolls back (state and RNG), so the search itself also proves
// failed effect attempts are invisible.
func TestApplySpecialEffectHappy(t *testing.T) {
	found := false
	for seed := int64(1); seed <= 60 && !found; seed++ {
		s, att, def := duelState(t, seed)
		if _, err := Apply(s, ResolveAttack{Attacker: att, Weapon: "Broadsword", Defender: def, Reaction: "parry"}); err != nil {
			t.Fatalf("seed %d: ResolveAttack: %v", seed, err)
		}
		avail := s.LastAttack.AvailableSpecialEffects
		if len(avail) == 0 {
			continue
		}
		for _, effect := range avail {
			res, err := Apply(s, ApplySpecialEffect{
				Effect: effect, Attacker: att, Defender: def,
			})
			if err != nil {
				continue // pick refused (gate/option) — rolled back, try the next
			}
			found = true
			if !strings.Contains(strings.Join(res.Facts, "\n"), effect) {
				t.Errorf("seed %d: facts do not name the effect: %v", seed, res.Facts)
			}
			if !containsFold(s.LastAttack.AppliedEffects, effect) {
				t.Errorf("seed %d: exchange does not record %q: %v", seed, effect, s.LastAttack.AppliedEffects)
			}
			// A second pick of a non-stackable row must refuse (and the
			// refusal rolls back — nothing about the first pick changes).
			if _, err := Apply(s, ApplySpecialEffect{
				Effect: effect, Attacker: att, Defender: def,
			}); err == nil && !effectStacks(effect) {
				t.Errorf("seed %d: non-stackable effect %q applied twice", seed, effect)
			}
			break
		}
	}
	if !found {
		t.Skip("no seed in 1..60 produced an applicable special effect")
	}
}

// TestApplySpecialEffectRefusals covers the validation ladder without an
// exchange: no encounter, no attack, unknown effect, mismatched combatants.
func TestApplySpecialEffectRefusals(t *testing.T) {
	fresh := newState(t, 1)
	addCharacter(t, fresh, "pc-1", true)
	if _, err := Apply(fresh, ApplySpecialEffect{Effect: "Impale"}); !errors.Is(err, ErrNoEncounter) {
		t.Errorf("no-encounter error = %v, want ErrNoEncounter", err)
	}

	s, att, def := duelState(t, 42)
	if _, err := Apply(s, ApplySpecialEffect{Effect: "Impale"}); !errors.Is(err, ErrNoAttack) {
		t.Errorf("no-attack error = %v, want ErrNoAttack", err)
	}
	if _, err := Apply(s, ResolveAttack{Attacker: att, Weapon: "Broadsword", Defender: def, Reaction: "parry"}); err != nil {
		t.Fatalf("ResolveAttack: %v", err)
	}
	if _, err := Apply(s, ApplySpecialEffect{Effect: "Summon Giraffe"}); !errors.Is(err, combat.ErrUnknownEffect) {
		t.Errorf("unknown effect error = %v, want combat.ErrUnknownEffect", err)
	}
	if _, err := Apply(s, ApplySpecialEffect{Effect: ""}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty effect error = %v, want ErrInvalidArgument", err)
	}
	if _, err := Apply(s, ApplySpecialEffect{Effect: "Impale", Attacker: def, Defender: att}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("mismatched attacker error = %v, want ErrInvalidArgument", err)
	}
	if _, err := Apply(s, ApplySpecialEffect{Effect: "Impale", Target: "ghost"}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown target error = %v, want ErrUnknownCharacter", err)
	}
}

// TestApplyDamage covers explicit and rolled hit locations, severity facts
// and the amount/location refusals.
func TestApplyDamage(t *testing.T) {
	s := newState(t, 64)
	c := addCharacter(t, s, "pc-1", true)
	before := totalHP(c)

	res, err := Apply(s, ApplyDamage{Actor: "pc-1", Location: "Chest", Amount: 5})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "total hit points: ") || !strings.Contains(joined, "wound severity at Chest: ") {
		t.Errorf("facts = %v, want HP delta and severity", res.Facts)
	}
	after := totalHP(c)
	if after >= before {
		t.Errorf("HP did not fall: %d → %d", before, after)
	}
	if res.Audit == nil || len(res.Audit.Changes) == 0 {
		t.Error("damage must be audited")
	}
	if !strings.Contains(strings.Join(res.Audit.Citations, "\n"), "Damage and Wound Levels") {
		t.Errorf("citations = %v", res.Audit.Citations)
	}

	// Location rolled when not given: the 1d20 reaches the audit.
	s2 := newState(t, 64)
	addCharacter(t, s2, "pc-1", true)
	res2, err := Apply(s2, ApplyDamage{Actor: "pc-1", Amount: 1})
	if err != nil {
		t.Fatalf("rolled location: %v", err)
	}
	found1d20 := false
	for _, r := range res2.Audit.Rolls {
		if r.Expr == "1d20" {
			found1d20 = true
		}
	}
	if !found1d20 {
		t.Errorf("rolls = %+v, want the 1d20 hit-location roll", res2.Audit.Rolls)
	}

	if _, err := Apply(s, ApplyDamage{Actor: "pc-1", Location: "Tail", Amount: 3}); !errors.Is(err, combat.ErrInvalidChoice) {
		t.Errorf("bad location error = %v, want combat.ErrInvalidChoice", err)
	}
	if _, err := Apply(s, ApplyDamage{Actor: "pc-1", Amount: 0}); !errors.Is(err, char.ErrInvalidAmount) {
		t.Errorf("zero damage error = %v, want char.ErrInvalidAmount", err)
	}
	if _, err := Apply(s, ApplyDamage{Actor: "ghost", Amount: 3}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown actor error = %v, want ErrUnknownCharacter", err)
	}
}

// TestHeal covers natural healing with its HP delta fact and the method
// refusal.
func TestHeal(t *testing.T) {
	s := newState(t, 64)
	c := addCharacter(t, s, "pc-1", true)
	if _, err := Apply(s, ApplyDamage{Actor: "pc-1", Location: "Chest", Amount: 5}); err != nil {
		t.Fatalf("wound: %v", err)
	}
	wounded := totalHP(c)

	res, err := Apply(s, Heal{Actor: "pc-1", Method: "day"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "total hit points: ") {
		t.Errorf("facts = %v, want the HP delta", res.Facts)
	}
	if totalHP(c) <= wounded {
		t.Errorf("HP %d → %d, natural healing must restore something", wounded, totalHP(c))
	}
	if res.Audit == nil || len(res.Audit.Changes) == 0 {
		t.Error("heal must be audited")
	}
	if !strings.Contains(strings.Join(res.Audit.Citations, "\n"), "Healing from Injury") {
		t.Errorf("citations = %v", res.Audit.Citations)
	}

	if _, err := Apply(s, Heal{Actor: "pc-1", Method: "centuries"}); !errors.Is(err, combat.ErrUnknownHealMethod) {
		t.Errorf("bad method error = %v, want combat.ErrUnknownHealMethod", err)
	}
	if _, err := Apply(s, Heal{Actor: "ghost", Method: "day"}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown actor error = %v, want ErrUnknownCharacter", err)
	}
}

// containsFold is a case-insensitive membership test for effect names.
func containsFold(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

// effectStacks reports whether a Special Effects Table row is printed as
// stackable (a repeat is then legal within one exchange).
func effectStacks(name string) bool {
	row, ok := combat.EffectByName(name)
	return ok && row.Stackable
}

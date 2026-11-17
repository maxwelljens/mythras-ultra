package combat

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
	"github.com/maxwelljens/mythras-ultra/internal/engine/rules"
)

// TestStartValidation checks the input contract of Start: nil or empty
// participant lists, nil participants and duplicates (pointer or shared
// ID) refuse the encounter, wrapping the documented sentinels.
func TestStartValidation(t *testing.T) {
	a := fighter(t, "dup-1", "A")
	b := fighter(t, "dup-2", "B")
	clone := *b // same ID, different pointer

	cases := []struct {
		name    string
		in      []*char.Character
		wantErr error
	}{
		{"nil list", nil, ErrInvalidEncounter},
		{"empty list", []*char.Character{}, ErrInvalidEncounter},
		{"nil participant", []*char.Character{a, nil}, ErrInvalidEncounter},
		{"same pointer twice", []*char.Character{a, a}, ErrDuplicateParticipant},
		{"same ID twice", []*char.Character{b, &clone}, ErrDuplicateParticipant},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Start(tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Start() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// TestStartRoundAndOrder verifies round 1, the full Action Point
// complement and the preliminary DEX/INT strike order (Combat ›
// Initiative), highest first.
func TestStartRoundAndOrder(t *testing.T) {
	slow := fighter(t, "slow-1", "Slow")
	slow.Chars.DEX, slow.Chars.INT = 8, 8
	if err := slow.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	fast := fighter(t, "fast-1", "Fast")
	e, err := Start([]*char.Character{slow, fast})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if e.Round != 1 || !e.Started || e.Ended {
		t.Fatalf("Round=%d Started=%v Ended=%v, want round 1 running", e.Round, e.Started, e.Ended)
	}
	for i, ap := range e.AP {
		if ap != 2 {
			t.Errorf("participant %d AP = %d, want 2 (tables.md §4.1)", i, ap)
		}
	}
	if len(e.Order) != 2 || e.Order[0].Index != 1 {
		t.Fatalf("order = %+v, want the faster character first", e.Order)
	}
	// The preliminary score is the Initiative modifier (average of DEX
	// and INT), before any 1d10.
	init, err := char.Derive(fast)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if e.Order[0].Score != init.Initiative {
		t.Errorf("order score = %d, want Initiative %d", e.Order[0].Score, init.Initiative)
	}
}

// TestRollInitiativeAudit checks the 1d10 initiative roll: every entry
// carries a recorded die, the armour penalty (highest worn AP) is
// subtracted, and the order is sorted by score, then DEX, then the
// tie-break die.
func TestRollInitiativeAudit(t *testing.T) {
	att := fighter(t, "init-a", "Armoured")
	wearMail(t, att) // AP 6
	def := fighter(t, "init-b", "Unarmoured")
	run := func(seed int64) []OrderEntry {
		e, err := Start([]*char.Character{att, def})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if err := RollInitiative(dice.NewRand(seed), e); err != nil {
			t.Fatalf("RollInitiative: %v", err)
		}
		return e.Order
	}
	first := run(42)
	if len(first) != 2 {
		t.Fatalf("order length %d", len(first))
	}
	for _, o := range first {
		if o.Roll.Expr != "1d10" || len(o.Roll.Dice) != 1 {
			t.Errorf("initiative roll = %+v, want one 1d10", o.Roll)
		}
		switch o.Index {
		case 0: // the armoured fighter pays the highest worn AP
			if o.Penalty != 6 {
				t.Errorf("armour penalty = %d, want the highest worn AP (6)", o.Penalty)
			}
		case 1:
			if o.Penalty != 0 {
				t.Errorf("unarmoured penalty = %d, want 0", o.Penalty)
			}
		}
	}
	// deterministic for a fixed seed
	second := run(42)
	for i := range first {
		if first[i].Score != second[i].Score || first[i].Roll.Total != second[i].Roll.Total {
			t.Fatalf("initiative not deterministic at %d: %+v vs %+v", i, first[i], second[i])
		}
	}
	for i := 1; i < len(first); i++ {
		if first[i-1].Score < first[i].Score {
			t.Errorf("order not sorted by score: %+v", first)
		}
	}
}

// TestEncounterJSONRoundTrip proves the encounter survives persistence:
// marshal, unmarshal, marshal again byte-for-byte, and the loaded copy
// still resolves operations (identity falls back to the character ID).
func TestEncounterJSONRoundTrip(t *testing.T) {
	_, e, att, _ := duel(t, 7)
	if err := SpendAP(e, att, "Attack"); err != nil {
		t.Fatalf("SpendAP: %v", err)
	}
	if err := e.NextRound(); err != nil {
		t.Fatalf("NextRound: %v", err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var loaded Encounter
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	raw2, err := json.Marshal(&loaded)
	if err != nil {
		t.Fatalf("marshal loaded: %v", err)
	}
	if string(raw) != string(raw2) {
		t.Fatalf("round trip changed bytes:\n%s\n%s", raw, raw2)
	}
	if loaded.Round != 2 {
		t.Errorf("loaded round = %d, want 2", loaded.Round)
	}
	// The loaded participants are fresh pointers: identity must resolve
	// through the character ID.
	if err := SpendAP(&loaded, att, "Attack"); err != nil {
		t.Fatalf("SpendAP on loaded encounter (ID identity): %v", err)
	}
	rep := loaded.Status()
	if len(rep.Participants) != 2 || rep.Errors != nil {
		t.Errorf("Status after load = %+v, want 2 participants and no errors", rep)
	}
}

// TestNextRoundRefreshTicksAndBleeds covers AP refresh (unused AP do not
// carry), round-limited condition ticking, the untreated-Major-Wound
// death timer, and Blood Loss charging one Fatigue level per round.
func TestNextRoundRefreshTicksAndBleeds(t *testing.T) {
	_, e, att, def := duel(t, 11)
	if err := SpendAP(e, att, "Attack"); err != nil {
		t.Fatalf("SpendAP: %v", err)
	}
	att.AddCondition(char.Condition{Name: CondNoAttacks, Rounds: 2})
	def.AddCondition(char.Condition{Name: char.CondBleeding})
	if err := e.NextRound(); err != nil {
		t.Fatalf("NextRound: %v", err)
	}
	if e.Round != 2 {
		t.Errorf("round = %d, want 2", e.Round)
	}
	if e.AP[0] != 2 {
		t.Errorf("AP after refresh = %d, want 2 (unused AP do not carry over)", e.AP[0])
	}
	ban := findCondition(att, CondNoAttacks)
	if ban == nil || ban.Rounds != 1 {
		t.Fatalf("No Attacks condition = %+v, want 1 round left", ban)
	}
	if !def.HasCondition(rules.FatigueCondition) {
		t.Fatalf("bleeding should have charged a Fatigue level")
	}
	// Second tick expires the ban.
	if err := e.NextRound(); err != nil {
		t.Fatalf("NextRound: %v", err)
	}
	if findCondition(att, CondNoAttacks) != nil {
		t.Errorf("No Attacks should have expired")
	}
}

// TestNextRoundUntreatedMajorDeath: the 2 × Healing Rounds treatment
// timer kills when it runs out (tables.md §3.9).
func TestNextRoundUntreatedMajorDeath(t *testing.T) {
	_, e, att, _ := duel(t, 13)
	d, err := char.Derive(att)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	att.AddCondition(char.Condition{Name: CondUntreatedMajorWound, Rounds: 1})
	if att.Dead {
		t.Fatal("precondition: alive")
	}
	if err := e.NextRound(); err != nil {
		t.Fatalf("NextRound: %v", err)
	}
	if !att.Dead {
		t.Fatalf("expired Untreated Major Wound should kill (Healing Rate %d)", d.HealingRate)
	}
}

// TestEndRefusesOperations: after End every mechanical operation refuses
// with ErrNotRunning, and ending twice is harmless.
func TestEndRefusesOperations(t *testing.T) {
	_, e, att, _ := duel(t, 17)
	e.End()
	e.End() // idempotent
	if err := SpendAP(e, att, "Attack"); !errors.Is(err, ErrNotRunning) {
		t.Errorf("SpendAP after End = %v, want ErrNotRunning", err)
	}
	if err := e.NextRound(); !errors.Is(err, ErrNotRunning) {
		t.Errorf("NextRound after End = %v, want ErrNotRunning", err)
	}
	_, err := ResolveAttack(dice.NewRand(1), e, att, "Broadsword", att, ReactionNone)
	if !errors.Is(err, ErrNotRunning) {
		t.Errorf("ResolveAttack after End = %v, want ErrNotRunning", err)
	}
	rep := e.Status()
	if !rep.Ended {
		t.Errorf("Status.Ended = false after End")
	}
}

// TestStatusReportsState: per-location Hit Points, armour values and
// conditions appear in the status report.
func TestStatusReportsState(t *testing.T) {
	_, _, att, def := duel(t, 19)
	wearMail(t, def)
	if _, _, err := def.ApplyWound("Chest", 3); err != nil {
		t.Fatalf("ApplyWound: %v", err)
	}
	e, err := Start([]*char.Character{att, def})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	rep := e.Status()
	if len(rep.Participants) != 2 {
		t.Fatalf("participants = %d, want 2", len(rep.Participants))
	}
	var chest *LocationStatus
	for i := range rep.Participants[1].Locations {
		if rep.Participants[1].Locations[i].Location == "Chest" {
			chest = &rep.Participants[1].Locations[i]
		}
	}
	if chest == nil {
		t.Fatal("no Chest entry in status")
	}
	if chest.Current != chest.Starting-3 {
		t.Errorf("Chest HP = %d/%d, want 3 down", chest.Current, chest.Starting)
	}
	if chest.ArmorPoints != 6 {
		t.Errorf("Chest armour = %d, want 6 (Mail)", chest.ArmorPoints)
	}
	if chest.Severity != "Minor Wound" {
		t.Errorf("Chest severity = %q, want Minor Wound", chest.Severity)
	}
}

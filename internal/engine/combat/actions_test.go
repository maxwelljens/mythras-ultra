package combat

import (
	"errors"
	"strings"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
)

// TestActionTable covers the Combat Actions and Costs table: every row
// resolves by name (case- and space-insensitively), unknown names do not,
// and Actions() hands out a copy.
func TestActionTable(t *testing.T) {
	if got := len(Actions()); got != 29 {
		t.Fatalf("Actions() = %d rows, want 29 (19 Proactive + 4 Reactive + 6 Free)", got)
	}
	cases := []struct {
		in      string
		wantAP  int
		wantTyp ActionType
		wantOK  bool
	}{
		{"attack", 1, Proactive, true},
		{"  Parry  ", 1, Reactive, true},
		{"SPEAK", 0, Free, true},
		{"Ward Location", 0, Free, true},
		{"Reload", 1, Proactive, true},
		{"Dance", 0, "", false},
		{"", 0, "", false},
	}
	for _, tc := range cases {
		got, ok := ActionByName(tc.in)
		if ok != tc.wantOK {
			t.Errorf("ActionByName(%q) ok = %v, want %v", tc.in, ok, tc.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if got.AP != tc.wantAP || got.Type != tc.wantTyp {
			t.Errorf("ActionByName(%q) = %+v, want AP %d type %s", tc.in, got, tc.wantAP, tc.wantTyp)
		}
		if got.Citation == "" || got.Mechanics == "" {
			t.Errorf("Action %q missing mechanics or citation", got.Name)
		}
	}
	// The copy must not alias the table.
	all := Actions()
	all[0].Name = "mutated"
	if again, _ := ActionByName("Attack"); again.Name != "Attack" {
		t.Fatal("Actions() leaked the internal table")
	}
}

// TestSpendAPEconomy walks the AP boundaries: 2 per Round, Free actions
// free, insufficient points refused with ErrNotEnoughAP, and every
// actor-state blocker refused with ErrCannotAct or ErrNotInEncounter.
func TestSpendAPEconomy(t *testing.T) {
	t.Run("2 AP per round", func(t *testing.T) {
		_, e, att, _ := duel(t, 1)
		if err := SpendAP(e, att, "Attack"); err != nil {
			t.Fatalf("first attack: %v", err)
		}
		if e.AP[0] != 1 {
			t.Fatalf("AP = %d, want 1", e.AP[0])
		}
		if err := SpendAP(e, att, "Attack"); err != nil {
			t.Fatalf("second attack: %v", err)
		}
		err := SpendAP(e, att, "Attack")
		if !errors.Is(err, ErrNotEnoughAP) {
			t.Fatalf("third attack = %v, want ErrNotEnoughAP", err)
		}
		if !strings.Contains(err.Error(), "costs 1") {
			t.Errorf("message %q should name the cost", err)
		}
	})
	t.Run("free actions cost nothing", func(t *testing.T) {
		_, e, att, _ := duel(t, 2)
		for i := 0; i < 3; i++ {
			if err := SpendAP(e, att, "Speak"); err != nil {
				t.Fatalf("Speak %d: %v", i, err)
			}
		}
		if e.AP[0] != 2 {
			t.Errorf("AP = %d, want 2 unchanged", e.AP[0])
		}
	})
	t.Run("unknown action", func(t *testing.T) {
		_, e, att, _ := duel(t, 3)
		if err := SpendAP(e, att, "Juggle"); !errors.Is(err, ErrUnknownAction) {
			t.Errorf("err = %v, want ErrUnknownAction", err)
		}
	})
	t.Run("dead cannot act", func(t *testing.T) {
		_, e, att, _ := duel(t, 4)
		att.Dead = true
		err := SpendAP(e, att, "Attack")
		if !errors.Is(err, ErrCannotAct) || !strings.Contains(err.Error(), "dead") {
			t.Errorf("err = %v, want ErrCannotAct naming death", err)
		}
	})
	t.Run("unconscious cannot act", func(t *testing.T) {
		_, e, att, _ := duel(t, 5)
		att.AddCondition(char.Condition{Name: char.CondUnconscious})
		err := SpendAP(e, att, "Parry")
		if !errors.Is(err, ErrCannotAct) || !strings.Contains(err.Error(), "unconscious") {
			t.Errorf("err = %v, want ErrCannotAct naming unconscious", err)
		}
	})
	t.Run("incapacitated cannot act", func(t *testing.T) {
		_, e, att, _ := duel(t, 6)
		att.AddCondition(char.Condition{Name: char.CondIncapacitated})
		err := SpendAP(e, att, "Attack")
		if !errors.Is(err, ErrCannotAct) || !strings.Contains(err.Error(), "incapacitated") {
			t.Errorf("err = %v, want ErrCannotAct naming incapacitated", err)
		}
	})
	t.Run("prone cannot move or charge", func(t *testing.T) {
		_, e, att, _ := duel(t, 7)
		att.AddCondition(char.Condition{Name: char.CondProne})
		err := SpendAP(e, att, "Move")
		if !errors.Is(err, ErrCannotAct) || !strings.Contains(err.Error(), "Regain Footing") {
			t.Errorf("err = %v, want the crawl/Regain Footing guidance", err)
		}
		if err := SpendAP(e, att, "Attack"); err != nil {
			t.Errorf("fighting from prone is penalised, not blocked: %v", err)
		}
	})
	t.Run("surprised cannot act offensively", func(t *testing.T) {
		_, e, att, _ := duel(t, 8)
		att.AddCondition(char.Condition{Name: CondSurprised, Rounds: 1})
		if err := SpendAP(e, att, "Move"); err != nil {
			t.Errorf("movement while surprised: %v", err)
		}
		err := SpendAP(e, att, "Attack")
		if !errors.Is(err, ErrCannotAct) || !strings.Contains(err.Error(), "surprised") {
			t.Errorf("err = %v, want ErrCannotAct naming surprise", err)
		}
	})
	t.Run("Serious Wound attack ban", func(t *testing.T) {
		_, e, att, _ := duel(t, 9)
		att.AddCondition(char.Condition{Name: CondNoAttacks, Rounds: 2})
		err := SpendAP(e, att, "Attack")
		if !errors.Is(err, ErrCannotAct) || !strings.Contains(err.Error(), "Serious Wound") {
			t.Errorf("err = %v, want the Serious Wound ban", err)
		}
		if err := SpendAP(e, att, "Parry"); err != nil {
			t.Errorf("Serious Wound still allows Parry: %v", err)
		}
	})
	t.Run("stunned head blocks everything", func(t *testing.T) {
		_, e, att, _ := duel(t, 10)
		att.AddCondition(char.Condition{Name: char.CondStunned, Location: "Head", Rounds: 3})
		err := SpendAP(e, att, "Attack")
		if !errors.Is(err, ErrCannotAct) || !strings.Contains(err.Error(), "insensible") {
			t.Errorf("err = %v, want insensible-stunned-head", err)
		}
	})
	t.Run("stunned torso may only defend", func(t *testing.T) {
		_, e, att, _ := duel(t, 11)
		att.AddCondition(char.Condition{Name: char.CondStunned, Location: "Chest", Rounds: 3})
		if err := SpendAP(e, att, "Parry"); err != nil {
			t.Errorf("defending with a stunned torso: %v", err)
		}
		err := SpendAP(e, att, "Move")
		if !errors.Is(err, ErrCannotAct) || !strings.Contains(err.Error(), "staggered") {
			t.Errorf("err = %v, want staggered-only-defend", err)
		}
	})
	t.Run("stunned leg cannot move", func(t *testing.T) {
		_, e, att, _ := duel(t, 12)
		att.AddCondition(char.Condition{Name: char.CondStunned, Location: "Right Leg", Rounds: 3})
		err := SpendAP(e, att, "Move")
		if !errors.Is(err, ErrCannotAct) || !strings.Contains(err.Error(), "cannot move") {
			t.Errorf("err = %v, want the leg-cannot-move blocker", err)
		}
	})
	t.Run("non-participant", func(t *testing.T) {
		_, e, _, _ := duel(t, 13)
		outsider := fighter(t, "out-1", "Outsider")
		err := SpendAP(e, outsider, "Attack")
		if !errors.Is(err, ErrNotInEncounter) {
			t.Errorf("err = %v, want ErrNotInEncounter", err)
		}
	})
	t.Run("stopped encounter", func(t *testing.T) {
		_, e, att, _ := duel(t, 14)
		e.End()
		if err := SpendAP(e, att, "Attack"); !errors.Is(err, ErrNotRunning) {
			t.Errorf("err = %v, want ErrNotRunning", err)
		}
	})
}

// TestCanActMessages: blockers come back as human-readable reasons an
// LLM GM can quote directly.
func TestCanActMessages(t *testing.T) {
	_, e, att, _ := duel(t, 21)
	if ok, why := CanAct(e, att, "Attack"); !ok || why != nil {
		t.Fatalf("CanAct(Attack) = %v, %v, want legal", ok, why)
	}
	att.AddCondition(char.Condition{Name: char.CondProne})
	ok, why := CanAct(e, att, "Move")
	if ok || len(why) == 0 {
		t.Fatalf("CanAct(prone Move) = %v, %v, want blocked", ok, why)
	}
	if !strings.Contains(why[0], "prone") {
		t.Errorf("blocker %q should name the state", why[0])
	}
	att.RemoveCondition(char.CondProne)
	if err := SpendAP(e, att, "Attack"); err != nil {
		t.Fatalf("SpendAP: %v", err)
	}
	if err := SpendAP(e, att, "Attack"); err != nil {
		t.Fatalf("SpendAP: %v", err)
	}
	ok, why = CanAct(e, att, "Attack")
	if ok || len(why) == 0 || !strings.Contains(why[0], "Action Points") {
		t.Fatalf("exhausted AP blocker = %v, %v, want an Action Points message", ok, why)
	}
	ok, why = CanAct(e, att, "Juggle")
	if ok || len(why) == 0 || !strings.Contains(why[0], "unknown action") {
		t.Fatalf("unknown action blocker = %v, %v", ok, why)
	}
}

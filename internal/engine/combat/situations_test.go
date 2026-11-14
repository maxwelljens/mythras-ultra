package combat

import (
	"errors"
	"strings"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/dice"
	"github.com/mythras-ultra/myul/internal/engine/rules"
)

// TestKnockbackTriggerDistanceAndFalling covers the printed knockback
// mechanic: the SIZ trigger, Brace's raised SIZ, 1 metre per 5 points of
// damage over SIZ (fractions up), and the fall avoided by Easy
// Acrobatics or Standard Athletics (tables.md §4.10).
func TestKnockbackTriggerDistanceAndFalling(t *testing.T) {
	t.Run("no trigger at or under SIZ", func(t *testing.T) {
		_, _, _, def := duel(t, 1)
		res, err := Knockback(dice.NewRand(1), def, def.Chars.SIZ)
		if err != nil {
			t.Fatalf("Knockback: %v", err)
		}
		if res.Triggered {
			t.Errorf("damage == SIZ triggered: %+v", res)
		}
		if len(res.Rolls) != 0 {
			t.Errorf("no fall roll should be made without a trigger")
		}
	})
	t.Run("distance over SIZ", func(t *testing.T) {
		_, _, _, def := duel(t, 2)
		siz := def.Chars.SIZ // 12
		res, err := Knockback(dice.NewRand(2), def, siz+18)
		if err != nil {
			t.Fatalf("Knockback: %v", err)
		}
		if !res.Triggered || res.Distance != 4 { // 18 over → ceil(18/5) = 4
			t.Errorf("triggered=%v distance=%d, want true/4", res.Triggered, res.Distance)
		}
		joined := strings.Join(res.Facts, " ")
		if !strings.Contains(joined, "obstacle") {
			t.Errorf("facts = %v, want the GM obstacle clause", res.Facts)
		}
	})
	t.Run("Brace raises the effective SIZ", func(t *testing.T) {
		_, _, _, def := duel(t, 3)
		def.AddCondition(char.Condition{Name: CondBraced})
		siz := def.Chars.SIZ
		eff := (siz*3 + 1) / 2                           // +50%
		res, err := Knockback(dice.NewRand(3), def, eff) // exactly the braced SIZ
		if err != nil {
			t.Fatalf("Knockback: %v", err)
		}
		if res.Triggered || !res.Braced {
			t.Errorf("bracing did not raise SIZ: %+v", res)
		}
	})
	t.Run("fall outcome tracks the rolls", func(t *testing.T) {
		for seed := int64(4); seed <= 60; seed++ {
			_, _, _, def := duel(t, seed)
			res, err := Knockback(dice.NewRand(seed), def, def.Chars.SIZ+10)
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			if !res.Triggered {
				continue
			}
			if len(res.Rolls) == 0 {
				t.Fatalf("seed %d: no avoid-fall roll", seed)
			}
			// Recompute the outcome from the recorded rolls: Easy
			// Acrobatics first, then Standard Athletics.
			wantFall := true
			for _, nr := range res.Rolls {
				skill, err := rules.SkillValue(def, nr.Name)
				if err != nil {
					t.Fatalf("seed %d: %s: %v", seed, nr.Name, err)
				}
				grade := rules.DifficultyStandard
				if nr.Name == "Acrobatics" {
					grade = rules.DifficultyEasy
				}
				mod, err := grade.Modify(skill)
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				g, err := rules.Classify(nr.Roll.Total, mod)
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				if g >= rules.GradeSuccess {
					wantFall = false
					break
				}
			}
			if res.Fell != wantFall {
				t.Fatalf("seed %d: Fell=%v, want %v for rolls %+v", seed, res.Fell, wantFall, res.Rolls)
			}
			if res.Fell != def.HasCondition(char.CondProne) {
				t.Fatalf("seed %d: fell=%v but prone=%v", seed, res.Fell, def.HasCondition(char.CondProne))
			}
			if res.Fell {
				return // saw the falling branch
			}
		}
		t.Skip("no fall in the scanned seeds")
	})
	t.Run("refusals", func(t *testing.T) {
		_, _, _, def := duel(t, 5)
		if _, err := Knockback(dice.NewRand(5), def, -1); !errors.Is(err, char.ErrInvalidAmount) {
			t.Errorf("negative damage = %v, want char.ErrInvalidAmount", err)
		}
		if _, err := Knockback(dice.NewRand(5), nil, 5); err == nil {
			t.Error("nil victim must refuse")
		}
		if _, err := Knockback(nil, def, 5); err == nil {
			t.Error("nil rand must refuse")
		}
	})
}

// TestSurpriseState covers tables.md §4.12: −10 to Initiative, the
// flat-footed state until their initiative (SurpriseTurn), no stacking,
// and the offence ban lasting the Round.
func TestSurpriseState(t *testing.T) {
	t.Run("applies once and re-sorts", func(t *testing.T) {
		_, e, att, def := duel(t, 11)
		var before int
		for _, o := range e.Order {
			if o.Index == 1 {
				before = o.Score
			}
		}
		res, err := Surprise(e, def)
		if err != nil {
			t.Fatalf("Surprise: %v", err)
		}
		if !res.Applied || res.OrderAfter != before-10 || res.OrderBefore != before {
			t.Fatalf("surprise = %+v, want a single −10 from %d", res, before)
		}
		if !def.HasCondition(CondSurprised) {
			t.Fatal("condition missing")
		}
		// Second application does not stack the penalty.
		again, err := Surprise(e, def)
		if err != nil {
			t.Fatalf("second Surprise: %v", err)
		}
		if again.Applied || again.OrderAfter != before-10 {
			t.Errorf("penalty stacked: %+v", again)
		}
		// The surprised defender cannot defend until their initiative
		// arrives, but may still move.
		plan, err := planReaction(def, ReactionParry, false, false)
		if err != nil {
			t.Fatalf("planReaction: %v", err)
		}
		if _, blocked := reactionBlocked(e, def, plan); !blocked {
			t.Error("a surprised character must not defend")
		}
		if ok, _ := CanAct(e, att, "Attack"); !ok {
			t.Error("the ambusher attacks normally")
		}
	})
	t.Run("SurpriseTurn restores the defence but not the offence", func(t *testing.T) {
		_, e, _, def := duel(t, 12)
		if _, err := Surprise(e, def); err != nil {
			t.Fatalf("Surprise: %v", err)
		}
		plan, err := planReaction(def, ReactionParry, false, false)
		if err != nil {
			t.Fatalf("planReaction: %v", err)
		}
		res, err := SurpriseTurn(e, def)
		if err != nil {
			t.Fatalf("SurpriseTurn: %v", err)
		}
		if !res.DefenceRestored || !res.Applied {
			t.Fatalf("SurpriseTurn = %+v", res)
		}
		if _, blocked := reactionBlocked(e, def, plan); blocked {
			t.Error("the defence returns at their initiative")
		}
		if ok, why := CanAct(e, def, "Attack"); ok || !strings.Contains(strings.Join(why, " "), "surprised") {
			t.Errorf("the offence stays banned until the Round ends: %v", why)
		}
		// Idempotent.
		again, err := SurpriseTurn(e, def)
		if err != nil {
			t.Fatalf("second SurpriseTurn: %v", err)
		}
		if again.Applied {
			t.Error("SurpriseTurn should not re-mark an already restored state")
		}
	})
	t.Run("healthy characters", func(t *testing.T) {
		_, e, _, def := duel(t, 13)
		res, err := SurpriseTurn(e, def)
		if err != nil {
			t.Fatalf("SurpriseTurn: %v", err)
		}
		if res.DefenceRestored {
			t.Error("SurpriseTurn on a healthy character changed state")
		}
		if _, err := Surprise(e, def); err != nil {
			t.Fatalf("Surprise: %v", err)
		}
		if _, err := Surprise(e, def); err != nil {
			t.Fatalf("Surprise twice: %v", err)
		}
	})
}

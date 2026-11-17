package combat

import (
	"errors"
	"strings"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
	"github.com/maxwelljens/mythras-ultra/internal/engine/rules"
)

// TestApplyDamageOrder walks the manual damage path: armour absorbs
// first, the rest wounds through char.ApplyWound, the Serious-Wound 1d3
// ban is rolled, and a Major Wound imposes its unconditional states.
func TestApplyDamageOrder(t *testing.T) {
	t.Run("armour absorbs", func(t *testing.T) {
		_, _, _, def := duel(t, 1)
		wearMail(t, def) // AP 6
		res, err := ApplyDamage(dice.NewRand(1), def, "", 10)
		if err != nil {
			t.Fatalf("ApplyDamage: %v", err)
		}
		if res.LocationRoll.Expr != "1d20" || res.Location == "" {
			t.Fatalf("location not rolled: %+v", res)
		}
		if res.ArmorPoints != 6 || res.ArmorAbsorbed != 6 || res.DamageApplied != 4 {
			t.Errorf("AP=%d absorbed=%d applied=%d, want 6/6/4", res.ArmorPoints, res.ArmorAbsorbed, res.DamageApplied)
		}
		sev, err := def.SeverityAt(res.Location)
		if err != nil || sev != res.Severity {
			t.Errorf("severity %s, want %s (%v)", res.Severity, sev, err)
		}
	})
	t.Run("zero damage is no wound", func(t *testing.T) {
		_, _, _, def := duel(t, 2)
		res, err := ApplyDamage(dice.NewRand(2), def, "Chest", 0)
		if err != nil {
			t.Fatalf("ApplyDamage: %v", err)
		}
		if res.DamageApplied != 0 || res.Severity != char.SeverityNone {
			t.Errorf("applied=%d severity=%s, want 0/None", res.DamageApplied, res.Severity)
		}
	})
	t.Run("refusals", func(t *testing.T) {
		_, _, _, def := duel(t, 3)
		if _, err := ApplyDamage(dice.NewRand(3), def, "Chest", -1); !errors.Is(err, char.ErrInvalidAmount) {
			t.Errorf("negative damage = %v, want char.ErrInvalidAmount", err)
		}
		if _, err := ApplyDamage(dice.NewRand(3), def, "Tail", 5); !errors.Is(err, ErrInvalidChoice) {
			t.Errorf("unknown location = %v, want ErrInvalidChoice", err)
		}
		if _, err := ApplyDamage(nil, def, "Chest", 5); err == nil {
			t.Error("nil rand must refuse")
		}
	})
	t.Run("Serious Wound rolls the attack ban", func(t *testing.T) {
		_, _, _, def := duel(t, 4)
		d, err := char.Derive(def)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		// Chest start (7 for this build) exactly zeroes the location.
		res, err := ApplyDamage(dice.NewRand(4), def, "Chest", d.LocationHP["Chest"])
		if err != nil {
			t.Fatalf("ApplyDamage: %v", err)
		}
		if res.Severity != char.SeveritySerious {
			t.Fatalf("severity = %s, want Serious", res.Severity)
		}
		if res.NoAttacksRoll.Expr != "1d3" || res.NoAttacksRoll.Total < 1 {
			t.Errorf("ban roll = %+v, want a 1d3", res.NoAttacksRoll)
		}
		ban := findCondition(def, CondNoAttacks)
		if ban == nil || ban.Rounds != res.NoAttacksRoll.Total {
			t.Errorf("No Attacks = %+v, want %d rounds", ban, res.NoAttacksRoll.Total)
		}
	})
	t.Run("Major Wound imposes its states and rolls no consequence", func(t *testing.T) {
		// No Endurance consequence here: there is no attack roll to
		// oppose (see ApplyDamage's doc). The unconditional states stand.
		_, _, _, def := duel(t, 5)
		d, err := char.Derive(def)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		res, err := ApplyDamage(dice.NewRand(5), def, "Chest", 2*d.LocationHP["Chest"])
		if err != nil {
			t.Fatalf("ApplyDamage: %v", err)
		}
		if res.Severity != char.SeverityMajor {
			t.Fatalf("severity = %s, want Major", res.Severity)
		}
		for _, want := range []string{char.CondProne, char.CondIncapacitated, char.CondUnconscious} {
			if !def.HasCondition(want) {
				t.Errorf("missing %s after a torso Major Wound", want)
			}
		}
		if def.HasCondition(CondNoAttacks) {
			t.Error("Major Wound rolled the Serious-only attack ban")
		}
	})
}

// TestHealFirstAid: 1d3 at the most wounded untreated injury, once per
// specific injury, and a First Aid roll to staunch an active bleed.
func TestHealFirstAid(t *testing.T) {
	t.Run("heals the worst injury once", func(t *testing.T) {
		_, _, _, def := duel(t, 11)
		if _, _, err := def.ApplyWound("Chest", 5); err != nil {
			t.Fatalf("wound: %v", err)
		}
		res, err := Heal(dice.NewRand(11), def, HealFirstAid)
		if err != nil {
			t.Fatalf("First Aid: %v", err)
		}
		if len(res.Healed) != 1 || res.Healed[0].Location != "Chest" {
			t.Fatalf("healed = %+v, want the Chest", res.Healed)
		}
		if res.Healed[0].Amount < 1 || res.Healed[0].Amount > 3 {
			t.Errorf("amount = %d, want 1d3", res.Healed[0].Amount)
		}
		if findConditionAt(def, CondFirstAid, "Chest") == nil {
			t.Error("once-per-injury marker missing")
		}
		if _, err := Heal(dice.NewRand(12), def, HealFirstAid); !errors.Is(err, ErrHealUnavailable) {
			t.Errorf("second First Aid = %v, want ErrHealUnavailable", err)
		}
		// A fresh wound at the same location reopens the use — the
		// combat damage path clears the marker (ApplyDamage does what
		// ResolveAttack does).
		if _, err := ApplyDamage(dice.NewRand(13), def, "Chest", 1); err != nil {
			t.Fatalf("rewound: %v", err)
		}
		if _, err := Heal(dice.NewRand(13), def, "firstaid"); err != nil {
			t.Errorf("First Aid after a new wound: %v", err)
		}
	})
	t.Run("staunch needs a successful roll", func(t *testing.T) {
		for seed := int64(21); seed <= 400; seed++ {
			_, _, _, def := duel(t, seed)
			if _, _, err := def.ApplyWound("Chest", 1); err != nil {
				t.Fatalf("wound: %v", err)
			}
			def.AddCondition(char.Condition{Name: char.CondBleeding, Notes: "loss: 0"})
			res, err := Heal(dice.NewRand(seed), def, HealFirstAid)
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			if !res.Rolled {
				t.Fatalf("seed %d: a bleed must roll First Aid", seed)
			}
			staunched := !def.HasCondition(char.CondBleeding)
			if staunched != (res.Grade >= rules.GradeSuccess) {
				t.Fatalf("seed %d: grade %s but bleeding=%v", seed, res.Grade, staunched)
			}
			if staunched {
				if len(res.Facts) == 0 || !strings.Contains(res.Facts[0], "staunches") {
					t.Errorf("facts = %v", res.Facts)
				}
				return
			}
		}
		t.Skip("no successful First Aid roll in the scanned seeds")
	})
	t.Run("unknown method", func(t *testing.T) {
		_, _, _, def := duel(t, 14)
		if _, err := Heal(dice.NewRand(14), def, "bloodletting"); !errors.Is(err, ErrUnknownHealMethod) {
			t.Errorf("err = %v, want ErrUnknownHealMethod", err)
		}
	})
	t.Run("nothing wounded", func(t *testing.T) {
		_, _, _, def := duel(t, 15)
		if _, err := Heal(dice.NewRand(15), def, HealFirstAid); !errors.Is(err, ErrHealUnavailable) {
			t.Errorf("err = %v, want ErrHealUnavailable", err)
		}
	})
}

// TestHealNaturalPeriods: day heals Minor locations, week heals still-
// Serious ones, month refuses untreated Major Wounds until a successful
// Healing roll treats them, and the Blood Loss daily recovery runs only
// once bleeding has stopped.
func TestHealNaturalPeriods(t *testing.T) {
	t.Run("day heals Minor", func(t *testing.T) {
		_, _, _, def := duel(t, 21)
		d, err := char.Derive(def)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		if _, _, err := def.ApplyWound("Chest", 2); err != nil {
			t.Fatalf("wound: %v", err)
		}
		res, err := Heal(dice.NewRand(21), def, HealDay)
		if err != nil {
			t.Fatalf("day: %v", err)
		}
		want := d.HealingRate
		if wound := 2; want > wound {
			want = wound // clamped at the starting value
		}
		if res.Total != want {
			t.Errorf("healed %d, want %d (Healing Rate %d)", res.Total, want, d.HealingRate)
		}
	})
	t.Run("week heals Serious", func(t *testing.T) {
		_, _, _, def := duel(t, 22)
		d, err := char.Derive(def)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		if _, _, err := def.ApplyWound("Chest", d.LocationHP["Chest"]); err != nil {
			t.Fatalf("wound: %v", err)
		}
		res, err := Heal(dice.NewRand(22), def, HealWeek)
		if err != nil {
			t.Fatalf("week: %v", err)
		}
		if res.Total != d.HealingRate {
			t.Errorf("healed %d, want the Healing Rate %d", res.Total, d.HealingRate)
		}
		if sev, _ := def.SeverityAt("Chest"); sev == char.SeveritySerious && res.Total == 0 {
			t.Error("a Serious wound healed nothing")
		}
	})
	t.Run("month refuses untreated Major", func(t *testing.T) {
		_, _, _, def := duel(t, 23)
		d, err := char.Derive(def)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		if _, _, err := def.ApplyWound("Chest", 2*d.LocationHP["Chest"]); err != nil {
			t.Fatalf("wound: %v", err)
		}
		res, err := Heal(dice.NewRand(23), def, HealMonth)
		if err != nil {
			t.Fatalf("month: %v", err)
		}
		if res.Total != 0 {
			t.Errorf("an untreated Major Wound healed %d HP", res.Total)
		}
		joined := strings.Join(res.Facts, " ")
		if !strings.Contains(joined, "successful Healing roll") || !strings.Contains(joined, "maimed") {
			t.Errorf("facts = %v, want the treatment refusal and the maiming deadline", res.Facts)
		}
	})
	t.Run("treat unblocks Major healing", func(t *testing.T) {
		for seed := int64(31); seed <= 400; seed++ {
			_, _, _, def := duel(t, seed)
			d, err := char.Derive(def)
			if err != nil {
				t.Fatalf("derive: %v", err)
			}
			if _, _, err := def.ApplyWound("Chest", 2*d.LocationHP["Chest"]); err != nil {
				t.Fatalf("wound: %v", err)
			}
			treat, err := Heal(dice.NewRand(seed), def, HealTreat)
			if err != nil {
				t.Fatalf("seed %d: treat: %v", seed, err)
			}
			if !treat.Rolled {
				t.Fatalf("seed %d: treat must roll Healing", seed)
			}
			if treat.Grade < rules.GradeSuccess {
				continue // this seed failed the roll: month stays blocked
			}
			if !def.HasCondition(CondMajorTreated) {
				t.Fatalf("seed %d: successful Healing roll recorded nothing", seed)
			}
			month, err := Heal(dice.NewRand(seed+1), def, HealMonth)
			if err != nil {
				t.Fatalf("seed %d: month: %v", seed, err)
			}
			if month.Total != d.HealingRate {
				t.Fatalf("seed %d: treated Major healed %d, want %d", seed, month.Total, d.HealingRate)
			}
			return
		}
		t.Skip("no successful Healing roll in the scanned seeds")
	})
	t.Run("blood loss fatigue recovery", func(t *testing.T) {
		_, _, _, def := duel(t, 24)
		if _, _, err := def.ApplyWound("Chest", 1); err != nil {
			t.Fatalf("wound: %v", err)
		}
		// Bleeding: no recovery while the wound still bleeds.
		def.AddCondition(char.Condition{Name: char.CondBleeding, Notes: "loss: 0"})
		if _, err := addFatigueLevels(def, 2); err != nil {
			t.Fatalf("add fatigue: %v", err)
		}
		bleeding, err := Heal(dice.NewRand(24), def, HealDay)
		if err != nil {
			t.Fatalf("day: %v", err)
		}
		if bleeding.FatigueRecovered != 0 {
			t.Errorf("recovered %d levels while still bleeding", bleeding.FatigueRecovered)
		}
		// Staunch, then the daily recovery takes one level.
		def.RemoveCondition(char.CondBleeding)
		staunched, err := Heal(dice.NewRand(24), def, HealDay)
		if err != nil {
			t.Fatalf("day: %v", err)
		}
		if staunched.FatigueRecovered != 1 {
			t.Errorf("recovered %d levels, want 1", staunched.FatigueRecovered)
		}
	})
}

// TestFatiguePlumbing: combat writes the seam's "levels: N" Fatigue
// condition for Blood Loss and refuses malformed notes loudly.
func TestFatiguePlumbing(t *testing.T) {
	_, _, att, def := duel(t, 31)
	if !def.HasCondition(char.CondBleeding) {
		def.AddCondition(char.Condition{Name: char.CondBleeding, Notes: "loss: 0"})
	}
	got, err := accrueBleeding(def)
	if err != nil || got != 1 {
		t.Fatalf("accrueBleeding = %d, %v; want 1, nil", got, err)
	}
	got, err = accrueBleeding(def)
	if err != nil || got != 2 {
		t.Fatalf("second accrual = %d, %v; want 2, nil", got, err)
	}
	if n := recoverFatigueLevel(def); n != 1 {
		t.Errorf("recoverFatigueLevel = %d, want 1", n)
	}
	// A healthy character bleeds nothing.
	if n, err := accrueBleeding(att); err != nil || n != 0 {
		t.Errorf("accrueBleeding on a healthy target = %d, %v; want 0, nil", n, err)
	}
	// Malformed persistence slot fails loudly (rules.ErrInvalidFatigue).
	def.AddCondition(char.Condition{Name: rules.FatigueCondition, Notes: "garbage"})
	if _, err := accrueBleeding(def); !errors.Is(err, rules.ErrInvalidFatigue) {
		t.Errorf("malformed Fatigue notes = %v, want rules.ErrInvalidFatigue", err)
	}
}

// TestWithdrawImpaled: the Ready Weapon withdrawal rolls Brawn (opposed
// when the victim resists), deals half the normal damage with no DM and
// no armour reduction on success, and leaves the weapon stuck on failure.
func TestWithdrawImpaled(t *testing.T) {
	setup := func(t *testing.T, seed int64) (*dice.Rand, *Encounter, *char.Character, *char.Character) {
		t.Helper()
		r, e, att, def := duel(t, seed)
		if _, _, err := def.ApplyWound("Chest", 3); err != nil {
			t.Fatalf("wound: %v", err)
		}
		att.AddCondition(char.Condition{Name: CondImpaled, Notes: "weapon=Broadsword; lodged in Defender"})
		def.AddCondition(char.Condition{Name: CondImpaled, Location: "Chest", Notes: "weapon=Broadsword; grade=formidable"})
		return r, e, att, def
	}
	t.Run("not impaled", func(t *testing.T) {
		r, e, att, def := duel(t, 41)
		if _, err := WithdrawImpaled(r, e, att, def, "Broadsword", false); !errors.Is(err, ErrEffectUnavailable) {
			t.Errorf("err = %v, want ErrEffectUnavailable", err)
		}
	})
	t.Run("success pulls it free and wounds", func(t *testing.T) {
		for seed := int64(42); seed <= 400; seed++ {
			r, e, att, def := setup(t, seed)
			before := def.CurrentHP["Chest"]
			res, err := WithdrawImpaled(r, e, att, def, "Broadsword", false)
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			if !res.Success {
				continue
			}
			if res.Location != "Chest" {
				t.Errorf("location = %q", res.Location)
			}
			if res.DamageRoll.Total <= 0 || res.DamageApplied <= 0 {
				t.Errorf("withdrawal damage = %+v", res)
			}
			if def.CurrentHP["Chest"] != before-res.DamageApplied {
				t.Errorf("Chest HP = %d, want %d", def.CurrentHP["Chest"], before-res.DamageApplied)
			}
			if att.HasCondition(CondImpaled) || findConditionAt(def, CondImpaled, "Chest") != nil {
				t.Error("the weapon came free but both Impaled records remain")
			}
			if len(res.Facts) == 0 || !strings.Contains(strings.Join(res.Facts, " "), "pulls the Broadsword free") {
				t.Errorf("facts = %v", res.Facts)
			}
			return
		}
		t.Skip("no successful Brawn withdrawal in the scanned seeds")
	})
	t.Run("failure leaves it stuck", func(t *testing.T) {
		for seed := int64(42); seed <= 400; seed++ {
			r, e, att, def := setup(t, seed)
			before := def.CurrentHP["Chest"]
			res, err := WithdrawImpaled(r, e, att, def, "Broadsword", false)
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			if res.Success {
				continue
			}
			if def.CurrentHP["Chest"] != before {
				t.Errorf("a failed withdrawal dealt damage")
			}
			if !att.HasCondition(CondImpaled) {
				t.Error("the weapon unstuck on a failure")
			}
			if !strings.Contains(strings.Join(res.Facts, " "), "stays stuck") {
				t.Errorf("facts = %v", res.Facts)
			}
			return
		}
		t.Skip("no failed Brawn withdrawal in the scanned seeds")
	})
	t.Run("opposed when resisted", func(t *testing.T) {
		r, e, att, def := setup(t, 44)
		res, err := WithdrawImpaled(r, e, att, def, "Broadsword", true)
		if err != nil {
			t.Fatalf("opposed withdrawal: %v", err)
		}
		if res.Opposed.A.Grade == 0 && res.Opposed.B.Grade == 0 && res.Opposed.Winner == 0 {
			// A true tie of grade and roll can produce Winner 0, but both
			// sides must have rolled.
			if res.Opposed.A.Roll == 0 {
				t.Fatalf("no opposed rolls recorded: %+v", res)
			}
		}
		if res.Opposed.A.Roll == 0 || res.Opposed.B.Roll == 0 {
			t.Fatalf("opposed rolls missing: %+v", res.Opposed)
		}
	})
}

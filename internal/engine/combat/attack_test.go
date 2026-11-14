package combat

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/dice"
	"github.com/mythras-ultra/myul/internal/engine/rules"
)

// TestResolveAttackPipelineInvariants walks a batch of seeds through the
// full close-combat pipeline and checks the audit against the state it
// claims: outcome classification, the Differential Roll, AP payment, the
// damage application order (damage → parry Size → armour → wound) and
// effect availability restricted to the winning side.
func TestResolveAttackPipelineInvariants(t *testing.T) {
	for seed := int64(1); seed <= 30; seed++ {
		_, e, att, def := duel(t, seed)
		wearMail(t, def) // AP 6 at every location
		before, err := char.Derive(def)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		res, err := ResolveAttack(dice.NewRand(seed), e, att, "Broadsword", def, ReactionParry)
		if err != nil {
			t.Fatalf("seed %d: ResolveAttack: %v", seed, err)
		}

		// Attack roll audit.
		if res.AttackRoll.Expr != "1d100" || len(res.AttackRoll.Dice) != 1 {
			t.Fatalf("seed %d: attack roll = %+v", seed, res.AttackRoll)
		}
		if !res.AutoAttack {
			g, err := rules.Classify(res.AttackRoll.Total, res.AttackSkill)
			if err != nil || g != res.AttackOutcome.Grade {
				t.Errorf("seed %d: outcome %s for roll %d vs skill %d (classify: %v)",
					seed, res.AttackOutcome.Grade, res.AttackRoll.Total, res.AttackSkill, err)
			}
		}

		// The differential: rules.Differential is the seam the digest
		// names for combat comparison (How Combat Works step 3).
		want := rules.Differential(res.AttackOutcome, res.ReactionOutcome)
		if res.Contest.Winner != want.Winner || res.Contest.Margin != want.Margin {
			t.Errorf("seed %d: contest %+v, want %+v", seed, res.Contest, want)
		}
		if res.SpecialEffects != res.Contest.Margin {
			t.Errorf("seed %d: SpecialEffects = %d, want margin %d", seed, res.SpecialEffects, res.Contest.Margin)
		}
		wantWinner := WinnerNone
		switch want.Winner {
		case 1:
			wantWinner = WinnerAttacker
		case 2:
			wantWinner = WinnerDefender
		}
		if res.Winner != wantWinner {
			t.Errorf("seed %d: winner %q, want %q", seed, res.Winner, wantWinner)
		}

		// AP: the attacker always pays for the Attack; the defender pays
		// for the Parry when one was actually rolled.
		if res.APSpent != 1 || e.AP[0] != 1 {
			t.Errorf("seed %d: APSpent=%d left=%d, want 1/1", seed, res.APSpent, e.AP[0])
		}
		if res.ReactionAutoFail {
			if res.ReactionReason == "" {
				t.Errorf("seed %d: auto-failed reaction without a reason", seed)
			}
		} else if res.ReactionAPSpent != 1 || e.AP[1] != 1 {
			t.Errorf("seed %d: ReactionAPSpent=%d left=%d, want 1/1", seed, res.ReactionAPSpent, e.AP[1])
		}

		// Hit ⇒ location and damage audit; miss ⇒ nothing rolled.
		if !res.Hit {
			if res.Location != "" || res.DamageRoll.Total != 0 {
				t.Errorf("seed %d: miss still rolled location/damage: %+v", seed, res)
			}
			continue
		}
		if res.Location == "" || res.LocationRoll.Expr != "1d20" {
			t.Fatalf("seed %d: hit without location audit: %+v", seed, res)
		}
		if res.DamageRoll.Total == 0 {
			t.Fatalf("seed %d: hit without damage roll", seed)
		}
		if !res.Damaged {
			t.Errorf("seed %d: damage path ran but Damaged is false", seed)
		}

		// Damage application order (tables.md §3.9): DM into DamagePre,
		// parry Size reduction, armour, then the wound.
		if res.DamagePre < res.DamageRoll.Total+res.DamageModifierRoll.Total && res.LongRange {
			t.Errorf("seed %d: long-range damage not halved: pre=%d roll=%d dm=%d",
				seed, res.DamagePre, res.DamageRoll.Total, res.DamageModifierRoll.Total)
		}
		afterParry := res.DamagePre - res.ParryReduction
		if afterParry < 0 {
			t.Fatalf("seed %d: parry over-reduced: pre=%d red=%d", seed, res.DamagePre, res.ParryReduction)
		}
		wantAbsorbed := min(afterParry, res.ArmorPoints)
		if res.ArmorAbsorbed != wantAbsorbed {
			t.Errorf("seed %d: armour absorbed %d, want %d (afterParry=%d AP=%d)",
				seed, res.ArmorAbsorbed, wantAbsorbed, afterParry, res.ArmorPoints)
		}
		wantApplied := afterParry - res.ArmorAbsorbed
		if res.DamageApplied != wantApplied {
			t.Errorf("seed %d: applied %d, want %d", seed, res.DamageApplied, wantApplied)
		}
		sev, err := def.SeverityAt(res.Location)
		if err != nil {
			t.Fatalf("seed %d: SeverityAt: %v", seed, err)
		}
		if res.DamageApplied > 0 {
			if sev != res.Severity {
				t.Errorf("seed %d: severity %s, want %s", seed, res.Severity, sev)
			}
		} else if res.Severity != char.SeverityNone {
			t.Errorf("seed %d: no damage applied but severity recorded: %s", seed, res.Severity)
		}
		_ = before

		// Effects: only printed rows, only the winning side.
		for _, name := range res.AvailableSpecialEffects {
			fx, ok := EffectByName(name)
			if !ok {
				t.Errorf("seed %d: available effect %q is not printed", seed, name)
			}
			side, sideOK := effectSide(fx, &res, res.AttackOutcome.Grade == rules.GradeFumble && res.WeaponFacts.Firearm)
			if !sideOK {
				t.Errorf("seed %d: effect %q offered to the losing side", seed, name)
			}
			if !effectGates(fx, &res, side, false) {
				t.Errorf("seed %d: effect %q offered with its gate shut", seed, name)
			}
		}
		if res.Winner == WinnerNone && len(res.AvailableSpecialEffects) > 0 {
			t.Errorf("seed %d: no winner but effects offered: %v", seed, res.AvailableSpecialEffects)
		}
	}
}

// TestResolveAttackMissEndsExchange: a failed attack with no reaction
// rolls nothing and grants no benefit (How Combat Works; Unsuccessful
// Rolls and Fumbles).
func TestResolveAttackMissEndsExchange(t *testing.T) {
	for seed := int64(1); seed <= 200; seed++ {
		_, e, att, def := duel(t, seed)
		res, err := ResolveAttack(dice.NewRand(seed), e, att, "Broadsword", def, ReactionNone)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if res.AttackOutcome.Grade >= rules.GradeSuccess || res.ReactionOutcome.Grade >= rules.GradeSuccess {
			continue // not a mutual failure
		}
		if res.Winner != WinnerNone {
			t.Errorf("seed %d: both failed but winner = %q", seed, res.Winner)
		}
		if res.Hit || res.Location != "" || res.DamageRoll.Total != 0 || res.Damaged {
			t.Errorf("seed %d: failed attack still damaged: %+v", seed, res)
		}
		if !res.ReactionAutoFail || !strings.Contains(res.ReactionReason, "automatic Failure") {
			t.Errorf("seed %d: decline reason = %q", seed, res.ReactionReason)
		}
		if len(res.AvailableSpecialEffects) != 0 {
			t.Errorf("seed %d: mutual failure offered effects: %v", seed, res.AvailableSpecialEffects)
		}
		return
	}
	t.Skip("no mutual failure in the scanned seeds")
}

// TestParryReductionTable checks the printed Damage Reduction by Parry
// table (tables.md §4.5): equal or greater Size deflects all, one Size
// less deflects half (damage halves round up), two or more less none.
func TestParryReductionTable(t *testing.T) {
	cases := []struct {
		dmg               int
		attack, parrySize string
		want              int
	}{
		{10, "M", "M", 10}, // equal: all
		{10, "M", "L", 10}, // greater: all
		{9, "M", "S", 4},   // one less: deflect half, damage halves up (5 gets through)
		{10, "M", "S", 5},
		{10, "L", "S", 0}, // two or more: none
		{10, "H", "M", 0}, // two less: none
		{10, "M", "", 0},  // barehanded: nothing to compare
		{0, "M", "S", 0},
	}
	for _, tc := range cases {
		if got := parryReduction(tc.dmg, tc.attack, tc.parrySize); got != tc.want {
			t.Errorf("parryReduction(%d, %q, %q) = %d, want %d", tc.dmg, tc.attack, tc.parrySize, got, tc.want)
		}
	}
}

// TestResolveAttackDodgeWinAvoidsBlow: a winning Evade avoids the damage
// completely and leaves the defender prone (Combat › Evading).
func TestResolveAttackDodgeWinAvoidsBlow(t *testing.T) {
	for seed := int64(1); seed <= 300; seed++ {
		_, e, att, def := duel(t, seed)
		hpBefore, err := char.Derive(def)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		res, err := ResolveAttack(dice.NewRand(seed), e, att, "Broadsword", def, ReactionDodge)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if !res.Hit {
			if !def.HasCondition(char.CondProne) {
				t.Fatalf("seed %d: Evade was spent but the defender is not prone", seed)
			}
			continue
		}
		if res.Winner != WinnerDefender {
			continue
		}
		if res.DamageApplied != 0 || res.Severity != char.SeverityNone {
			t.Fatalf("seed %d: winning dodge still wounded: %+v", seed, res)
		}
		for loc, start := range hpBefore.LocationHP {
			if def.CurrentHP[loc] != 0 && def.CurrentHP[loc] != start {
				t.Errorf("seed %d: %s lost HP despite the dodge", seed, loc)
			}
		}
		if !def.HasCondition(char.CondProne) {
			t.Errorf("seed %d: Evade should leave the defender prone", seed)
		}
		return
	}
	t.Skip("no winning dodge in the scanned seeds")
}

// TestResolveAttackCriticalGrantsEffects: an attacker Critical that wins
// the differential offers the Critical-gated offensive rows.
func TestResolveAttackCriticalGrantsEffects(t *testing.T) {
	for seed := int64(1); seed <= 400; seed++ {
		_, e, att, def := duel(t, seed)
		res, err := ResolveAttack(dice.NewRand(seed), e, att, "Broadsword", def, ReactionParry)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if res.AttackOutcome.Grade != rules.GradeCritical || res.Winner != WinnerAttacker {
			continue
		}
		if res.SpecialEffects < 1 {
			t.Fatalf("seed %d: critical win with no Special Effects", seed)
		}
		for _, want := range []string{"Maximize Damage", "Bypass Armor", "Circumvent Parry"} {
			if !containsString(res.AvailableSpecialEffects, want) {
				t.Errorf("seed %d: critical attacker lacks %q (has %v)", seed, want, res.AvailableSpecialEffects)
			}
		}
		return
	}
	t.Skip("no attacker critical in the scanned seeds")
}

// TestFirearmFumbleGivesTargetThePick walks the Fumbling with Firearms
// rule: the firearm jams on the fumble only when the intended target
// applies Weapon Malfunction, the fumble-specific rows are offered even
// though the Differential grants nothing, and a jammed weapon refuses to
// fire until cleaned.
func TestFirearmFumbleGivesTargetThePick(t *testing.T) {
	var seed int64
	var res AttackResult
	var r *dice.Rand
	var e *Encounter
	var att, def *char.Character
	found := false
	for seed = 1; seed <= 400; seed++ {
		att = fighter(t, "gun-a", "Gunner")
		def = fighter(t, "gun-b", "Target")
		learnStyle(t, att, "Guns", 65, "Rifle")
		giveWeapon(t, att, "Rifle")
		giveAmmo(t, att, "Rifle rounds", 5)
		var err error
		r, e, _, _ = duelBetween(t, seed, att, def)
		res, err = ResolveAttack(r, e, att, "Rifle", def, ReactionParry)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if res.AttackOutcome.Grade == rules.GradeFumble {
			found = true
			break
		}
	}
	if !found {
		t.Skip("no firearm fumble in the scanned seeds")
	}
	if !res.Malfunction {
		t.Errorf("firearm fumble must set the Malfunction audit flag")
	}
	if !res.ReactionAutoFail || !strings.Contains(res.ReactionReason, "firearms cannot be actively parried") {
		t.Errorf("firearms must refuse an active parry, got autoFail=%v reason=%q", res.ReactionAutoFail, res.ReactionReason)
	}
	if res.SpecialEffects != 1 {
		t.Errorf("fumble picks = %d, want 1 (Fumbling with Firearms)", res.SpecialEffects)
	}
	for _, want := range []string{"Accidental Injury", "Select Target", "Weapon Malfunction"} {
		if !containsString(res.AvailableSpecialEffects, want) {
			t.Errorf("target lacks fumble pick %q (has %v)", want, res.AvailableSpecialEffects)
		}
	}
	// The shooter is not jammed until the target applies the effect...
	if att.HasCondition(CondMalfunction) {
		t.Fatal("fumble jammed the weapon before the target picked anything")
	}
	// ...and the target is not a participant problem: effects need the
	// encounter running and the recorded identities.
	if _, err := ApplySpecialEffect(r, e, "Weapon Malfunction", att, def, &res, EffectOptions{}); err != nil {
		t.Fatalf("apply Weapon Malfunction: %v", err)
	}
	if !att.HasCondition(CondMalfunction) {
		t.Fatal("Weapon Malfunction did not jam the rifle")
	}
	// A jammed firearm refuses to fire (ErrMalfunction), before any AP
	// is spent.
	apBefore := e.AP[0]
	if _, err := ResolveAttack(r, e, att, "Rifle", def, ReactionNone); !errors.Is(err, ErrMalfunction) {
		t.Fatalf("firing a jammed rifle = %v, want ErrMalfunction", err)
	}
	if e.AP[0] != apBefore {
		t.Errorf("AP spent on a refused shot: %d → %d", apBefore, e.AP[0])
	}
}

// TestAmmunitionLifecycle: ranged fire consumes one round on a shot that
// actually fires, and a dry quiver refuses the attack through the char
// sentinel before any AP moves.
func TestAmmunitionLifecycle(t *testing.T) {
	att := fighter(t, "arch-a", "Archer")
	def := fighter(t, "arch-b", "Dummy")
	learnStyle(t, att, "Bow", 70, "Bow")
	giveWeapon(t, att, "Bow")
	giveAmmo(t, att, "Arrows", 1)
	r, e, _, _ := duelBetween(t, 3, att, def)

	res, err := ResolveAttack(r, e, att, "Bow", def, ReactionNone)
	if err != nil {
		t.Fatalf("first shot: %v", err)
	}
	if res.AmmoConsumed != 1 {
		t.Errorf("AmmoConsumed = %d, want 1", res.AmmoConsumed)
	}
	// ConsumeAmmo drops the emptied stack, so the quiver reads as "no
	// ammunition at all" once the last arrow goes.
	left, err := att.ArrowsLeft()
	if left != 0 && !errors.Is(err, char.ErrNoAmmo) {
		t.Fatalf("ArrowsLeft = %d, %v; want 0 or ErrNoAmmo", left, err)
	}
	if errors.Is(err, char.ErrNoAmmo) && left != 0 {
		t.Fatalf("ArrowsLeft = %d with an error", left)
	}
	apBefore := e.AP[0]
	if _, err := ResolveAttack(r, e, att, "Bow", def, ReactionNone); !errors.Is(err, char.ErrNoAmmo) {
		t.Fatalf("second shot = %v, want char.ErrNoAmmo", err)
	}
	if e.AP[0] != apBefore {
		t.Errorf("AP spent on a refused shot: %d → %d", apBefore, e.AP[0])
	}
	if res.AmmoConsumed != 1 || res.Ranged != true {
		t.Errorf("shot audit: ranged=%v consumed=%d", res.Ranged, res.AmmoConsumed)
	}
}

// TestRangedBandsAndLongRange: distances map onto the printed
// Close/Effective/Long bands; the Long band halves the damage (rounding
// up) and steps Force down one step.
func TestRangedBandsAndLongRange(t *testing.T) {
	bow, err := char.WeaponByName("Bow")
	if err != nil {
		t.Fatalf("WeaponByName: %v", err)
	}
	w := infoFromStats(bow.Name, bow.Stats, bow.Tags)
	band, long := rangeBand(w, 10)
	if band != "close" || long {
		t.Errorf("10 m → %q/%v, want close/false", band, long)
	}
	band, long = rangeBand(w, 60)
	if band != "effective" || long {
		t.Errorf("60 m → %q/%v, want effective/false", band, long)
	}
	band, long = rangeBand(w, 150)
	if band != "long" || !long {
		t.Errorf("150 m → %q/%v, want long/true", band, long)
	}
	if got := effectiveAttackSize(w, AttackParams{Distance: 150}); got != "M" {
		t.Errorf("long-range Force step: %q, want M (printed L − 1 step)", got)
	}
	if got := effectiveAttackSize(w, AttackParams{Distance: 60}); got != "L" {
		t.Errorf("effective Force: %q, want L", got)
	}

	// End-to-end: a Long-Range shot halves its rolled damage.
	for seed := int64(1); seed <= 100; seed++ {
		att := fighter(t, "arch-a", "Archer")
		def := fighter(t, "arch-b", "Dummy")
		learnStyle(t, att, "Bow", 75, "Bow")
		giveWeapon(t, att, "Bow")
		giveAmmo(t, att, "Arrows", 3)
		r, e, _, _ := duelBetween(t, seed, att, def)
		res, err := ResolveAttackWith(r, e, AttackParams{
			Attacker: att, Defender: def, Weapon: "Bow", Reaction: ReactionNone,
			Distance: 150, Grade: rules.DifficultyStandard,
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if !res.Hit || res.DamageRoll.Total == 0 {
			continue
		}
		sum := res.DamageRoll.Total + res.DamageModifierRoll.Total
		if res.DamagePre != ceilHalf(sum) {
			t.Errorf("seed %d: long-range pre-damage %d, want ceilHalf(%d)=%d", seed, res.DamagePre, sum, ceilHalf(sum))
		}
		if !strings.Contains(strings.Join(res.Facts, " "), "Long Range") {
			t.Errorf("seed %d: long-range shot announced no fact", seed)
		}
		return
	}
	t.Skip("no long-range hit in the scanned seeds")
}

// TestParseRangedGrade covers the Ranged Situational Modifiers table and
// the AMBIGUOUS "Impossible" footnote: Herculean when the target's
// general location is known, refused as Hopeless when it is not.
func TestParseRangedGrade(t *testing.T) {
	cases := []struct {
		name    string
		known   bool
		want    rules.DifficultyGrade
		wantErr bool
	}{
		{"Impossible", true, rules.DifficultyHerculean, false},
		{"impossible", false, rules.DifficultyHopeless, true},
		{"light wind", false, rules.DifficultyHard, false},
		{"moderate wind", false, rules.DifficultyFormidable, false},
		{"strong wind", false, rules.DifficultyHerculean, false},
		{"Gale, Storm, or Worse", false, rules.DifficultyHopeless, false},
		{"target is running", false, rules.DifficultyHard, false},
		{"target is sprinting", false, rules.DifficultyFormidable, false},
		{"Target completely invisible", true, rules.DifficultyHerculean, false},
		{"attacker prone", false, rules.DifficultyHerculean, false},
		{"attacker on unstable ground", false, rules.DifficultyHard, false},
		{"hard", false, rules.DifficultyHard, false}, // plain grade names pass through
		{"nonsense situation", false, rules.DifficultyStandard, true},
	}
	for _, tc := range cases {
		got, err := ParseRangedGrade(tc.name, tc.known)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseRangedGrade(%q, %v) = %v, want an error", tc.name, tc.known, got)
			}
			if got != tc.want {
				t.Errorf("ParseRangedGrade(%q, %v) grade = %v, want %v", tc.name, tc.known, got, tc.want)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("ParseRangedGrade(%q, %v) = %v, %v; want %v, nil", tc.name, tc.known, got, err, tc.want)
		}
	}
}

// TestAttackDifficultyNotes checks the situational grade plumbing: a
// charge is Hard, prone is Formidable (hardest wins, nothing stacks) and
// a Hopeless attack refuses outright while Automatic needs no roll.
func TestAttackDifficultyNotes(t *testing.T) {
	t.Run("charge plus prone is Formidable", func(t *testing.T) {
		_, e, att, def := duel(t, 5)
		att.AddCondition(char.Condition{Name: char.CondProne})
		res, err := ResolveAttackWith(dice.NewRand(5), e, AttackParams{
			Attacker: att, Defender: def, Weapon: "Broadsword",
			Reaction: ReactionNone, Charging: true,
		})
		if err != nil {
			t.Fatalf("ResolveAttackWith: %v", err)
		}
		if res.Grade != rules.DifficultyFormidable {
			t.Errorf("grade = %v, want formidable (charge hard loses to prone)", res.Grade)
		}
		notes := strings.Join(res.GradeNotes, "; ")
		if !strings.Contains(notes, "charge: hard") || !strings.Contains(notes, "prone") {
			t.Errorf("grade notes %q must record both situations", notes)
		}
	})
	t.Run("Automatic needs no roll", func(t *testing.T) {
		_, e, att, def := duel(t, 6)
		res, err := ResolveAttackWith(dice.NewRand(6), e, AttackParams{
			Attacker: att, Defender: def, Weapon: "Broadsword",
			Reaction: ReactionNone, Grade: rules.DifficultyAutomatic,
		})
		if err != nil {
			t.Fatalf("ResolveAttackWith: %v", err)
		}
		if !res.AutoAttack || res.AttackRoll.Total != 0 {
			t.Errorf("autoAttack=%v roll=%d, want an unrolled success", res.AutoAttack, res.AttackRoll.Total)
		}
		if !res.Hit {
			t.Error("Automatic attack did not land")
		}
	})
	t.Run("Hopeless refuses", func(t *testing.T) {
		_, e, att, def := duel(t, 7)
		_, err := ResolveAttackWith(dice.NewRand(7), e, AttackParams{
			Attacker: att, Defender: def, Weapon: "Broadsword",
			Reaction: ReactionNone, Grade: rules.DifficultyHopeless,
		})
		if !errors.Is(err, rules.ErrHopeless) {
			t.Errorf("err = %v, want rules.ErrHopeless", err)
		}
		if e.AP[0] != 2 {
			t.Errorf("refused attack spent AP: %d", e.AP[0])
		}
	})
	t.Run("weapon outside combat style costs a grade", func(t *testing.T) {
		att := fighter(t, "oos-a", "Outsider")
		def := fighter(t, "oos-b", "Defender")
		learnStyle(t, att, "Spear Art", 70, "1H Spear")
		giveWeapon(t, att, "Broadsword") // not in the style
		learnStyle(t, def, "Broadsword Art", 60, "Broadsword")
		giveWeapon(t, def, "Broadsword")
		r, e, _, _ := duelBetween(t, 8, att, def)
		res, err := ResolveAttack(r, e, att, "Broadsword", def, ReactionNone)
		if err != nil {
			t.Fatalf("ResolveAttack: %v", err)
		}
		if res.Grade != rules.DifficultyHard {
			t.Errorf("grade = %v, want hard", res.Grade)
		}
		if !strings.Contains(strings.Join(res.GradeNotes, "; "), "outside combat style") {
			t.Errorf("missing out-of-style note: %v", res.GradeNotes)
		}
	})
}

// TestSurpriseBonusEffect: the first successful attack against a
// surprised target gains one bonus Special Effect, once.
func TestSurpriseBonusEffect(t *testing.T) {
	for seed := int64(1); seed <= 300; seed++ {
		_, e, att, def := duel(t, seed)
		if _, err := Surprise(e, def); err != nil {
			t.Fatalf("Surprise: %v", err)
		}
		res, err := ResolveAttack(dice.NewRand(seed), e, att, "Broadsword", def, ReactionParry)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if !res.Hit || res.Winner != WinnerAttacker {
			continue
		}
		if res.SpecialEffects != res.Contest.Margin+1 {
			t.Fatalf("seed %d: effects = %d, want margin %d + surprise bonus", seed, res.SpecialEffects, res.Contest.Margin)
		}
		// The surprised defender could not defend.
		if !res.ReactionAutoFail || !strings.Contains(res.ReactionReason, "flat-footed") {
			t.Errorf("seed %d: surprised defender defended: %v %q", seed, res.ReactionAutoFail, res.ReactionReason)
		}
		// A second hit does not gain the bonus again.
		res2, err := ResolveAttack(dice.NewRand(seed+1), e, att, "Broadsword", def, ReactionParry)
		if err != nil {
			t.Fatalf("second attack: %v", err)
		}
		if res2.Hit && res2.SpecialEffects > res2.Contest.Margin {
			t.Errorf("bonus granted twice: %d > %d", res2.SpecialEffects, res2.Contest.Margin)
		}
		return
	}
	t.Skip("no surprise hit in the scanned seeds")
}

// TestResolveAttackDeterminism: the same seed and the same call sequence
// produce byte-identical audits and identical state.
func TestResolveAttackDeterminism(t *testing.T) {
	run := func() ([]byte, []byte) {
		_, e, att, def := duel(t, 99)
		wearMail(t, def)
		res, err := ResolveAttack(dice.NewRand(99), e, att, "Broadsword", def, ReactionParry)
		if err != nil {
			t.Fatalf("ResolveAttack: %v", err)
		}
		rawRes, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		rawState, err := json.Marshal([]any{e.Status(), def.CurrentHP, def.Conditions})
		if err != nil {
			t.Fatalf("marshal state: %v", err)
		}
		return rawRes, rawState
	}
	aRes, aState := run()
	bRes, bState := run()
	if string(aRes) != string(bRes) {
		t.Fatalf("results differ:\n%s\n%s", aRes, bRes)
	}
	if string(aState) != string(bState) {
		t.Fatalf("state differs:\n%s\n%s", aState, bState)
	}
}

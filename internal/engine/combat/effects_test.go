package combat

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
	"github.com/maxwelljens/mythras-ultra/internal/engine/rules"
)

// ---------------------------------------------------------------------------
// the Special Effects Table
// ---------------------------------------------------------------------------

// TestSpecialEffectsTable: all 32 printed rows are present, unique, and
// carry their side, weapon-type, Specific Roll and stack columns.
func TestSpecialEffectsTable(t *testing.T) {
	rows := SpecialEffects()
	if len(rows) != 32 {
		t.Fatalf("SpecialEffects() = %d rows, want 32", len(rows))
	}
	seen := map[string]bool{}
	for _, fx := range rows {
		if seen[fx.Name] {
			t.Errorf("duplicate row %q", fx.Name)
		}
		seen[fx.Name] = true
		if fx.Citation == "" {
			t.Errorf("row %q missing citation", fx.Name)
		}
		if !fx.Offensive && !fx.Defensive {
			t.Errorf("row %q is on neither side", fx.Name)
		}
	}
	// Spot-check the printed columns (tables.md §4.6).
	want := map[string]struct {
		off, def, stack          bool
		weaponType, specificRoll string
	}{
		"Accidental Injury":  {false, true, false, "", "Attacker Fumbles"},
		"Bash":               {true, false, false, "Shields or Bludgeoning", ""},
		"Bleed":              {true, false, false, "Cutting Weapons", "Firearms: Attacker Criticals"},
		"Bypass Armor":       {true, false, true, "", "Attacker Criticals"},
		"Enhance Parry":      {false, true, false, "", "Defender Criticals"},
		"Force Failure":      {true, true, false, "", "Opponent Fumbles"},
		"Impale":             {true, false, false, "Impaling Weapons", ""},
		"Maximize Damage":    {true, false, true, "", "Attacker Criticals"},
		"Sunder":             {true, false, false, "Two Handed Weapons", ""},
		"Weapon Malfunction": {false, true, false, "Firearms Only", "Attacker Fumbles"},
		"Withdraw":           {false, true, false, "", ""},
	}
	for name, w := range want {
		fx, ok := EffectByName(name)
		if !ok {
			t.Errorf("row %q missing", name)
			continue
		}
		if fx.Offensive != w.off || fx.Defensive != w.def || fx.Stackable != w.stack ||
			fx.WeaponType != w.weaponType || fx.SpecificRoll != w.specificRoll {
			t.Errorf("row %q = %+v, want columns off=%v def=%v stack=%v %q/%q",
				name, fx, w.off, w.def, w.stack, w.weaponType, w.specificRoll)
		}
	}
	if _, ok := EffectByName("MINIMISE DAMAGE"); ok {
		t.Error("the source prints no Minimise Damage row")
	}
	if _, ok := EffectByName("impaLE"); !ok {
		t.Error("EffectByName must match case-insensitively")
	}
	// The copy must not alias the table.
	rows[0].Name = "mutated"
	if again, _ := EffectByName("Accidental Injury"); again.Name != "Accidental Injury" {
		t.Fatal("SpecialEffects() leaked the internal table")
	}
}

// ---------------------------------------------------------------------------
// availability gating (computeAvailable)
// ---------------------------------------------------------------------------

// gateFixture builds an exchange result for gating checks (stationary =
// the ComputeAvailable flag for Choose Location's Close Range clause).
func gateFixture(mutate func(*AttackResult)) []string {
	return gateFixtureAt(mutate, false)
}

// gateFixtureAt is gateFixture with an explicit stationary flag.
func gateFixtureAt(mutate func(*AttackResult), stationary bool) []string {
	res := &AttackResult{
		Round:    1,
		Attacker: "gate-att", Defender: "gate-def",
		Weapon:          "Broadsword",
		WeaponFacts:     WeaponFacts{Name: "Broadsword", Damage: "1d8", Size: "M", DMAdds: true},
		ReactionWeapon:  WeaponFacts{Name: "Broadsword", Damage: "1d8", Size: "M", DMAdds: true},
		AttackSkill:     70,
		AttackRoll:      dice.Roll{Expr: "1d100", Total: 40},
		AttackOutcome:   rules.Outcome{Roll: 40, Skill: 70, Grade: rules.GradeSuccess},
		Reaction:        ReactionParry,
		ReactionSkill:   70,
		ReactionRoll:    dice.Roll{Expr: "1d100", Total: 85},
		ReactionOutcome: rules.Outcome{Roll: 85, Skill: 70, Grade: rules.GradeFailure},
		ParriedWith:     "Broadsword",
		Winner:          WinnerAttacker,
		SpecialEffects:  3,
		Hit:             true,
		Location:        "Chest",
	}
	res.Contest = rules.Differential(res.AttackOutcome, res.ReactionOutcome)
	if mutate != nil {
		mutate(res)
	}
	return computeAvailable(res, stationary)
}

// Scenario mutations for the gating table.
var (
	gateBase = func(*AttackResult) {}

	gateAttackerCritical = func(r *AttackResult) {
		r.AttackRoll.Total = 5
		r.AttackOutcome = rules.Outcome{Roll: 5, Skill: 70, Grade: rules.GradeCritical}
		r.Contest = rules.Differential(r.AttackOutcome, r.ReactionOutcome)
	}
	gateDefenderWins = func(r *AttackResult) {
		r.Winner = WinnerDefender
		r.AttackRoll.Total = 85
		r.AttackOutcome = rules.Outcome{Roll: 85, Skill: 70, Grade: rules.GradeFailure}
		r.Contest = rules.Differential(r.AttackOutcome, r.ReactionOutcome)
	}
	gateDefenderCritical = func(r *AttackResult) {
		gateDefenderWins(r)
		r.ReactionRoll.Total = 3
		r.ReactionOutcome = rules.Outcome{Roll: 3, Skill: 70, Grade: rules.GradeCritical}
		r.Contest = rules.Differential(r.AttackOutcome, r.ReactionOutcome)
	}
	gateFirearmFumble = func(r *AttackResult) {
		r.Weapon = "Rifle"
		r.WeaponFacts = WeaponFacts{Name: "Rifle", Damage: "2d6", Force: "H", Ranged: true, Firearm: true}
		r.ReactionWeapon = WeaponFacts{}
		r.Reaction = ReactionNone
		r.Winner = WinnerNone
		r.SpecialEffects = 0
		r.AttackRoll.Total = 99
		r.AttackOutcome = rules.Outcome{Roll: 99, Skill: 65, Grade: rules.GradeFumble}
		r.ReactionAutoFail = true
		r.ReactionOutcome = rules.Outcome{Roll: 0, Skill: 0, Grade: rules.GradeFailure}
		r.Contest = rules.Differential(r.AttackOutcome, r.ReactionOutcome)
	}
	gateFirearm = func(r *AttackResult) {
		r.Weapon = "Rifle"
		r.WeaponFacts = WeaponFacts{Name: "Rifle", Damage: "2d6", Force: "H", Ranged: true, Firearm: true}
		r.ReactionWeapon = WeaponFacts{}
		r.Reaction = ReactionNone
		r.ReactionAutoFail = true
		r.ReactionOutcome = rules.Outcome{Roll: 0, Skill: 0, Grade: rules.GradeFailure}
		r.Contest = rules.Differential(r.AttackOutcome, r.ReactionOutcome)
	}
	gateHighTech = func(r *AttackResult) {
		gateFirearm(r)
		r.WeaponFacts.Notes = "high tech plasma"
		r.WeaponFacts.HighTech = true
	}
	gateReactionFumble = func(r *AttackResult) {
		r.ReactionRoll.Total = 99
		r.ReactionOutcome = rules.Outcome{Roll: 99, Skill: 70, Grade: rules.GradeFumble}
		r.Contest = rules.Differential(r.AttackOutcome, r.ReactionOutcome)
	}
)

// TestComputeAvailableGating exercises every printed row's Specific Roll
// and Special Weapon Type gates, plus the side columns and the firearm
// fumble override, in both directions.
func TestComputeAvailableGating(t *testing.T) {
	type tc struct {
		effect   string
		scenario func(*AttackResult)
		want     bool
	}
	cases := []tc{
		// side columns: an offensive row needs the attacker to win.
		{"Withdraw", gateBase, false},
		{"Withdraw", gateDefenderWins, true},
		{"Rapid Reload", gateBase, true},
		{"Rapid Reload", gateDefenderWins, false},
		{"Prepare Counter", gateDefenderWins, true},
		{"Prepare Counter", gateBase, false},
		{"Arise", gateDefenderWins, true},
		{"Arise", gateBase, false},
		{"Scar Foe", gateBase, true},
		{"Scar Foe", gateDefenderWins, true},
		{"Trip Opponent", gateBase, true},
		{"Trip Opponent", gateDefenderWins, true},

		// Specific Roll gates.
		{"Maximize Damage", gateBase, false},
		{"Maximize Damage", gateAttackerCritical, true},
		{"Bypass Armor", gateAttackerCritical, true},
		{"Circumvent Parry", gateAttackerCritical, true},
		{"Circumvent Parry", gateBase, false},
		{"Blind Opponent", gateDefenderCritical, true},
		{"Blind Opponent", gateDefenderWins, false},
		{"Slip Free", gateDefenderCritical, true},
		{"Slip Free", gateDefenderWins, false},
		{"Enhance Parry", gateDefenderCritical, true},
		{"Enhance Parry", gateAttackerCritical, false},
		{"Force Failure", gateReactionFumble, true},
		{"Force Failure", gateBase, false},
		{"Accidental Injury", gateFirearmFumble, true},
		{"Accidental Injury", gateBase, false},
		{"Select Target", gateFirearmFumble, true},
		{"Select Target", gateFirearm, false},
		{"Weapon Malfunction", gateFirearmFumble, true},
		{"Weapon Malfunction", gateFirearm, false},
		{"Weapon Malfunction", gateBase, false},

		// Special Weapon Type gates (attacker's weapon offensively).
		{"Bash", func(r *AttackResult) { r.WeaponFacts.Shield = true }, true},
		{"Bash", gateBase, false},
		{"Bash", func(r *AttackResult) { r.WeaponFacts.DamageType = "bludgeoning" }, true},
		{"Bleed", func(r *AttackResult) { r.WeaponFacts.DamageType = "cutting" }, true},
		{"Bleed", func(r *AttackResult) { r.WeaponFacts.DamageType = "bludgeoning" }, false},
		{"Bleed", gateFirearm, false}, // firearms: Critical only
		{"Bleed", func(r *AttackResult) { gateFirearm(r); gateAttackerCritical(r) }, true},
		{"Entangle", func(r *AttackResult) { r.WeaponFacts.Notes = "Can entangle" }, true},
		{"Entangle", gateBase, false},
		{"Grip", func(r *AttackResult) { r.WeaponFacts.Unarmed = true }, true},
		{"Grip", gateBase, false},
		{"Impale", gateBase, true}, // plain melee lodges
		{"Impale", func(r *AttackResult) { r.WeaponFacts.Notes = "Entangling" }, false},
		{"Impale", func(r *AttackResult) { r.WeaponFacts.DamageType = "bludgeoning" }, false},
		{"Impale", func(r *AttackResult) {
			r.Weapon = "Bow"
			r.WeaponFacts = WeaponFacts{Name: "Bow", Damage: "1d8", Force: "L", ImpaleSize: "S", Ranged: true, DMAdds: true}
			r.Reaction = ReactionNone
			r.ReactionAutoFail = true
			r.ReactionWeapon = WeaponFacts{}
			r.ReactionOutcome = rules.Outcome{Roll: 0, Skill: 0, Grade: rules.GradeFailure}
			r.Contest = rules.Differential(r.AttackOutcome, r.ReactionOutcome)
			if r.Contest.Winner == 1 {
				r.Winner = WinnerAttacker
			}
		}, true},
		{"Impale", func(r *AttackResult) { // thrown bolas print no Impale Size
			r.Weapon = "Bolas"
			r.WeaponFacts = WeaponFacts{Name: "Bolas", Damage: "1d4", Force: "-", Ranged: true}
			r.Reaction = ReactionNone
			r.ReactionAutoFail = true
			r.ReactionWeapon = WeaponFacts{}
			r.ReactionOutcome = rules.Outcome{Roll: 0, Skill: 0, Grade: rules.GradeFailure}
			r.Contest = rules.Differential(r.AttackOutcome, r.ReactionOutcome)
			if r.Contest.Winner == 1 {
				r.Winner = WinnerAttacker
			}
		}, false},
		{"Marksman", func(r *AttackResult) { r.WeaponFacts.Ranged = true }, true},
		{"Marksman", gateBase, false},
		{"Stun Location", func(r *AttackResult) { r.WeaponFacts.DamageType = "bludgeoning" }, true},
		{"Stun Location", gateBase, false},
		{"Sunder", func(r *AttackResult) { r.WeaponFacts.Notes = "Two handed" }, true},
		{"Sunder", gateBase, false},
		{"Drop Foe", gateFirearm, true},
		{"Drop Foe", gateBase, false},
		{"Duck Back", gateFirearm, true},
		{"Duck Back", gateBase, false},
		{"Pin Down", gateFirearm, true},
		{"Pin Down", gateBase, false},
		{"Overpenetration", func(r *AttackResult) { gateFirearm(r); gateAttackerCritical(r) }, true},
		{"Overpenetration", gateFirearm, false},
		{"Overpenetration", gateAttackerCritical, false}, // melee critical: not a firearm
		{"Circumvent Cover", func(r *AttackResult) { gateHighTech(r); gateAttackerCritical(r) }, true},
		{"Circumvent Cover", func(r *AttackResult) { gateFirearm(r); gateAttackerCritical(r) }, false},

		// Damage Weapon: needs a parrying weapon and, for firearms, a
		// Critical.
		{"Damage Weapon", gateBase, true},
		{"Damage Weapon", func(r *AttackResult) {
			r.ReactionWeapon = WeaponFacts{}
			r.Reaction = ReactionNone
			r.ReactionAutoFail = true
		}, false},
		{"Damage Weapon", gateFirearm, false},
		{"Damage Weapon", func(r *AttackResult) { gateFirearm(r); gateAttackerCritical(r) }, true},

		// Choose Location: ranged needs a Critical, or Close Range with a
		// stationary or unaware target.
		{"Choose Location", gateBase, true},
		{"Choose Location", func(r *AttackResult) {
			r.Weapon = "Bow"
			r.WeaponFacts = WeaponFacts{Name: "Bow", Damage: "1d8", Force: "L", Ranged: true, DMAdds: true}
			r.Ranged = true
			r.RangeBand = "effective"
		}, false},
	}
	for _, c := range cases {
		got := gateFixture(c.scenario)
		if containsString(got, c.effect) != c.want {
			t.Errorf("%s: available=%v want=%v (list %v)", c.effect, containsString(got, c.effect), c.want, got)
		}
	}

	// Choose Location at Close Range opens only for a stationary or
	// unaware target (computeAvailable carries the flag).
	bowEffective := func(r *AttackResult) {
		r.Weapon = "Bow"
		r.WeaponFacts = WeaponFacts{Name: "Bow", Damage: "1d8", Force: "L", Ranged: true, DMAdds: true}
		r.Ranged = true
		r.RangeBand = "close"
	}
	if got := gateFixtureAt(bowEffective, true); !containsString(got, "Choose Location") {
		t.Errorf("Close Range + stationary must open Choose Location: %v", got)
	}
	if got := gateFixtureAt(bowEffective, false); containsString(got, "Choose Location") {
		t.Errorf("Close Range alone must not open Choose Location: %v", got)
	}

	// The firearm fumble grants exactly one pick even when the
	// Differential granted nothing.
	fumble := gateFixture(gateFirearmFumble)
	if !containsString(fumble, "Accidental Injury") || !containsString(fumble, "Select Target") {
		t.Errorf("firearm fumble picks missing: %v", fumble)
	}
}

// TestFirearmFumbleBumpsPickCount: computeAvailable raises the spent
// Special Effects count to 1 for the intended target (Fumbling with
// Firearms) and records the reason.
func TestFirearmFumbleBumpsPickCount(t *testing.T) {
	res := &AttackResult{
		Winner:          WinnerNone,
		SpecialEffects:  0,
		WeaponFacts:     WeaponFacts{Name: "Pistol", Damage: "1d6", Force: "L", Ranged: true, Firearm: true},
		AttackOutcome:   rules.Outcome{Roll: 100, Skill: 40, Grade: rules.GradeFumble},
		ReactionOutcome: rules.Outcome{Grade: rules.GradeFailure},
	}
	computeAvailable(res, false)
	if res.SpecialEffects != 1 {
		t.Errorf("SpecialEffects = %d, want 1", res.SpecialEffects)
	}
	if len(res.Facts) == 0 || !strings.Contains(strings.Join(res.Facts, " "), "Fumbling with Firearms") {
		t.Errorf("missing the Fumbling with Firearms fact: %v", res.Facts)
	}
}

// ---------------------------------------------------------------------------
// applying effects
// ---------------------------------------------------------------------------

// effectFixture builds a running three-way encounter with a hand-built
// attack audit: the attacker's Broadsword struck the defender's Chest for
// 7 damage (no armour), the exchange won one differential margin... every
// printed effect is offered, and the wound stands exactly as recorded so
// later picks can adjust it.
func effectFixture(t *testing.T, seed int64, mutate func(*AttackResult, *char.Character, *char.Character)) (*dice.Rand, *Encounter, *char.Character, *char.Character, *char.Character, *AttackResult) {
	t.Helper()
	att := fighter(t, "fx-att", "Attacker")
	def := fighter(t, "fx-def", "Defender")
	bys := fighter(t, "fx-bys", "Bystander")
	learnStyle(t, att, "Art", 70, "Broadsword")
	learnStyle(t, def, "Art", 70, "Broadsword")
	giveWeapon(t, att, "Broadsword")
	giveWeapon(t, def, "Broadsword")
	e, err := Start([]*char.Character{att, def, bys})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	res := &AttackResult{
		Round:    e.Round,
		Attacker: idOf(att), Defender: idOf(def),
		Weapon:             "Broadsword",
		WeaponFacts:        WeaponFacts{Name: "Broadsword", Damage: "1d8", Size: "M", DMAdds: true, AP: 6, HP: 12},
		ReactionWeapon:     WeaponFacts{Name: "Broadsword", Damage: "1d8", Size: "M", DMAdds: true, AP: 6, HP: 12},
		AttackSkill:        70,
		AttackRoll:         dice.Roll{Expr: "1d100", Total: 5},
		AttackOutcome:      rules.Outcome{Roll: 5, Skill: 70, Grade: rules.GradeCritical},
		Reaction:           ReactionParry,
		ReactionSkill:      70,
		ReactionRoll:       dice.Roll{Expr: "1d100", Total: 60},
		ReactionOutcome:    rules.Outcome{Roll: 60, Skill: 70, Grade: rules.GradeSuccess},
		Winner:             WinnerAttacker,
		SpecialEffects:     3,
		Hit:                true,
		Damaged:            true,
		ParriedWith:        "Broadsword",
		Location:           "Chest",
		LocationRoll:       dice.Roll{Expr: "1d20", Total: 10},
		DamageRoll:         dice.Roll{Expr: "1d8", Dice: []dice.Die{{Sides: 8, Value: 5}}, Total: 5},
		DamageModifierRoll: dice.Roll{Expr: "1d4", Dice: []dice.Die{{Sides: 4, Value: 2}}, Total: 2},
		DamagePre:          7,
		DamageApplied:      7,
	}
	res.Contest = rules.Differential(res.AttackOutcome, res.ReactionOutcome)
	res.AvailableSpecialEffects = func() []string {
		var all []string
		for _, fx := range specialEffects {
			all = append(all, fx.Name)
		}
		return all
	}()
	if mutate != nil {
		mutate(res, att, def)
	}
	if res.DamageApplied > 0 {
		res.preCond = &conditionSnapshot{
			Conditions: append([]char.Condition(nil), def.Conditions...),
			Dead:       def.Dead,
			WoundsLen:  len(def.Wounds),
		}
		if _, _, err := def.ApplyWound(res.Location, res.DamageApplied); err != nil {
			t.Fatalf("ApplyWound: %v", err)
		}
		sev, err := def.SeverityAt(res.Location)
		if err != nil {
			t.Fatalf("SeverityAt: %v", err)
		}
		res.Severity = sev
	}
	return dice.NewRand(seed), e, att, def, bys, res
}

// condNotes returns the notes of the named condition, or "".
func condNotes(c *char.Character, name string) (string, bool) {
	cond := findCondition(c, name)
	if cond == nil {
		return "", false
	}
	return cond.Notes, true
}

// TestApplySpecialEffectMechanics applies every printed Special Effect to
// a crafted exchange and checks its deterministic outcome: conditions,
// wounds, ledgers, adjudication flags and audit rolls.
func TestApplySpecialEffectMechanics(t *testing.T) {
	t.Run("Accidental Injury", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 101, func(res *AttackResult, a, d *char.Character) {
			res.Winner = WinnerDefender
			res.AttackOutcome = rules.Outcome{Roll: 99, Skill: 70, Grade: rules.GradeFumble}
			res.Weapon = "Fist/Kick"
			res.WeaponFacts = WeaponFacts{Name: "Fist/Kick", Damage: "1d3", Size: "S", Unarmed: true, DMAdds: true}
			res.Contest = rules.Differential(res.AttackOutcome, res.ReactionOutcome)
		})
		out, err := ApplySpecialEffect(r, e, "Accidental Injury", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if len(out.Rolls) < 2 {
			t.Fatalf("rolls = %+v, want damage, modifier and location", out.Rolls)
		}
		if !strings.Contains(strings.Join(out.Facts, " "), "themself") {
			t.Errorf("facts = %v", out.Facts)
		}
		if out.DamageApplied > 0 {
			sev, err := att.SeverityAt(out.Location)
			if err != nil || sev == char.SeverityNone {
				t.Errorf("self-wound at %q: severity %v (%v)", out.Location, sev, err)
			}
		}
	})
	t.Run("Arise", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 102, func(res *AttackResult, a, d *char.Character) {
			res.Winner = WinnerDefender
			d.AddCondition(char.Condition{Name: char.CondProne})
		})
		out, err := ApplySpecialEffect(r, e, "Arise", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if def.HasCondition(char.CondProne) {
			t.Error("Arise left the defender prone")
		}
		if out.Side != SideDefensive || out.Actor != idOf(def) {
			t.Errorf("side/actor = %q/%q", out.Side, out.Actor)
		}
	})
	t.Run("Bash", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 103, func(res *AttackResult, a, d *char.Character) {
			res.WeaponFacts.Shield = true
			res.WeaponFacts.Name = "Buckler"
		})
		out, err := ApplySpecialEffect(r, e, "Bash", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		// 7 damage ÷ 2 per metre for shields, rounding up = 4 metres.
		if out.Distance != 4 {
			t.Errorf("Distance = %d, want 4", out.Distance)
		}
	})
	t.Run("Bleed", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 104, func(res *AttackResult, a, d *char.Character) {
			res.ForcedFailure = true // Force Failure was won first
		})
		out, err := ApplySpecialEffect(r, e, "Bleed", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !def.HasCondition(char.CondBleeding) {
			t.Fatalf("bleed did not start: %v", out.Facts)
		}
		if len(out.Rolls) == 0 || out.Rolls[0].Name != "endurance" {
			t.Errorf("rolls = %+v, want the opposed Endurance", out.Rolls)
		}
	})
	t.Run("Blind Opponent", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 105, func(res *AttackResult, a, d *char.Character) {
			res.Winner = WinnerDefender
			res.ReactionOutcome = rules.Outcome{Roll: 3, Skill: 70, Grade: rules.GradeCritical}
			res.ForcedFailure = true
			res.Contest = rules.Differential(res.AttackOutcome, res.ReactionOutcome)
		})
		out, err := ApplySpecialEffect(r, e, "Blind Opponent", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		notes, ok := condNotes(att, CondBlinded)
		if !ok {
			t.Fatalf("attacker not blinded: %v", out.Facts)
		}
		if !strings.Contains(notes, "grade=hard") {
			t.Errorf("blinded notes = %q, want the default Hard grade", notes)
		}
		var found bool
		for _, nr := range out.Rolls {
			if nr.Name == "evade" {
				found = true
			}
		}
		if !found {
			t.Errorf("rolls = %+v, want the blinded party's Evade", out.Rolls)
		}
	})
	t.Run("Bypass Armor", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 106, func(res *AttackResult, a, d *char.Character) {
			wearMail(t, d) // AP 6
			res.ArmorPoints, res.ArmorAbsorbed = 6, 6
			res.DamageApplied = 1 // 7 − 6 armour
		})
		before := res.DamageApplied
		out, err := ApplySpecialEffect(r, e, "Bypass Armor", att, def, res, EffectOptions{ArmorComponent: "Mail"})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !containsString(res.BypassedArmor, "Mail") {
			t.Fatalf("BypassedArmor = %v", res.BypassedArmor)
		}
		if res.DamageApplied <= before {
			t.Errorf("damage did not grow: %d → %d", before, res.DamageApplied)
		}
		if out.Delta != res.DamageApplied-before {
			t.Errorf("Delta = %d, want %d", out.Delta, res.DamageApplied-before)
		}
	})
	t.Run("Choose Location", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 107, nil)
		out, err := ApplySpecialEffect(r, e, "Choose Location", att, def, res, EffectOptions{Location: "Abdomen"})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if res.Location != "Abdomen" || out.Location != "Abdomen" {
			t.Fatalf("location = %q/%q, want Abdomen", res.Location, out.Location)
		}
		_, e2, att2, def2, _, res2 := effectFixture(t, 107, nil)
		if _, err := ApplySpecialEffect(r, e2, "Choose Location", att2, def2, res2, EffectOptions{Location: "Tail"}); !errors.Is(err, ErrInvalidChoice) {
			t.Errorf("unknown location = %v, want ErrInvalidChoice", err)
		}
	})
	t.Run("Circumvent Cover", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 108, nil)
		out, err := ApplySpecialEffect(r, e, "Circumvent Cover", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !out.Adjudicated {
			t.Error("Circumvent Cover must be adjudicated (no cover model)")
		}
	})
	t.Run("Circumvent Parry", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 109, func(res *AttackResult, a, d *char.Character) {
			res.ParryApplied = true
			res.ParryReduction = 3
			res.DamageApplied = 4 // 7 − 3 deflected
		})
		out, err := ApplySpecialEffect(r, e, "Circumvent Parry", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if res.ParryApplied {
			t.Error("parry still recorded as applied")
		}
		if res.DamageApplied != 7 || out.Delta != 3 {
			t.Errorf("damage %d (delta %d), want 7 (+3)", res.DamageApplied, out.Delta)
		}
	})
	t.Run("Damage Weapon", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 110, func(res *AttackResult, a, d *char.Character) {
			res.ReactionWeapon.HP = 1 // one good hit breaks it
		})
		out, err := ApplySpecialEffect(r, e, "Damage Weapon", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		// 7 damage (5 + 2 DM) vs the weapon's 6 AP → 1 through → 0 HP.
		if !def.HasCondition(CondBrokenWeapon) {
			t.Fatalf("weapon not broken: %v %v", out.Facts, e.WeaponHP)
		}
		if len(e.WeaponHP) != 1 {
			t.Errorf("WeaponHP ledger = %v", e.WeaponHP)
		}
	})
	t.Run("Disarm Opponent", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 111, func(res *AttackResult, a, d *char.Character) {
			res.ForcedFailure = true
		})
		out, err := ApplySpecialEffect(r, e, "Disarm Opponent", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		for _, it := range def.Inventory {
			if it != nil && it.Name == "Broadsword" && it.Equipped {
				t.Fatalf("weapon still wielded: %v %v", out.Facts, out.Rolls)
			}
		}
		if len(out.Rolls) == 0 || out.Rolls[0].Name != "disarm resistance" {
			t.Errorf("rolls = %+v", out.Rolls)
		}
	})
	t.Run("Drop Foe", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 112, func(res *AttackResult, a, d *char.Character) {
			res.ForcedFailure = true
		})
		out, err := ApplySpecialEffect(r, e, "Drop Foe", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !def.HasCondition(char.CondIncapacitated) {
			t.Fatalf("target not incapacitated: %v", out.Facts)
		}
	})
	t.Run("Duck Back", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 113, nil)
		out, err := ApplySpecialEffect(r, e, "Duck Back", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !out.Adjudicated {
			t.Error("Duck Back must be adjudicated (no cover model)")
		}
	})
	t.Run("Enhance Parry", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 114, func(res *AttackResult, a, d *char.Character) {
			res.Winner = WinnerDefender // a defensive pick: the parry won
		})
		d, err := char.Derive(def)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		out, err := ApplySpecialEffect(r, e, "Enhance Parry", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if res.DamageApplied != 0 || res.Severity != char.SeverityNone {
			t.Errorf("wound not reversed: applied=%d severity=%s", res.DamageApplied, res.Severity)
		}
		if res.ParryReduction != res.DamagePre {
			t.Errorf("ParryReduction = %d, want all %d", res.ParryReduction, res.DamagePre)
		}
		if def.CurrentHP["Chest"] != d.LocationHP["Chest"] {
			t.Errorf("Chest HP = %d, want restored to %d", def.CurrentHP["Chest"], d.LocationHP["Chest"])
		}
		if !strings.Contains(strings.Join(out.Facts, " "), "reversed") {
			t.Errorf("facts = %v", out.Facts)
		}
	})
	t.Run("Entangle", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 115, nil)
		out, err := ApplySpecialEffect(r, e, "Entangle", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		cond := findCondition(def, CondEntangled)
		if cond == nil || cond.Location != "Chest" {
			t.Fatalf("entangled condition = %+v", cond)
		}
		if out.Conditions == nil || out.Conditions[0] != CondEntangled {
			t.Errorf("conditions = %v", out.Conditions)
		}
	})
	t.Run("Force Failure", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 116, nil)
		if _, err := ApplySpecialEffect(r, e, "Force Failure", att, def, res, EffectOptions{}); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !res.ForcedFailure {
			t.Fatal("Force Failure not recorded on the exchange")
		}
	})
	t.Run("Grip", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 117, func(res *AttackResult, a, d *char.Character) {
			res.WeaponFacts.Unarmed = true
		})
		if _, err := ApplySpecialEffect(r, e, "Grip", att, def, res, EffectOptions{}); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !def.HasCondition(CondGripped) {
			t.Fatal("victim not gripped")
		}
	})
	t.Run("Impale", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 118, nil)
		before := res.DamageApplied
		out, err := ApplySpecialEffect(r, e, "Impale", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		var extra bool
		for _, nr := range out.Rolls {
			if nr.Name == "impale damage" {
				extra = true
			}
		}
		if !extra {
			t.Fatalf("rolls = %+v, want the second damage roll", out.Rolls)
		}
		if res.DamageApplied < before {
			t.Errorf("damage shrank: %d → %d", before, res.DamageApplied)
		}
		victimNotes, ok := condNotes(def, CondImpaled)
		if !ok || !strings.HasPrefix(victimNotes, "weapon=Broadsword; grade=formidable") {
			t.Errorf("victim Impaled = %q (SIZ 12 vs Medium weapon must read formidable)", victimNotes)
		}
		attNotes, ok := condNotes(att, CondImpaled)
		if !ok || !strings.HasPrefix(attNotes, "weapon=Broadsword;") {
			t.Errorf("attacker Impaled = %q (their weapon cannot parry)", attNotes)
		}
	})
	t.Run("Marksman", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 119, func(res *AttackResult, a, d *char.Character) {
			res.Location = "Right Leg"
			res.WeaponFacts.Ranged = true
		})
		out, err := ApplySpecialEffect(r, e, "Marksman", att, def, res, EffectOptions{Location: "Abdomen"})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if res.Location != "Abdomen" || out.Location != "Abdomen" {
			t.Fatalf("location = %q, want Abdomen", res.Location)
		}
		_, e2, att2, def2, _, res2 := effectFixture(t, 119, func(res *AttackResult, a, d *char.Character) {
			res.Location = "Right Leg"
			res.WeaponFacts.Ranged = true
		})
		if _, err := ApplySpecialEffect(r, e2, "Marksman", att2, def2, res2, EffectOptions{Location: "Head"}); !errors.Is(err, ErrInvalidChoice) {
			t.Errorf("non-adjoining shift = %v, want ErrInvalidChoice", err)
		}
	})
	t.Run("Maximize Damage", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 120, nil)
		out, err := ApplySpecialEffect(r, e, "Maximize Damage", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		// 1d8 rolled 5 → becomes 8: +3 damage, none of it absorbed.
		if res.Maximised != 1 || res.DamageApplied != 10 || out.Delta != 3 {
			t.Errorf("maximised=%d applied=%d delta=%d, want 1/10/3", res.Maximised, res.DamageApplied, out.Delta)
		}
		if len(res.Adjustments) == 0 {
			t.Error("adjustment not recorded")
		}
	})
	t.Run("Overpenetration", func(t *testing.T) {
		r, e, att, def, bys, res := effectFixture(t, 121, nil)
		out, err := ApplySpecialEffect(r, e, "Overpenetration", att, def, res, EffectOptions{Target: bys})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		// half of 7 damage (rounding up) with no armour on the bystander.
		if out.DamageApplied != 4 || out.Location == "" {
			t.Errorf("second victim took %d at %q, want 4 at a rolled location", out.DamageApplied, out.Location)
		}
		sev, err := bys.SeverityAt(out.Location)
		if err != nil || sev == char.SeverityNone {
			t.Errorf("bystander not wounded: %v (%v)", sev, err)
		}
		_, e2, att2, def2, _, res2 := effectFixture(t, 121, nil)
		if _, err := ApplySpecialEffect(r, e2, "Overpenetration", att2, def2, res2, EffectOptions{}); !errors.Is(err, ErrInvalidChoice) {
			t.Errorf("missing target = %v, want ErrInvalidChoice", err)
		}
	})
	t.Run("Pin Down", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 122, func(res *AttackResult, a, d *char.Character) {
			res.ForcedFailure = true
		})
		out, err := ApplySpecialEffect(r, e, "Pin Down", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		cond := findCondition(def, CondPinned)
		if cond == nil || cond.Rounds != 2 {
			t.Fatalf("pinned = %+v, want 2 rounds", cond)
		}
		_ = out
	})
	t.Run("Prepare Counter", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 123, func(res *AttackResult, a, d *char.Character) {
			res.Winner = WinnerDefender
			res.Contest = rules.Differential(res.AttackOutcome, res.ReactionOutcome)
		})
		out, err := ApplySpecialEffect(r, e, "Prepare Counter", att, def, res,
			EffectOptions{CounterAgainst: "Trip Opponent", CounterSubstitute: "Withdraw"})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if len(e.Counters) != 1 || e.Counters[0].Against != "Trip Opponent" || e.Counters[0].Substitute != "Withdraw" {
			t.Fatalf("counters = %+v (%v)", e.Counters, out.Facts)
		}
	})
	t.Run("Rapid Reload", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 124, nil)
		if _, err := ApplySpecialEffect(r, e, "Rapid Reload", att, def, res, EffectOptions{}); err != nil {
			t.Fatalf("first: %v", err)
		}
		if _, err := ApplySpecialEffect(r, e, "Rapid Reload", att, def, res, EffectOptions{}); err != nil {
			t.Fatalf("second (stackable): %v", err)
		}
		cond := findCondition(att, CondRapidReload)
		if cond == nil || cond.Rounds != 2 {
			t.Fatalf("stacks = %+v, want 2 (Rounds carries the stack)", cond)
		}
	})
	t.Run("Scar Foe", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 125, nil)
		out, err := ApplySpecialEffect(r, e, "Scar Foe", att, def, res, EffectOptions{Scar: "a duelling brand across the cheek"})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		notes, ok := condNotes(def, CondScarred)
		if !ok || notes != "a duelling brand across the cheek" {
			t.Errorf("scar = %q", notes)
		}
		if out.Conditions[0] != CondScarred {
			t.Errorf("conditions = %v", out.Conditions)
		}
	})
	t.Run("Select Target", func(t *testing.T) {
		r, e, att, def, bys, res := effectFixture(t, 126, func(res *AttackResult, a, d *char.Character) {
			res.Winner = WinnerDefender
			res.AttackOutcome = rules.Outcome{Roll: 99, Skill: 70, Grade: rules.GradeFumble}
			res.Contest = rules.Differential(res.AttackOutcome, res.ReactionOutcome)
		})
		d, err := char.Derive(def)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		out, err := ApplySpecialEffect(r, e, "Select Target", att, def, res, EffectOptions{Target: bys})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if def.CurrentHP["Chest"] != d.LocationHP["Chest"] {
			t.Errorf("the defender still carries the deflected blow: %d/%d", def.CurrentHP["Chest"], d.LocationHP["Chest"])
		}
		if out.DamageApplied != 7 {
			t.Errorf("bystander took %d, want the full 7", out.DamageApplied)
		}
		sev, err := bys.SeverityAt(out.Location)
		if err != nil || sev == char.SeverityNone {
			t.Errorf("bystander not wounded: %v (%v)", sev, err)
		}
	})
	t.Run("Slip Free", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 127, func(res *AttackResult, a, d *char.Character) {
			res.Winner = WinnerDefender
			d.AddCondition(char.Condition{Name: CondEntangled, Location: "Chest"})
			res.Contest = rules.Differential(res.AttackOutcome, res.ReactionOutcome)
		})
		if _, err := ApplySpecialEffect(r, e, "Slip Free", att, def, res, EffectOptions{}); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if def.HasCondition(CondEntangled) {
			t.Error("still entangled after Slip Free")
		}
	})
	t.Run("Stun Location", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 128, func(res *AttackResult, a, d *char.Character) {
			res.WeaponFacts.DamageType = "bludgeoning"
			res.ForcedFailure = true
		})
		out, err := ApplySpecialEffect(r, e, "Stun Location", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		cond := findCondition(def, char.CondStunned)
		if cond == nil || cond.Location != "Chest" || cond.Rounds != res.DamageApplied {
			t.Fatalf("stunned = %+v, want the Chest for %d rounds", cond, res.DamageApplied)
		}
		if !strings.Contains(strings.Join(out.Facts, " "), "staggers") {
			t.Errorf("torso stun fact missing: %v", out.Facts)
		}
	})
	t.Run("Sunder", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 129, func(res *AttackResult, a, d *char.Character) {
			wearMail(t, d) // AP 6
			res.ArmorPoints, res.ArmorAbsorbed = 6, 6
			res.DamageApplied = 1
		})
		out, err := ApplySpecialEffect(r, e, "Sunder", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		ap, err := def.ArmorAt("Chest")
		if err != nil {
			t.Fatalf("ArmorAt: %v", err)
		}
		if ap != 0 {
			t.Errorf("mail at the Chest still %d AP (7 damage should strip all 6): %v", ap, out.Facts)
		}
	})
	t.Run("Trip Opponent", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 130, func(res *AttackResult, a, d *char.Character) {
			res.ForcedFailure = true
		})
		out, err := ApplySpecialEffect(r, e, "Trip Opponent", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !def.HasCondition(char.CondProne) {
			t.Fatalf("victim still standing: %v", out.Facts)
		}
	})
	t.Run("Weapon Malfunction", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 131, func(res *AttackResult, a, d *char.Character) {
			res.Winner = WinnerDefender
			res.Weapon = "Rifle"
			res.WeaponFacts = WeaponFacts{Name: "Rifle", Damage: "2d6", Force: "H", Ranged: true, Firearm: true}
			res.AttackOutcome = rules.Outcome{Roll: 100, Skill: 65, Grade: rules.GradeFumble}
			res.Contest = rules.Differential(res.AttackOutcome, res.ReactionOutcome)
		})
		if _, err := ApplySpecialEffect(r, e, "Weapon Malfunction", att, def, res, EffectOptions{}); err != nil {
			t.Fatalf("apply: %v", err)
		}
		notes, ok := condNotes(att, CondMalfunction)
		if !ok || notes != "weapon=Rifle" {
			t.Fatalf("malfunction = %q", notes)
		}
	})
	t.Run("Withdraw", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 132, func(res *AttackResult, a, d *char.Character) {
			res.Winner = WinnerDefender
			res.Contest = rules.Differential(res.AttackOutcome, res.ReactionOutcome)
		})
		out, err := ApplySpecialEffect(r, e, "Withdraw", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !out.Adjudicated {
			t.Error("Withdraw must be adjudicated (no engagement model)")
		}
	})
}

// TestPrepareCounterInterception: a secret nomination intercepts the
// effect — neutralised, or substituted with the counter-holder's own row,
// which succeeds automatically.
func TestPrepareCounterInterception(t *testing.T) {
	t.Run("neutralise", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 201, func(res *AttackResult, a, d *char.Character) {
			res.ForcedFailure = true
		})
		idx, err := e.indexOf(def)
		if err != nil {
			t.Fatal(err)
		}
		e.Counters = append(e.Counters, CounterEntry{Index: idx, Against: "Trip Opponent"})
		out, err := ApplySpecialEffect(r, e, "Trip Opponent", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !out.Counter {
			t.Fatal("counter did not fire")
		}
		if def.HasCondition(char.CondProne) {
			t.Error("trip landed despite the counter")
		}
		if len(e.Counters) != 0 {
			t.Errorf("counter not consumed: %+v", e.Counters)
		}
	})
	t.Run("substitute", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 202, func(res *AttackResult, a, d *char.Character) {
			res.ForcedFailure = true
			d.AddCondition(char.Condition{Name: char.CondProne})
		})
		idx, err := e.indexOf(def)
		if err != nil {
			t.Fatal(err)
		}
		e.Counters = append(e.Counters, CounterEntry{Index: idx, Against: "Trip Opponent", Substitute: "Arise"})
		out, err := ApplySpecialEffect(r, e, "Trip Opponent", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !out.Counter {
			t.Fatal("counter did not fire")
		}
		// The substituted Arise succeeds automatically: the defender who
		// nominated it (the recipient of the trip) is left standing.
		if def.HasCondition(char.CondProne) {
			t.Errorf("substituted Arise did not stand them up: %v", out.Facts)
		}
	})
}

// TestApplySpecialEffectGating covers the refusal sentinels: unknown
// names, wrong side, effects not on offer, spent picks and illegal
// repeats, plus the choice errors.
func TestApplySpecialEffectGating(t *testing.T) {
	t.Run("unknown effect", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 301, nil)
		if _, err := ApplySpecialEffect(r, e, "Riposte", att, def, res, EffectOptions{}); !errors.Is(err, ErrUnknownEffect) {
			t.Errorf("err = %v, want ErrUnknownEffect", err)
		}
	})
	t.Run("wrong combatants", func(t *testing.T) {
		r, e, att, _, bys, res := effectFixture(t, 302, nil)
		if _, err := ApplySpecialEffect(r, e, "Trip Opponent", att, bys, res, EffectOptions{}); err == nil {
			t.Error("applying an effect with swapped combatants must refuse")
		}
	})
	t.Run("losing side", func(t *testing.T) {
		// The attacker won the differential; a defensive row has no side
		// to apply from, whoever holds it on offer.
		r, e, att, def, _, res := effectFixture(t, 303, func(res *AttackResult, a, d *char.Character) {
			res.AvailableSpecialEffects = []string{"Withdraw"}
		})
		if _, err := ApplySpecialEffect(r, e, "Withdraw", att, def, res, EffectOptions{}); !errors.Is(err, ErrEffectUnavailable) {
			t.Errorf("err = %v, want ErrEffectUnavailable", err)
		}
	})
	t.Run("not on offer", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 304, func(res *AttackResult, a, d *char.Character) {
			res.AvailableSpecialEffects = []string{"Trip Opponent"}
		})
		if _, err := ApplySpecialEffect(r, e, "Bash", att, def, res, EffectOptions{}); !errors.Is(err, ErrEffectUnavailable) {
			t.Errorf("err = %v, want ErrEffectUnavailable", err)
		}
	})
	t.Run("pick budget", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 305, func(res *AttackResult, a, d *char.Character) {
			res.SpecialEffects = 1
		})
		if _, err := ApplySpecialEffect(r, e, "Trip Opponent", att, def, res, EffectOptions{}); err != nil {
			t.Fatalf("first pick: %v", err)
		}
		_, err := ApplySpecialEffect(r, e, "Rapid Reload", att, def, res, EffectOptions{})
		if !errors.Is(err, ErrEffectUnavailable) {
			t.Errorf("second pick = %v, want ErrEffectUnavailable (only one won)", err)
		}
		if !strings.Contains(err.Error(), "Special Effect") {
			t.Errorf("message %q should name the budget", err)
		}
	})
	t.Run("non-stackable repeat", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 306, nil)
		if _, err := ApplySpecialEffect(r, e, "Trip Opponent", att, def, res, EffectOptions{}); err != nil {
			t.Fatalf("first: %v", err)
		}
		if _, err := ApplySpecialEffect(r, e, "Trip Opponent", att, def, res, EffectOptions{}); !errors.Is(err, ErrEffectUnavailable) {
			t.Errorf("repeat = %v, want ErrEffectUnavailable", err)
		}
	})
	t.Run("choice errors", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 307, nil)
		if _, err := ApplySpecialEffect(r, e, "Choose Location", att, def, res, EffectOptions{}); !errors.Is(err, ErrInvalidChoice) {
			t.Errorf("missing location = %v", err)
		}
		_, e2, att2, def2, _, res2 := effectFixture(t, 307, func(res *AttackResult, a, d *char.Character) {
			res.Winner = WinnerDefender // Prepare Counter is a defensive row
		})
		if _, err := ApplySpecialEffect(r, e2, "Prepare Counter", att2, def2, res2, EffectOptions{}); !errors.Is(err, ErrInvalidChoice) {
			t.Errorf("missing nomination = %v", err)
		}
		if _, err := ApplySpecialEffect(r, e2, "Prepare Counter", att2, def2, res2,
			EffectOptions{CounterAgainst: "No Such Effect"}); !errors.Is(err, ErrUnknownEffect) {
			t.Errorf("nominating an unknown effect = %v", err)
		}
	})
	t.Run("ended encounter", func(t *testing.T) {
		r, e, att, def, _, res := effectFixture(t, 308, nil)
		e.End()
		if _, err := ApplySpecialEffect(r, e, "Trip Opponent", att, def, res, EffectOptions{}); !errors.Is(err, ErrNotRunning) {
			t.Errorf("err = %v, want ErrNotRunning", err)
		}
	})
}

// TestEffectsDeterminism: the same seed produces byte-identical effect
// audits for the rolls they make.
func TestEffectsDeterminism(t *testing.T) {
	run := func() []byte {
		r, e, att, def, _, res := effectFixture(t, 401, nil)
		out, err := ApplySpecialEffect(r, e, "Impale", att, def, res, EffectOptions{})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		raw, err := json.Marshal(out)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return raw
	}
	first, second := run(), run()
	if string(first) != string(second) {
		t.Fatalf("effect audit is not deterministic for a fixed seed:\n%s\n%s", first, second)
	}
}

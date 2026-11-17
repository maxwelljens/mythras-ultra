package resolve

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/dice"
	"github.com/maxwelljens/mythras-ultra/internal/engine/rules"
)

// TestRollDice covers the dice tool: audit trail, citation, reason, and the
// parse-failure refusal that consumes no randomness.
func TestRollDice(t *testing.T) {
	const seed = 42
	s := newState(t, seed)
	want, err := dice.NewRand(seed).Roll("2d6+3")
	if err != nil {
		t.Fatalf("reference roll: %v", err)
	}
	res, err := Apply(s, RollDice{Expr: " 2d6+3 ", Reason: "falling damage"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Audit == nil || len(res.Audit.Rolls) != 1 {
		t.Fatalf("audit = %+v, want one roll", res.Audit)
	}
	if got := res.Audit.Rolls[0]; got.Total != want.Total || got.Expr != want.Expr {
		t.Errorf("roll = %s, want %s", got, want)
	}
	if len(res.Audit.Citations) == 0 {
		t.Error("roll_dice must cite the rulebook's dice section")
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "falling damage") || !strings.Contains(joined, "roll:") {
		t.Errorf("facts = %v, want reason and roll lines", res.Facts)
	}
	if res.Summary == "" {
		t.Error("empty summary")
	}

	if _, err := Apply(s, RollDice{Expr: "10000d6"}); !errors.Is(err, dice.ErrTooManyDice) {
		t.Fatalf("bad expr error = %v, want dice.ErrTooManyDice", err)
	}
	if s.EventSeq != 1 {
		t.Errorf("EventSeq = %d after one accepted + one refused roll, want 1", s.EventSeq)
	}
}

// TestSkillTest covers the skill tool: first-draw determinism, difficulty
// folding (hardest wins), audit citation, and every refusal sentinel.
func TestSkillTest(t *testing.T) {
	const seed = 42
	s := newState(t, seed)
	addCharacter(t, s, "pc-1", true)
	want := dice.NewRand(seed).D100()

	res, err := Apply(s, SkillTest{
		Actor: "pc-1", Skill: "Brawn", Grade: "hard",
		Modifiers: []string{"very easy"}, // hardest must win: hard
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Audit == nil || len(res.Audit.Rolls) != 1 {
		t.Fatalf("audit = %+v, want one 1d100", res.Audit)
	}
	if got := res.Audit.Rolls[0]; got.Expr != "1d100" || got.Total != want {
		t.Errorf("roll = %d, want first draw %d", got.Total, want)
	}
	if !slices.Contains(res.Audit.Citations, "Mythras Imperative, Skills: How Skills Work") {
		t.Errorf("citations = %v", res.Audit.Citations)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "under hard difficulty") {
		t.Errorf("difficulty facts = %v, want hardest grade applied", res.Facts)
	}
	if !strings.Contains(joined, "outcome: ") {
		t.Errorf("facts = %v, want outcome line", res.Facts)
	}

	cases := []struct {
		name    string
		move    Move
		wantErr error
	}{
		{"unknown actor", SkillTest{Actor: "ghost", Skill: "Brawn"}, ErrUnknownCharacter},
		{"empty skill", SkillTest{Actor: "pc-1", Skill: ""}, ErrInvalidArgument},
		{"unknown skill", SkillTest{Actor: "pc-1", Skill: "Nonsense"}, rules.ErrUnknownSkill},
		{"unknown grade", SkillTest{Actor: "pc-1", Skill: "Brawn", Grade: "brutal"}, ErrInvalidArgument},
		{"hopeless refuses without rolling", SkillTest{Actor: "pc-1", Skill: "Brawn", Grade: "hopeless"}, rules.ErrHopeless},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seq, stream := s.EventSeq, string(s.RNGState)
			if _, err := Apply(s, tc.move); !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want wrapping %v", err, tc.wantErr)
			}
			if s.EventSeq != seq || string(s.RNGState) != stream {
				t.Error("refused test mutated counters or the dice stream")
			}
		})
	}
}

// TestOpposedTest covers the contest: both d100s audited in order, a
// winner line, and side-B validation happening before side A rolls.
func TestOpposedTest(t *testing.T) {
	const seed = 7
	s := newState(t, seed)
	addCharacter(t, s, "pc-1", true)
	addCharacter(t, s, "npc-1", false)

	res, err := Apply(s, OpposedTest{
		ActorA: "pc-1", SkillA: "Brawn",
		ActorB: "npc-1", SkillB: "Brawn",
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Audit == nil || len(res.Audit.Rolls) != 2 {
		t.Fatalf("audit rolls = %+v, want both d100s", res.Audit)
	}
	ref := dice.NewRand(seed)
	refA, refB := ref.D100(), ref.D100() // side A then side B, one draw each
	if res.Audit.Rolls[0].Total != refA || res.Audit.Rolls[1].Total != refB {
		t.Errorf("rolls = %d/%d, want %d/%d", res.Audit.Rolls[0].Total, res.Audit.Rolls[1].Total, refA, refB)
	}
	if !slices.Contains(res.Audit.Citations, "Mythras Imperative, Skills: Opposed Rolls") {
		t.Errorf("citations = %v", res.Audit.Citations)
	}
	if !strings.Contains(strings.Join(res.Facts, "\n"), "winner:") {
		t.Errorf("facts = %v, want a winner line", res.Facts)
	}
	if !strings.Contains(res.Summary, "opposed") {
		t.Errorf("summary = %q", res.Summary)
	}

	// Side B is validated before side A rolls: the stream stays put.
	stream := string(s.RNGState)
	if _, err := Apply(s, OpposedTest{
		ActorA: "pc-1", SkillA: "Brawn",
		ActorB: "npc-1", SkillB: "No Such Skill",
	}); !errors.Is(err, rules.ErrUnknownSkill) {
		t.Fatalf("error = %v, want rules.ErrUnknownSkill", err)
	}
	if string(s.RNGState) != stream {
		t.Error("side-B validation failure consumed side-A's roll")
	}
}

// TestLuckPoint covers the printed Luck uses: Cheat Fate's replacement
// d100 with its citation, plus the two refusals (bad purpose, empty pool).
func TestLuckPoint(t *testing.T) {
	const seed = 11
	s := newState(t, seed)
	addCharacter(t, s, "pc-1", true)
	want := dice.NewRand(seed).D100()

	res, err := Apply(s, LuckPoint{Actor: "pc-1", Purpose: "cheat fate"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Audit == nil || len(res.Audit.Rolls) != 1 || res.Audit.Rolls[0].Total != want {
		t.Fatalf("audit = %+v, want the first draw %d", res.Audit, want)
	}
	if !slices.Contains(res.Audit.Citations, "Mythras Imperative, Game System: Luck Points") {
		t.Errorf("citations = %v", res.Audit.Citations)
	}
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "Luck Points") || !strings.Contains(joined, "effect:") {
		t.Errorf("facts = %v, want pool and effect lines", res.Facts)
	}

	if _, err := Apply(s, LuckPoint{Actor: "pc-1", Purpose: "buy experience"}); !errors.Is(err, rules.ErrUnknownLuckUse) {
		t.Errorf("bad purpose error = %v, want rules.ErrUnknownLuckUse", err)
	}
	// A refused move before this point rebuilt the aggregate: re-query.
	s.Characters["pc-1"].Derived.LuckPoints = 0 // emptied/unrefreshed pool
	if _, err := Apply(s, LuckPoint{Actor: "pc-1", Purpose: "cheat fate"}); !errors.Is(err, rules.ErrNoLuck) {
		t.Errorf("empty pool error = %v, want rules.ErrNoLuck", err)
	}
	if _, err := Apply(s, LuckPoint{Actor: "ghost", Purpose: "cheat fate"}); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown actor error = %v, want ErrUnknownCharacter", err)
	}
}

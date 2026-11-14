package resolve

import (
	"errors"
	"strings"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/char"
)

// TestMoveKinds pins every move's Kind to its GM-facing tool name:
// harness-surface.md promises tools wrap moves 1:1, and the session log
// records Kind as the tool trail — a rename here would silently desync the
// log from the tool registry.
func TestMoveKinds(t *testing.T) {
	cases := map[string]Move{
		"roll_dice":            RollDice{},
		"skill_test":           SkillTest{},
		"opposed_test":         OpposedTest{},
		"luck_point":           LuckPoint{},
		"create_character":     CreateCharacter{},
		"finalize_character":   FinalizeCharacter{},
		"update_character":     UpdateCharacter{},
		"skill_advance":        SkillAdvance{},
		"inventory_add":        InventoryAdd{},
		"inventory_remove":     InventoryRemove{},
		"equip":                Equip{},
		"combat_start":         StartEncounter{},
		"spend_ap":             SpendAP{},
		"next_round":           NextRound{},
		"combat_end":           CombatEnd{},
		"resolve_attack":       ResolveAttack{},
		"apply_special_effect": ApplySpecialEffect{},
		"apply_damage":         ApplyDamage{},
		"heal":                 Heal{},
		"world_set_flag":       SetWorldFlag{},
		"add_clock":            AddClock{},
		"advance_clock":        AdvanceClock{},
		"advance_time":         AdvanceTime{},
		"set_scene":            SetScene{},
		"note":                 Note{},
		"end_session_recap":    EndRecap{},
	}
	if len(cases) != 26 {
		t.Fatalf("catalogue has %d moves, want 26 (harness-surface.md)", len(cases))
	}
	for want, m := range cases {
		if got := m.Kind(); got != want {
			t.Errorf("%T.Kind() = %q, want %q", m, got, want)
		}
	}
}

// TestFreeAdvanceOnPassion pins the passion branch of the free fumble
// increase: passions improve like skills, and the reader resolves the value
// from the Passions table.
func TestFreeAdvanceOnPassion(t *testing.T) {
	s := newState(t, 5)
	c := addCharacter(t, s, "pc-1", true)
	if err := c.AddPassion("Hate (Orcs)", "race", 2); err != nil {
		t.Fatalf("AddPassion: %v", err)
	}
	before := c.Passions["Hate (Orcs)"].Value
	c.Passions["Hate (Orcs)"].Fumbled = true

	if _, err := Apply(s, SkillAdvance{Actor: "pc-1", Skill: "Hate (Orcs)", Free: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	p := c.Passions["Hate (Orcs)"]
	if p.Value != before+1 || p.Fumbled {
		t.Errorf("passion = %d fumbled=%v, want %d with the flag cleared", p.Value, p.Fumbled, before+1)
	}
}

// TestExperienceAdvanceWithPendingFumble covers the free +1% folded into
// the comparison value: the roll is made against value-at-roll-time (old
// + 1) and the facts say so.
func TestExperienceAdvanceWithPendingFumble(t *testing.T) {
	s := newState(t, 9)
	c := addCharacter(t, s, "pc-1", true)
	if err := c.SetSkill("Stealth", 40); err != nil {
		t.Fatalf("SetSkill: %v", err)
	}
	c.Skills["Stealth"].Fumbled = true

	res, err := Apply(s, SkillAdvance{Actor: "pc-1", Skill: "Stealth"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	sk, _ := c.Skill("Stealth")
	if sk.Fumbled {
		t.Error("pending fumble flag survived the experience roll")
	}
	if sk.Value < 41 {
		t.Errorf("Stealth = %d, want at least the free +1%% (41)", sk.Value)
	}
	found := false
	for _, f := range res.Facts {
		if f == "pending fumble +1% applied before the roll" {
			found = true
		}
	}
	if !found {
		t.Errorf("facts = %v, want the pending-fumble line", res.Facts)
	}
}

// TestIdentifierFallbacks pins the label helpers' name fallback for
// characters without an engine ID (hand-built fixtures).
func TestIdentifierFallbacks(t *testing.T) {
	if got := idLabel("", "Torvik"); got != "Torvik" {
		t.Errorf("idLabel = %q, want the name fallback", got)
	}
	if got := idLabel("npc-1", "Torvik"); got != "npc-1" {
		t.Errorf("idLabel = %q, want the ID", got)
	}
	if stateLabel(true) != "dead" || stateLabel(false) != "alive" {
		t.Errorf("stateLabel = %q/%q", stateLabel(true), stateLabel(false))
	}
}

// TestUnknownCharacterListsKnownIDs pins the harness refusal message shape
// (harness-surface.md: IMPOSSIBLE with the real choices listed).
func TestUnknownCharacterListsKnownIDs(t *testing.T) {
	s := newState(t, 1)
	addCharacter(t, s, "pc-2", true)
	addCharacter(t, s, "npc-1", false)
	_, err := GetCharacter(s, "pc-9")
	if !errors.Is(err, ErrUnknownCharacter) {
		t.Fatalf("error = %v, want ErrUnknownCharacter", err)
	}
	want := `unknown character "pc-9" (known: npc-1, pc-2)`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("message = %q, want it to contain %q", err, want)
	}
	// The character created after a failed move must still be reachable:
	// CreateCharacter mints IDs from the known set without reuse.
	if _, err := Apply(s, CreateCharacter{
		Name: "Scout", PC: true, Culture: "Barbarian", Career: "Agent",
		Characteristics: &char.Characteristics{STR: 9, CON: 9, SIZ: 9, DEX: 9, INT: 9, POW: 9, CHA: 9},
	}); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}
	if _, err := GetCharacter(s, "pc-3"); err != nil {
		t.Errorf("pc-3 missing: %v", err)
	}
}

package tools

import (
	"strings"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/resolve"
)

// newTestRegistry builds a registry without a decision client: every tool
// except decide works offline, decide degrades to its IMPOSSIBLE result.
func newTestRegistry() *Registry {
	return NewRegistry(nil)
}

// newPC creates one player character and returns the state with it in play.
func newPC(t *testing.T, r *Registry) *resolve.State {
	t.Helper()
	s := resolve.NewState("tools-test", 42)
	mustOK(t, r, s, "create_character", map[string]any{
		"name": "Wren Ashdown", "pc": true, "culture": "Civilised", "career": "Hunter", "age": 26,
	})
	return s
}

// TestRulesTools exercises the codex lookups and the decide degradation.
func TestRulesTools(t *testing.T) {
	r := newTestRegistry()
	s := resolve.NewState("tools-test", 42)

	res := mustOK(t, r, s, "rules_lookup", map[string]any{"query": "opposed rolls", "limit": 3})
	if len(res.Facts) == 0 {
		t.Error("rules_lookup returned no facts")
	}

	res = mustOK(t, r, s, "rules_section", map[string]any{
		"doc": "Mythras-Imperative.md", "heading": "Dice Used",
	})
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "Mythras Imperative") && !strings.Contains(joined, "d100") {
		t.Errorf("rules_section facts do not carry the section text: %v", res.Facts)
	}

	// Unknown document is an argument refusal, not a crash.
	bad := dispatch(t, r, s, "rules_section", map[string]any{"doc": "nope.md", "heading": "Dice Used"})
	if bad.OK || !strings.HasPrefix(bad.Summary, "IMPOSSIBLE: invalid arguments for rules_section") {
		t.Errorf("unknown doc Summary = %q, want invalid-arguments refusal", bad.Summary)
	}

	// decide with no client degrades exactly as the contract specifies.
	dec := dispatch(t, r, s, "decide", map[string]any{
		"question": "Is a test needed?", "options": []string{"yes", "no"},
	})
	if dec.OK || dec.Summary != unavailableText {
		t.Errorf("decide Summary = %q, want %q", dec.Summary, unavailableText)
	}
}

// TestDiceTools exercises the four dice and test tools against one seeded
// state.
func TestDiceTools(t *testing.T) {
	r := newTestRegistry()
	s := newPC(t, r)
	mustOK(t, r, s, "create_character", map[string]any{
		"name": "Grimjaw", "pc": false, "culture": "Barbarian", "career": "Bounty Hunter",
	})

	res := mustOK(t, r, s, "roll_dice", map[string]any{"expr": "2d6+3", "reason": "foraging yield"})
	if res.Audit == nil || len(res.Audit.Rolls) == 0 {
		t.Errorf("roll_dice audit = %+v, want rolls", res.Audit)
	}

	res = mustOK(t, r, s, "skill_test", map[string]any{"actor": "pc-1", "skill": "Perception", "grade": "hard"})
	if !strings.Contains(strings.Join(res.Facts, " "), "outcome:") {
		t.Errorf("skill_test facts = %v, want outcome grade", res.Facts)
	}

	res = mustOK(t, r, s, "opposed_test", map[string]any{
		"actor_a": "pc-1", "skill_a": "Stealth", "actor_b": "npc-1", "skill_b": "Stealth",
	})
	if !strings.Contains(strings.Join(res.Facts, " "), "winner:") {
		t.Errorf("opposed_test facts = %v, want winner line", res.Facts)
	}

	res = mustOK(t, r, s, "luck_point", map[string]any{"actor": "pc-1", "purpose": "cheat fate"})
	if !strings.Contains(strings.Join(res.Facts, " "), "replacement roll:") {
		t.Errorf("luck_point facts = %v, want replacement roll", res.Facts)
	}
}

// TestCharacterTools walks the creation → finalization → read → update →
// improvement arc.
func TestCharacterTools(t *testing.T) {
	r := newTestRegistry()
	s := resolve.NewState("tools-test", 42)

	res := mustOK(t, r, s, "create_character", map[string]any{
		"name": "Wren Ashdown", "pc": true, "culture": "Civilised", "career": "Hunter",
		"skills": map[string]int{"Perception": 65},
	})
	if !strings.Contains(strings.Join(res.Facts, " "), "id: pc-1") {
		t.Errorf("create_character facts = %v, want id: pc-1", res.Facts)
	}

	mustOK(t, r, s, "finalize_character", map[string]any{
		"actor":     "pc-1",
		"equipment": []map[string]any{{"name": "Broadsword"}},
		"passions":  []map[string]any{{"name": "Hate (Orcs)", "kind": "race", "order": 1}},
	})

	res = mustOK(t, r, s, "get_character", map[string]any{"actor": "pc-1"})
	joined := strings.Join(res.Facts, "\n")
	for _, want := range []string{"characteristics:", "derived:", "inventory:", "skills:", "passions:"} {
		if !strings.Contains(joined, want) {
			t.Errorf("get_character facts missing %q", want)
		}
	}

	mustOK(t, r, s, "update_character", map[string]any{
		"actor": "pc-1",
		"changes": map[string]any{
			"concept": "A cartographer who trusts maps more than people.",
			"notes":   "Owes the ferryman a favour.",
		},
	})

	res = mustOK(t, r, s, "list_characters", map[string]any{"filter": "pc"})
	if !strings.Contains(strings.Join(res.Facts, " "), "pc-1: Wren Ashdown") {
		t.Errorf("list_characters facts = %v, want the PC line", res.Facts)
	}

	res = mustOK(t, r, s, "skill_advance", map[string]any{"actor": "pc-1", "skill": "Perception"})
	if !strings.Contains(strings.Join(res.Facts, " "), "experience roll:") {
		t.Errorf("skill_advance facts = %v, want experience roll", res.Facts)
	}

	// One Experience Roll per session per skill.
	again := dispatch(t, r, s, "skill_advance", map[string]any{"actor": "pc-1", "skill": "Perception"})
	if again.OK || !strings.HasPrefix(again.Summary, "IMPOSSIBLE: skill already received") {
		t.Errorf("second skill_advance Summary = %q, want already-experienced refusal", again.Summary)
	}
}

// TestInventoryTools exercises add → remove → equip → advisory carrying.
func TestInventoryTools(t *testing.T) {
	r := newTestRegistry()
	s := newPC(t, r)
	mustOK(t, r, s, "finalize_character", map[string]any{
		"actor":     "pc-1",
		"equipment": []map[string]any{{"name": "Broadsword"}},
	})

	mustOK(t, r, s, "inventory_add", map[string]any{
		"actor": "pc-1", "item": "arrows", "qty": 10, "tags": []string{"ammunition"},
	})
	mustOK(t, r, s, "inventory_remove", map[string]any{"actor": "pc-1", "item": "arrows", "qty": 3})

	pc := s.Characters["pc-1"]
	left, err := pc.ArrowsLeft()
	if err != nil {
		t.Fatalf("ArrowsLeft: %v", err)
	}
	if left != 7 {
		t.Errorf("arrows left = %d, want 7", left)
	}

	mustOK(t, r, s, "equip", map[string]any{"actor": "pc-1", "item": "Broadsword", "slot": "main"})

	res := mustOK(t, r, s, "check_encumbrance", map[string]any{"actor": "pc-1"})
	joined := strings.Join(res.Facts, "\n")
	if !strings.Contains(joined, "advisory:") || !strings.Contains(joined, "no encumbrance rules") {
		t.Errorf("check_encumbrance facts = %v, want advisory disclaimer", res.Facts)
	}

	// Removing more than carried is an engine refusal, not a crash.
	over := dispatch(t, r, s, "inventory_remove", map[string]any{"actor": "pc-1", "item": "arrows", "qty": 99})
	if over.OK || !strings.HasPrefix(over.Summary, "IMPOSSIBLE: not enough items") {
		t.Errorf("over-removal Summary = %q, want not-enough-items refusal", over.Summary)
	}
}

// TestCombatTools runs a minimal encounter: start, checks, rounds, an
// exchange, environmental damage, healing and the end.
func TestCombatTools(t *testing.T) {
	r := newTestRegistry()
	s := newPC(t, r)
	mustOK(t, r, s, "create_character", map[string]any{
		"name": "Grimjaw", "pc": false, "culture": "Barbarian", "career": "Bounty Hunter",
	})

	res := mustOK(t, r, s, "combat_start", map[string]any{"participants": []string{"pc-1", "npc-1"}})
	if !strings.Contains(strings.Join(res.Facts, " "), "round 1") {
		t.Errorf("combat_start facts = %v, want round 1", res.Facts)
	}

	res = mustOK(t, r, s, "can_act", map[string]any{"actor": "pc-1", "action": "Attack"})
	if !strings.Contains(strings.Join(res.Facts, " "), "allowed: true") {
		t.Errorf("can_act facts = %v, want allowed: true", res.Facts)
	}

	mustOK(t, r, s, "spend_ap", map[string]any{"actor": "pc-1", "action": "Move"})
	res = mustOK(t, r, s, "combat_status", nil)
	if !strings.Contains(strings.Join(res.Facts, " "), "1 AP") {
		t.Errorf("combat_status facts = %v, want pc-1 on 1 AP", res.Facts)
	}

	mustOK(t, r, s, "next_round", nil)
	mustOK(t, r, s, "resolve_attack", map[string]any{
		"attacker": "pc-1", "defender": "npc-1", "reaction": "none",
	})
	mustOK(t, r, s, "apply_damage", map[string]any{"actor": "npc-1", "amount": 3})
	mustOK(t, r, s, "heal", map[string]any{"actor": "npc-1", "method": "first aid"})

	mustOK(t, r, s, "combat_end", nil)
	if s.Encounter == nil || !s.Encounter.Ended {
		t.Fatalf("encounter after combat_end = %+v, want ended record", s.Encounter)
	}

	// The ended record stays readable; a second end is refused.
	res = mustOK(t, r, s, "combat_status", nil)
	if !strings.Contains(strings.Join(res.Facts, " "), "state: ended") {
		t.Errorf("combat_status after end facts = %v, want state: ended", res.Facts)
	}
	again := dispatch(t, r, s, "combat_end", nil)
	if again.OK || !strings.HasPrefix(again.Summary, "IMPOSSIBLE: encounter is not running") {
		t.Errorf("second combat_end Summary = %q, want not-running refusal", again.Summary)
	}
}

// TestWorldTools walks the world and continuity tools, creating the
// progress clock through add_clock and then filling it with advance_clock.
func TestWorldTools(t *testing.T) {
	r := newTestRegistry()
	s := resolve.NewState("tools-test", 42)
	res := mustOK(t, r, s, "add_clock", map[string]any{
		"name": "unrest", "segments": 6, "on_complete": []string{"the mob storms the granary"},
	})
	if !strings.Contains(strings.Join(res.Facts, " "), "0 of 6 segments") {
		t.Errorf("add_clock facts = %v, want the fresh clock state", res.Facts)
	}
	dup := dispatch(t, r, s, "add_clock", map[string]any{"name": "unrest", "segments": 4})
	if dup.OK || !strings.HasPrefix(dup.Summary, "IMPOSSIBLE: clock already exists") {
		t.Errorf("duplicate add_clock Summary = %q, want already-exists refusal", dup.Summary)
	}

	mustOK(t, r, s, "set_scene", map[string]any{
		"location": "The Salt Roads", "description": "A wind-scoured trade road.", "present": []string{"pc-1"},
	})
	mustOK(t, r, s, "note", map[string]any{
		"topic": "clues", "text": "The ferryman knows the smugglers.", "tags": []string{"salt roads"},
	})

	res = mustOK(t, r, s, "world_set_flag", map[string]any{
		"key": "economy", "value": "strained", "reason": "the baron's blockade",
	})
	if !strings.Contains(strings.Join(res.Facts, " "), "economy: (unset) → strained") {
		t.Errorf("world_set_flag facts = %v, want old → new", res.Facts)
	}

	res = mustOK(t, r, s, "world_get", map[string]any{"key": "economy"})
	if !strings.Contains(strings.Join(res.Facts, " "), "value: strained") {
		t.Errorf("world_get facts = %v, want strained", res.Facts)
	}
	mustOK(t, r, s, "world_get", nil)

	res = mustOK(t, r, s, "can_acquire", map[string]any{"item": "Broadsword", "rarity": "rare"})
	if !strings.Contains(strings.Join(res.Facts, " "), "allowed:") {
		t.Errorf("can_acquire facts = %v, want allowed verdict", res.Facts)
	}

	mustOK(t, r, s, "advance_clock", map[string]any{
		"clock": "unrest", "segments": 2, "reason": "blockade rumours",
	})
	if got := s.World.Clocks["unrest"].Filled; got != 2 {
		t.Errorf("unrest filled = %d, want 2", got)
	}
	unknownClock := dispatch(t, r, s, "advance_clock", map[string]any{
		"clock": "no-such-clock", "segments": 1, "reason": "x",
	})
	if unknownClock.OK || !strings.HasPrefix(unknownClock.Summary, "IMPOSSIBLE: unknown clock") {
		t.Errorf("unknown clock Summary = %q, want unknown-clock refusal", unknownClock.Summary)
	}

	mustOK(t, r, s, "advance_time", map[string]any{"days": 3, "hours": 6})

	mustOK(t, r, s, "end_session_recap", map[string]any{
		"summary": "Wren and Grimjaw struck a deal at the wayside shrine.",
	})
	res = mustOK(t, r, s, "session_recap", nil)
	if !strings.Contains(strings.Join(res.Facts, " "), "wayside shrine") {
		t.Errorf("session_recap facts = %v, want the stored recap", res.Facts)
	}
}

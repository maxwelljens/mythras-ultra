package resolve

import (
	"errors"
	"strings"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/engine/world"
)

// TestGetCharacter covers the folded PC/NPC lookup and its refusal.
func TestGetCharacter(t *testing.T) {
	s := newState(t, 1)
	pc := addCharacter(t, s, "pc-1", true)
	npc := addCharacter(t, s, "npc-1", false)

	gotPC, err := GetCharacter(s, "pc-1")
	if err != nil || gotPC != pc {
		t.Fatalf("GetCharacter(pc-1) = %v, %v", gotPC, err)
	}
	gotNPC, err := GetCharacter(s, "npc-1") // get_npc folds into this
	if err != nil || gotNPC != npc || gotNPC.PC {
		t.Fatalf("GetCharacter(npc-1) = %v, %v", gotNPC, err)
	}
	if _, err := GetCharacter(s, "ghost"); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown error = %v, want ErrUnknownCharacter", err)
	}
	if !strings.Contains(func() string { _, e := GetCharacter(s, "ghost"); return e.Error() }(), "known: npc-1, pc-1") {
		t.Error("unknown-character message must list the known IDs")
	}
	if _, err := GetCharacter(nil, "pc-1"); !errors.Is(err, ErrInvalidState) {
		t.Errorf("nil state error = %v, want ErrInvalidState", err)
	}
}

// TestListCharacters covers the stable ID ordering and nil safety.
func TestListCharacters(t *testing.T) {
	s := newState(t, 1)
	addCharacter(t, s, "pc-2", true)
	addCharacter(t, s, "npc-1", false)
	addCharacter(t, s, "pc-1", true)

	list := ListCharacters(s)
	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
	wantOrder := []string{"npc-1", "pc-1", "pc-2"}
	for i, c := range list {
		if c.ID != wantOrder[i] {
			t.Errorf("list[%d] = %q, want %q", i, c.ID, wantOrder[i])
		}
	}
	if got := ListCharacters(nil); got != nil {
		t.Errorf("ListCharacters(nil) = %v, want nil", got)
	}
}

// TestCombatStatus covers the status query's happy and refusal paths.
func TestCombatStatus(t *testing.T) {
	s := newState(t, 1)
	addCharacter(t, s, "pc-1", true)
	if _, err := CombatStatus(s); !errors.Is(err, ErrNoEncounter) {
		t.Errorf("no-encounter error = %v, want ErrNoEncounter", err)
	}
	st, _, _ := duelState(t, 1)
	rep, err := CombatStatus(st)
	if err != nil {
		t.Fatalf("CombatStatus: %v", err)
	}
	if rep.Round != 1 || len(rep.Participants) != 2 || len(rep.Errors) != 0 {
		t.Errorf("status = %+v", rep)
	}
	if _, err := CombatStatus(nil); !errors.Is(err, ErrInvalidState) {
		t.Errorf("nil state error = %v, want ErrInvalidState", err)
	}
}

// TestWorldGet covers the summary and single-flag reads.
func TestWorldGet(t *testing.T) {
	s := newState(t, 1)
	summary, err := WorldGet(s, "")
	if err != nil {
		t.Fatalf("WorldGet: %v", err)
	}
	if !strings.Contains(summary, s.Name) {
		t.Errorf("summary does not name the world: %.60s", summary)
	}
	if _, err := Apply(s, SetWorldFlag{Key: "omen", Value: "comet", Reason: "overhead"}); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	v, err := WorldGet(s, "omen")
	if err != nil || v != "comet" {
		t.Errorf("WorldGet(omen) = %q, %v", v, err)
	}
	if _, err := WorldGet(s, "missing"); !errors.Is(err, world.ErrUnknownFlag) {
		t.Errorf("missing flag error = %v, want world.ErrUnknownFlag", err)
	}
	if _, err := WorldGet(&State{}, ""); !errors.Is(err, ErrNoWorld) {
		t.Errorf("no-world error = %v, want ErrNoWorld", err)
	}
}

// TestCanAcquire covers the availability query: a peaceful world admits
// common goods, rarity validation refuses bad input, and nothing mutates.
func TestCanAcquire(t *testing.T) {
	s := newState(t, 1)
	d, err := CanAcquire(s, "a spear", world.RarityCommon, "")
	if err != nil {
		t.Fatalf("CanAcquire: %v", err)
	}
	if !d.Allowed {
		t.Errorf("common good refused on a fresh world: %+v", d)
	}
	if len(d.Reasons) == 0 {
		t.Error("decision carries no reasoning")
	}
	if _, err := CanAcquire(s, "a spear", "mystical", ""); !errors.Is(err, world.ErrInvalidRarity) {
		t.Errorf("bad rarity error = %v, want world.ErrInvalidRarity", err)
	}
	if _, err := CanAcquire(&State{}, "x", world.RarityCommon, ""); !errors.Is(err, ErrNoWorld) {
		t.Errorf("no-world error = %v, want ErrNoWorld", err)
	}
	// A collapsed economy strains availability (typed flags feed the table).
	if _, err := Apply(s, SetWorldFlag{Key: world.FlagEconomy, Value: world.EconomyCollapsed, Reason: "trade has broken down"}); err != nil {
		t.Fatalf("economy: %v", err)
	}
	legendary, err := CanAcquire(s, "a legend-forged blade", world.RarityLegendary, "")
	if err != nil {
		t.Fatalf("CanAcquire legendary: %v", err)
	}
	if legendary.Allowed {
		t.Errorf("legendary still allowed under collapsed economy: %+v", legendary)
	}
}

// TestCanAct covers the advisory action check.
func TestCanAct(t *testing.T) {
	s := newState(t, 1)
	addCharacter(t, s, "pc-1", true)
	if _, _, err := CanAct(s, "pc-1", "attack"); !errors.Is(err, ErrNoEncounter) {
		t.Errorf("no-encounter error = %v, want ErrNoEncounter", err)
	}

	st, actor, _ := duelState(t, 1)
	ok, blockers, err := CanAct(st, actor, "attack")
	if err != nil {
		t.Fatalf("CanAct: %v", err)
	}
	if !ok || len(blockers) != 0 {
		t.Errorf("fresh combatant cannot attack: ok=%v blockers=%v", ok, blockers)
	}
	ok, blockers, err = CanAct(st, actor, "breakdance")
	if err != nil {
		t.Fatalf("CanAct unknown: %v", err)
	}
	if ok || len(blockers) == 0 {
		t.Errorf("unknown action accepted: ok=%v blockers=%v", ok, blockers)
	}
	if _, _, err := CanAct(st, "ghost", "attack"); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown actor error = %v, want ErrUnknownCharacter", err)
	}
}

// TestCheckCarrying covers the advisory bulk count (no encumbrance rules).
func TestCheckCarrying(t *testing.T) {
	s := newState(t, 1)
	c := addCharacter(t, s, "pc-1", true)
	if n, err := CheckCarrying(s, "pc-1"); err != nil || n != 0 {
		t.Errorf("CheckCarrying = %d, %v; want 0", n, err)
	}
	if _, err := Apply(s, InventoryAdd{Actor: "pc-1", Item: "Rations", Qty: 5}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if n, err := CheckCarrying(s, "pc-1"); err != nil || n != 5 {
		t.Errorf("CheckCarrying = %d, %v; want 5", n, err)
	}
	if c.Carrying() != 5 {
		t.Errorf("direct Carrying = %d", c.Carrying())
	}
	if _, err := CheckCarrying(s, "ghost"); !errors.Is(err, ErrUnknownCharacter) {
		t.Errorf("unknown actor error = %v, want ErrUnknownCharacter", err)
	}
}

// TestSessionRecap covers the recap read: empty for a fresh session, nil
// state safe.
func TestSessionRecap(t *testing.T) {
	s := newState(t, 1)
	if got := SessionRecap(s); len(got) != 0 {
		t.Errorf("fresh recaps = %v, want none", got)
	}
	if got := SessionRecap(nil); got != nil {
		t.Errorf("SessionRecap(nil) = %v, want nil", got)
	}
	if _, err := Apply(s, EndRecap{Summary: "one"}); err != nil {
		t.Fatalf("EndRecap: %v", err)
	}
	if _, err := Apply(s, EndRecap{Summary: "two"}); err != nil {
		t.Fatalf("EndRecap: %v", err)
	}
	got := SessionRecap(s)
	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Errorf("recaps = %v, want [one two] oldest first", got)
	}
}

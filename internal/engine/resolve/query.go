package resolve

import (
	"fmt"
	"sort"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/combat"
	"github.com/mythras-ultra/myul/internal/engine/world"
)

// This file holds the read-only queries: plain functions returning data,
// never moves — they mutate nothing and consume no dice (harness-surface.md:
// "get_character", "combat_status", ... are reads). The harness projects
// their values into tool results.

// GetCharacter returns the character with the given ID — PCs and NPCs
// alike; char.Character.PC distinguishes them (tool: get_character; the
// get_npc tool folds into this one). The returned pointer aliases session
// state: treat it as read-only, and make changes through their own moves.
// An unknown ID errors wrapping [ErrUnknownCharacter], listing the known
// IDs.
func GetCharacter(s *State, id string) (*char.Character, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: nil state", ErrInvalidState)
	}
	return s.character(id)
}

// ListCharacters returns every character in the session, PCs and NPCs,
// sorted by ID for a stable listing (tool: list_characters; any filter is
// the tool's presentation concern). Pointers alias session state —
// read-only.
func ListCharacters(s *State) []*char.Character {
	if s == nil {
		return nil
	}
	out := make([]*char.Character, 0, len(s.Characters))
	for _, c := range s.Characters {
		if c != nil {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// CombatStatus reports the running (or just-ended) encounter: round,
// strike order and every participant's Action Points, conditions and hit
// locations (tool: combat_status). Errors wrapping [ErrNoEncounter] when no
// encounter has been started.
func CombatStatus(s *State) (combat.StatusReport, error) {
	if s == nil {
		return combat.StatusReport{}, fmt.Errorf("%w: nil state", ErrInvalidState)
	}
	e, err := s.encounter()
	if err != nil {
		return combat.StatusReport{}, err
	}
	return e.Status(), nil
}

// WorldGet returns the world as a compact structured block, or the value of
// one flag when key is non-empty (tool: world_get). Errors wrapping
// [ErrNoWorld] for states without a world, or world.ErrUnknownFlag for a
// missing key.
func WorldGet(s *State, key string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("%w: nil state", ErrInvalidState)
	}
	if s.World == nil {
		return "", ErrNoWorld
	}
	if key == "" {
		return s.World.Summary(), nil
	}
	return s.World.Get(key)
}

// CanAcquire reports whether an item of the given rarity can be obtained in
// a region under the world's current flags, and why (tool: can_acquire).
// Pure world query — no dice, no mutation; world.ErrInvalidRarity and
// friends propagate.
func CanAcquire(s *State, item, rarity, region string) (world.Decision, error) {
	if s == nil {
		return world.Decision{}, fmt.Errorf("%w: nil state", ErrInvalidState)
	}
	if s.World == nil {
		return world.Decision{}, ErrNoWorld
	}
	return s.World.CanAcquire(item, rarity, region)
}

// CanAct reports whether the actor may currently perform the named Combat
// Action, and if not, every blocker as a human-readable reason (tool:
// can_act; combat.CanAct — advisory: it never spends Action Points).
func CanAct(s *State, actor, action string) (bool, []string, error) {
	if s == nil {
		return false, nil, fmt.Errorf("%w: nil state", ErrInvalidState)
	}
	e, err := s.encounter()
	if err != nil {
		return false, nil, err
	}
	c, err := s.character(actor)
	if err != nil {
		return false, nil, err
	}
	ok, blockers := combat.CanAct(e, c, action)
	return ok, blockers, nil
}

// CheckCarrying returns the advisory carried-item count (tool:
// check_encumbrance). Mythras Imperative defines no encumbrance rules
// (digest §9) — nothing in the engine penalises this number; it is GM
// judgement input only.
func CheckCarrying(s *State, actor string) (int, error) {
	if s == nil {
		return 0, fmt.Errorf("%w: nil state", ErrInvalidState)
	}
	c, err := s.character(actor)
	if err != nil {
		return 0, err
	}
	return c.Carrying(), nil
}

// SessionRecap returns the stored recaps, oldest first — the context-rebuild
// input the harness feeds back into the system prompt (tool:
// session_recap; the summary itself is written by [EndRecap]).
func SessionRecap(s *State) []string {
	if s == nil {
		return nil
	}
	return s.Recaps
}

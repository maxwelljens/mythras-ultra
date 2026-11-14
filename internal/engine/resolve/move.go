package resolve

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Move is one typed, auditable game operation: [RollDice], [SkillTest],
// [StartEncounter] and friends. A Move validates against the state before it
// mutates anything, executes mechanics only through the engine subpackages,
// and returns a [Result] whose Facts state the after-values the GM narrates
// from (ARCHITECTURE.md §4.5, harness-surface.md).
//
// Implementations must be pure descriptions of intent: no clocks, no
// randomness, no I/O — the session RNG comes from State.Rand and every
// mutation runs under [Apply], which rolls the whole move back when it
// returns an error.
type Move interface {
	// Kind is the move's stable identifier, matching the GM-facing tool
	// name (harness-surface.md) so the session log reads as the tool trail.
	Kind() string
	// Apply validates and executes the move against s, mutating state only
	// via engine subpackages. On error nothing in s has changed: [Apply]
	// restores the pre-move state and the pre-move RNG position.
	Apply(s *State) (Result, error)
}

// Sentinel errors returned by the move layer (wrapped with %w, like every
// engine sentinel, so harness tools can translate them into structured
// IMPOSSIBLE results — harness-surface.md, Tool contract).
var (
	// ErrInvalidState reports a missing or structurally broken state: a nil
	// *State passed to [Apply], or deserialised data Normalize refuses
	// (dangling references, unreadable RNG stream, out-of-range indices).
	ErrInvalidState = errors.New("resolve: invalid state")

	// ErrUnknownCharacter reports a character ID the session does not
	// hold. The wrapped message lists the known IDs.
	ErrUnknownCharacter = errors.New("resolve: unknown character")

	// ErrNoEncounter reports a combat operation without a running
	// encounter: call [StartEncounter] first.
	ErrNoEncounter = errors.New("resolve: no encounter running")

	// ErrEncounterActive reports a [StartEncounter] call while an
	// encounter is already running — end it first.
	ErrEncounterActive = errors.New("resolve: encounter already running")

	// ErrNoWorld reports a state without world data (hand-built states;
	// [NewState] always provides one).
	ErrNoWorld = errors.New("resolve: no world in state")

	// ErrNoAttack reports [ApplySpecialEffect] with no recorded exchange —
	// resolve an attack first (or start a new encounter).
	ErrNoAttack = errors.New("resolve: no recorded attack")

	// ErrInvalidArgument reports an empty or otherwise unacceptable move
	// argument detected before any mutation (empty name, zero duration,
	// no changes, malformed namespaced flag key).
	ErrInvalidArgument = errors.New("resolve: invalid argument")

	// ErrDanglingParticipant reports deserialised encounter state that
	// references a character the session does not hold, or structural
	// indices outside the participant list — [State.Normalize] refuses it
	// so a corrupt state.json is never loaded into play.
	ErrDanglingParticipant = errors.New("resolve: encounter references unknown character")

	// ErrNotEnoughItems reports an [InventoryRemove] larger than the
	// quantity carried; the wrapped message states both amounts.
	ErrNotEnoughItems = errors.New("resolve: not enough items")
)

// Apply runs one Move against the state — the single mutation entry point
// (golden rule 1). It snapshots the state, executes the move, and then:
//
//   - on success, increments EventSeq and marshals the live RNG stream into
//     State.RNGState, so the next save resumes exactly here;
//   - on error, restores the entire pre-move state — data, EventSeq and RNG
//     position — so a failed move is invisible: no partial mutation, and no
//     dice consumed (session-loop.md, "moves are atomic").
//
// UpdatedAt is NOT stamped here: the engine never reads clocks; the caller
// and the persistence layer own wall time. Errors are the move's own,
// wrapping engine sentinels with %w.
func Apply(s *State, m Move) (Result, error) {
	if s == nil {
		return Result{}, fmt.Errorf("%w: nil state", ErrInvalidState)
	}
	if m == nil {
		return Result{}, fmt.Errorf("%w: nil move", ErrInvalidState)
	}
	before, err := json.Marshal(s)
	if err != nil {
		return Result{}, fmt.Errorf("resolve: snapshot state before %s: %w", m.Kind(), err)
	}
	// rollback restores every field from the snapshot. The unexported rng
	// becomes nil, so the next State.Rand re-restores the pre-move stream
	// position from RNGState — a failed move consumes no randomness.
	// Normalize re-binds encounter participants to the restored character
	// instances (a JSON decode copies them), so the aggregate is
	// structurally identical — pointers included — to what it was before
	// the move. Because the aggregate is rebuilt wholesale, callers must
	// re-query characters and other references after an error (the harness
	// re-reads state on every tool call; session-loop.md: moves are atomic).
	rollback := func(cause error) error {
		var fresh State
		if uerr := json.Unmarshal(before, &fresh); uerr != nil {
			return errors.Join(cause, fmt.Errorf("resolve: state rollback after %s failed: %w", m.Kind(), uerr))
		}
		*s = fresh
		if nerr := s.Normalize(); nerr != nil {
			return errors.Join(cause, fmt.Errorf("resolve: rollback identity re-bind after %s failed: %w", m.Kind(), nerr))
		}
		return cause
	}
	res, err := m.Apply(s)
	if err != nil {
		return Result{}, rollback(err)
	}
	s.EventSeq++
	if s.rng != nil {
		stream, err := s.rng.MarshalBinary()
		if err != nil {
			return Result{}, rollback(fmt.Errorf("resolve: persist RNG state after %s: %w", m.Kind(), err))
		}
		s.RNGState = stream
	}
	return res, nil
}

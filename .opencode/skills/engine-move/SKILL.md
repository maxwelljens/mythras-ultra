---
id: engine-move
name: engine-move
description: Add a new deterministic game operation (Move) to internal/engine/resolve end-to-end — typed move struct, state validation, execution in engine subpackages, Result with facts + dice audit + rule citation, sentinel errors, seeded unit tests, and session-log wiring. Use when adding, extending, or debugging a Move such as RollDice, SkillTest, ResolveAttack, SetWorldFlag, or AdvanceClock.
---

# engine-move

A `Move` is the only way game state changes. It is a typed, auditable
operation in `internal/engine/resolve`: validate against state → execute via
an engine subpackage → return a `Result` (summary, machine facts, dice audit,
rule citation) → append to the session log. Moves are both the engine's public
API and the mock-session test surface (ARCHITECTURE.md §4.5, §3.1).

## Recipe

1. **Pick the execution subpackage first.** Mechanical logic lives in
   `internal/engine/dice`, `char`, `rules`, `combat`, or `world` — never in
   `resolve`. `resolve` only validates, sequences, and audits. `engine/*`
   imports stdlib + tiny deps only (layering rule, AGENTS.md golden rule 4).
2. **Add the typed move** in `internal/engine/resolve` — one struct (or
   `match`-style variant) per operation with explicit input fields. No maps of
   untyped `any`; inputs must be validatable before mutation.
3. **Validate against current state** before any mutation: preconditions
   (character exists, encounter active, enough AP, item present). Failures
   return a sentinel error (e.g. `ErrNotEnoughAP`, `ErrNoEncounter`) wrapped
   with `%w` — never a panic, never a partial mutation.
4. **Execute** by calling the subpackage functions with the explicit
   `*State`; the session RNG is `State.Rand()` — a seedable `*dice.Rand`.
   All randomness flows through `engine/dice` (ARCHITECTURE.md §3.1) — no
   `math/rand` use elsewhere, no coin flips in `resolve`.
5. **Return a `Result`**: human summary, machine `Facts` (before/after
   deltas the GM narrates from), and an `Audit` carrying the dice rolls
   (expression, dice, modifier, total), rule citations where applicable
   (e.g. `"Mythras Imperative, Skills: Modifying Skills"`), and
   before → after `Changes`. The applied difficulty must be visible in the
   facts ("45% base → 65% under very easy difficulty").
6. **Wire into the state log**: accepted moves mutate state, then the caller
   path persists `state.json` atomically and appends to `log.jsonl`
   (ARCHITECTURE.md §5). Do not bypass `resolve` from `harness` or `tui`.
7. **Unit test** in `internal/engine/resolve`: table-driven, fixed
   `dice.NewRand(seed)`, asserting on `Result.Facts` and audit entries — not
   just error/success. Cover the happy path and each sentinel error.
8. **If the move will be GM-facing**, pair it with a tool (see the
   `harness-tool` skill) and a scripted turn in
   `internal/harness/testdata/mocksession/`.

## Code shape (the real shape — see `moves_combat.go`, `result.go`)

```go
// internal/engine/resolve
var ErrNotEnoughAP = errors.New("not enough action points")

// A move implements Move: Kind() + Apply(s *State).
type SpendAP struct {
    Actor  string // character ID
    Action string // printed Combat Action name
}

func (m SpendAP) Kind() string { return "spend_ap" }

func (m SpendAP) Apply(s *State) (Result, error) {
    // 1. validate against s → sentinel error (wrapped with %w), no mutation
    // 2. execute via engine/combat, drawing dice from s.Rand()
    // 3. mutate s only here; resolve.Apply restores the pre-move state
    //    and RNG position on error — nothing partial ever escapes
    // 4. the session then persists state.json + appends log.jsonl
    ...
}

type Result struct {
    Summary string   // human line for the GM/UI
    Facts   []string // authoritative before/after facts
    Audit   *Audit   // Rolls, Citations, Changes; nil when empty
}
```

## Checklist — done when all exist

- [ ] Typed move struct in `internal/engine/resolve` with validation
      against state before any mutation.
- [ ] Execution delegated to the right `engine/*` subpackage; randomness only
      via `dice.NewRand`-supplied `*Rand`.
- [ ] `Result` carries facts + dice audit + rule citation (where applicable).
- [ ] Sentinel errors defined in `engine`, wrapped with `%w`, no panics.
- [ ] Accepted move appends to the session log and persists `state.json`.
- [ ] Table-driven unit tests with a fixed seed, asserting `Result.Facts`
      and audit; error paths covered.
- [ ] `go test ./...` passes offline; `just lint` clean; layering intact
      (no `tui`/`harness`/`cmd` imports).

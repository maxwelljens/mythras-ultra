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
   `*State` and the session `*dice.Rand`. All randomness flows through
   `engine/dice` with the seedable RNG (ARCHITECTURE.md §3.1) — no
   `math/rand` use elsewhere, no coin flips in `resolve`.
5. **Return a `Result`**: human summary, machine `Facts` (before/after
   deltas the GM narrates from), the dice `Audit` entries (each roll with
   expression, rolls, modifiers), and the rule reference where applicable
   (e.g. `"Mythras Imperative, Combat §3.2"`). Applied modifiers must appear
   in the audit ("+20 difficult, −10 darkness").
6. **Wire into the state log**: accepted moves mutate state, then the caller
   path persists `state.json` atomically and appends to `log.jsonl`
   (ARCHITECTURE.md §5). Do not bypass `resolve` from `harness` or `tui`.
7. **Unit test** in `internal/engine/resolve`: table-driven, fixed
   `dice.NewRand(seed)`, asserting on `Result.Facts` and audit entries — not
   just error/success. Cover the happy path and each sentinel error.
8. **If the move will be GM-facing**, pair it with a tool (see the
   `harness-tool` skill) and a `testdata/mocksession/` turn.

## Code shape (guidance, not gospel)

```go
// internal/engine/resolve — names illustrative; follow existing moves.
var ErrNotEnoughAP = errors.New("not enough action points")

type SpendAPMove struct {
    ActorID string
    Cost    int
    Reason  string
}

func (m SpendAPMove) Execute(st *State, rnd *dice.Rand) (Result, error) {
    // 1. validate → sentinel error (wrapped with %w), no mutation
    // 2. execute via engine/combat (AP economy) using rnd
    // 3. build Result{Summary, Facts []Fact, Audit []dice.AuditEntry, Rule}
    // 4. mutate st only here; caller persists state.json + log.jsonl
}

type Result struct {
    Summary string        // human line for the GM/UI
    Facts   []Fact        // machine before/after deltas
    Audit   []dice.AuditEntry
    Rule    string        // "Mythras Imperative, <section>"
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

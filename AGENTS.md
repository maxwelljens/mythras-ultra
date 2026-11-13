# Mythras Ultra — Agent Guidelines

`myul` is a production-grade Go harness for LLM-driven Mythras Imperative
play. This file contains concrete development guidelines for humans and
coding agents. Read `docs/ARCHITECTURE.md` first — it is the design contract.

## Golden rules

1. **Determinism is sacred.** Mechanical outcomes (rolls, tests, damage,
   inventory, world flags) only change through `engine` moves. Never let the
   LLM or UI code invent or mutate game facts directly.
2. **No shortcuts.** No TODOs in shipped code, no panics on user input,
   no swallowed errors. Production-grade: errors wrapped with `%w`, validated
   inputs, documented exported symbols.
3. **Test what matters.** Every engine package gets unit tests with seeded
   RNG. Every harness tool must be exercised by the mock session
   (`testdata/mocksession/`). `go test ./...` passes offline.
4. **Layering** (enforced by review): `engine/*` imports stdlib (+ tiny
   deps) only. `state`, `codex` → `engine`. `harness`, `tui` → everything
   below. Never import `tui`/`harness`/`cmd` from `engine`.
5. **Rulebook fidelity.** `Mythras-Imperative.md` is the source of truth;
   `docs/rules/digest.md` and `docs/rules/tables.md` are its
   implementation-ready mirror. If code and digest disagree, one of them is
   a bug — fix it and cite the rulebook section in the commit.

## Project commands

```sh
just build      # static binary → bin/myul
just test       # go test ./... (offline, fakes only)
just lint       # golangci-lint run ./...
just run        # build + run TUI
```

## Module layout

See `docs/ARCHITECTURE.md` §2. Quick orientation:

- `internal/engine/resolve` — the single mutation entry point (`Move`s).
- `internal/harness/tools` — GM tool registry; add tools here, schema + handler.
- `internal/state` — persistence (`Store` interface, JSON store).
- `internal/codex` — embedded rulebook + lookup.
- `internal/tui` — Bubbletea models; one concern per file.

## Style

- Go 1.27, standard library first. Follow `golangci.yml` (errcheck, govet,
  staticcheck, gocritic, errorlint, ...). Run the linter before finishing.
- Tables for game data: numeric rule tables live as Go data in `engine/rules`
  or `engine/combat`, transcribed from `docs/rules/tables.md`, with a comment
  citing the rulebook heading.
- Naming: interfaces on the consumer side; `Result`/`Audit` types for engine
  outputs; no global mutable state (pass `*State` explicitly).
- Errors: sentinel errors in `engine` (e.g. `ErrNotEnoughAP`) so harness tools
  can translate them into structured `IMPOSSIBLE:` results for the GM.

## Testing conventions

- Table-driven tests; fixed `dice.NewRand(seed)`; assert on `Result.Facts` and
  audit, not just error/success.
- Golden files under `testdata/` — regenerate with `go test ./... -update`.
- Mock session: `harness.ScriptedClient` replays canned GM turns
  (`testdata/mocksession/script.json`); assert final `state.json` snapshot.

## Commits

- Conventional, scoped: `engine: ...`, `harness: ...`, `tui: ...`,
  `docs: ...`, `state: ...`. Small, reviewable commits on `master`.

## House style for the game itself

- The GM prompt favours long-form narrative, intrigue, and character-driven
  tension over combat sprints. When touching `harness/prompts`, preserve that
  voice guidance.
- Player-facing text: British-neutral English, second person, present tense.

## Skills

Project-specific workflow skills live in `.opencode/skills/`:

- `engine-move` — adding a new deterministic engine move end-to-end.
- `harness-tool` — exposing a new tool to the GM (schema + handler + tests).
- `tui-component` — building/altering Bubbletea UI pieces.
- `rules-codex` — updating rule data from the rulebook.

Use them when the task matches; they encode the recipes and checklists that
keep the codebase consistent as it scales.

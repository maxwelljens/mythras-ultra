# Mythras Ultra (`myul`)

Mythras Ultra is a terminal game where an LLM plays the game master of a
Mythras Imperative (d100) campaign. The GM's mechanical outcomes are never
hallucinated: dice, combat, inventory, and world state change only through a
deterministic Go engine the GM must call via tools. Play happens in a rich
Bubbletea TUI — chat, animated dice, character HUD, and session persistence.

## Features

- Deterministic engine (`internal/engine`) — seeded dice, skill and opposed
  resolution, AP combat, wounds, inventory, world flags and clocks; every
  operation returns facts, a dice audit, and a rulebook citation.
- Tool-driven GM harness (`internal/harness`) — the LLM narrates from
  structured tool results; sentinel errors come back as `IMPOSSIBLE:`
  refusals it must respect; optional local decision model with graceful
  fallback.
- Embedded rulebook codex (`internal/codex`) — `Mythras-Imperative.md` plus
  an implementation digest, searchable by the GM (`rules_lookup`).
- Bubbletea TUI (`internal/tui`) — Markdown chat with IC/OOC channels,
  dice theatre with accessibility toggle, metadata panel, 16-colour themes
  (`mythras`, `obsidian`, `parchment`, `terminal`), adaptive light/dark.
- Session persistence (`internal/state`) — schema-versioned `state.json`,
  append-only `log.jsonl`, regenerable transcript; sessions are replayable
  from the persisted seed.
- Offline test suite — seeded unit tests plus a scripted mock session
  (`testdata/mocksession/`) that exercises every registered tool.

## Building

```sh
just build                              # static binary → bin/myul
go build ./cmd/myul                    # plain Go, produces ./myul
```

Tests and lint:

```sh
just test      # go test ./... (offline, fakes only)
just lint      # golangci-lint run ./...
```

Requires Go 1.27. A `just` recipe exists for each of the commands above.

## Status

Status: early development. The architecture is settled but large parts of
the engine, harness, and TUI are still being built out; APIs may change.

## Documentation

- [`AGENTS.md`](AGENTS.md) — development guidelines, golden rules, testing
  and commit conventions.
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — the design contract;
  read it before touching code.
- Project skills (`.opencode/skills/`): `engine-move`, `harness-tool`,
  `tui-component`, `rules-codex`.

## Licence

European Union Public Licence 1.2 (EUPL-1.2) — see [`LICENSE`](LICENSE).

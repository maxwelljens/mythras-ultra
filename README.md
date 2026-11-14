# Mythras Ultra (`myul`)

Mythras Ultra is a terminal game where an LLM plays the game master of a
Mythras Imperative (d100) campaign. The GM's mechanical outcomes are never
hallucinated: dice, combat, inventory, and world state change only through a
deterministic Go engine the GM must call via tools. Play happens in a rich
Bubbletea TUI — chat, animated dice, character HUD, and session persistence.

## Features

- LLM game master — provider adapters over charmbracelet `fantasy`
  (tool-calling and streamed replies); the GM narrates from structured tool
  results and must respect `IMPOSSIBLE:` refusals.
- Deterministic Mythras Imperative engine (`internal/engine`) — seeded dice,
  skill and opposed resolution, AP combat, wounds, inventory, world flags and
  clocks; every operation returns facts, a dice audit, and a rulebook citation.
- 37 GM tools (`internal/harness/tools`) — the full catalogue is in
  `docs/design/harness-surface.md`, pinned by registry tests and exercised by
  the mock session.
- Optional decision model (`internal/harness/decide`) — tev1/Ollama answers
  "does this need a check, and which skill?" via the `decide` tool; play
  continues without it.
- Embedded rulebook codex (`internal/codex`) — `Mythras-Imperative.md`,
  searchable by the GM (`rules_lookup`, `rules_section`).
- Bubbletea v2 TUI (`internal/tui`) — Markdown chat with an IC/OOC switch,
  `$VISUAL` editor for long messages, dice theatre with accessibility toggle,
  right-hand metadata panel, and four 16-colour themes (`mythras`, `obsidian`,
  `parchment`, `terminal`) with adaptive light/dark.
- Session persistence (`internal/state`, under `~/.local/share/myul`) —
  atomic `state.json` with rotating snapshots, append-only `log.jsonl`,
  regenerable `transcript.md`; sessions resume from the persisted seed.
- Offline test suite — seeded unit tests plus scripted mock sessions
  (`internal/harness/testdata/mocksession/`) that exercise every registered
  tool.

## Commands

| Command | Description |
|---|---|
| `myul` | Session picker (or resume the newest session; `--no-tui` headless). |
| `myul new [name]` | Create a session and play it (`--seed`, `--model`, `--provider`, `--skip-llm`, `--no-tui`). |
| `myul resume [session-id]` | Resume a saved session; picker when the id is omitted. |
| `myul sessions` | List saved sessions, newest first (`--json`). |
| `myul delete <session-id>` | Delete a session and its log (`--force` skips confirmation). |
| `myul export <session-id>` | Write the Markdown play log (`--out FILE`, `--state` for state JSON). |
| `myul config path` | Print the resolved configuration file path. |
| `myul config init` | Write a commented default config file (`--force` overwrites). |
| `myul doctor` | Validate the config and probe the LLM API key and decision model. |
| `myul version` | Print version and build date. |

`--config PATH` is accepted by every command.

## Building

```sh
just build            # static binary → bin/myul
go build ./cmd/myul   # plain Go build → ./myul
```

Tests and lint:

```sh
just test      # go test ./... (offline, fakes only)
just lint      # golangci-lint run ./...
```

Requires Go 1.27. A `just` recipe exists for each of the commands above.

## Status

Status: early development. The engine, harness, session loop, TUI and CLI all
work and are covered by the offline test suite; APIs may still change. Play
needs a configured `[llm]` provider — start with `myul config init`.

## Documentation

- [`AGENTS.md`](AGENTS.md) — development guidelines, golden rules, testing
  and commit conventions.
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — the design contract;
  read it before touching code.
- [`docs/design/harness-surface.md`](docs/design/harness-surface.md) — the
  37-tool catalogue; [`docs/design/session-loop.md`](docs/design/session-loop.md)
  — how a turn runs.
- Project skills (`.opencode/skills/`): `engine-move`, `harness-tool`,
  `tui-component`, `rules-codex`.

## Licence

European Union Public Licence 1.2 (EUPL-1.2) — see [`LICENSE`](LICENSE).

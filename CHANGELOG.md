# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-11-17

### Added

- Deterministic Mythras Imperative engine: skill tests with grades,
  opposed/differential tests and difficulty modifiers; combat with an AP
  economy, atomic attacks and special effects; all state changes flow
  through typed moves with a persisted, seeded RNG stream.
- LLM game-master harness: 37 GM tools with JSON schemas and structured
  `IMPOSSIBLE:` results, exercised by offline mock sessions; embedded
  provider catalog with environment-key resolution; long-form narrative
  GM prompt with a golden player-agency rule.
- Bubbletea TUI: IC/OOC channels with run-based authorship tags, dice
  theatre animation, ctrl+p command palette with keybind hints, designed
  character sheet (banner, characteristic cards, wound table, two-column
  skills, inventory), metadata panel, help overlay and 16-colour themes.
- Paragraph streaming: GM replies render as they arrive via in-place
  continuation batches, while the chat log records exactly one line per
  completed paragraph.
- Decision-model advisories (local Ollama): an IC/OOC channel check
  before each send (N/y confirm, draft returned to the input on N) and a
  golden-rule check of GM replies with a player-bounded rewrite loop
  (Y/n confirm); both gated at confidence ≥ 0.7 and silently degrade when
  the model is unavailable.
- Status line above the input showing the active background worker:
  `Thinking...` for model calls, `Deciding...` for decision-model checks.
- Persistence: `state.json`, append-only `log.jsonl`, `chat.json` with
  resume replay (dice always, tool annotations opt-in via
  `[ui] replay_tools`), atomic `meta.json` and Markdown transcripts;
  rewritten replies tombstone their drafts and carry a single muted note.
- Context usage in the footer (`42.3K (18%)`), resolved from the embedded
  catalog or `[llm] context_window`; tool-loop cap via
  `[game] max_tool_rounds` (default 200).
- CLI: session picker, `new`, `resume`, `sessions`, `delete`, `export`,
  `config`, `doctor` (LLM endpoint probe) and `version`; optional LLM
  setup; TOML configuration with a commented template and `[prompts]`
  overrides.
- Offline test suite (1205 tests across 17 packages), golangci-lint clean,
  static cross-platform binaries via GoReleaser.

[Unreleased]: https://github.com/maxwelljens/mythras-ultra/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/maxwelljens/mythras-ultra/releases/tag/v0.1.0

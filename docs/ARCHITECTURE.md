# Mythras Ultra — Architecture

This document is the design contract for all work on `myul`. Read it before
touching code. Keep it current: if implementation diverges, either fix the code
or amend this document in the same commit.

## 1. What Mythras Ultra is

A production-grade Go harness + engine for playing Mythras Imperative (d100
TTRPG) in the terminal with an LLM game master. Two halves:

1. **TUI** — a rich Bubbletea terminal interface for play (chat, dice, status).
2. **Engine + Harness** — a deterministic game engine the GM must call through
   tools. Mechanical outcomes are never hallucinated: dice, combat, inventory,
   and world state only change through engine operations.

## 2. Repository layout

```
cmd/myul/            main entry point (cobra root, subcommands)
internal/
  config/            Viper/TOML configuration, XDG paths, theme selection
  engine/
    dice/            dice expressions, seeded RNG, roll audit records
    char/            Character model (PC/NPC): stats, skills, inventory, wounds
    rules/           derived attributes, skill resolution, grades, advancement
    combat/          action points, opposed resolution, damage, hit locations
    world/           world state: cultures, factions, flags, clocks, economy
    resolve/         high-level deterministic operations ("moves") with audit
  state/             persistence: Store interface, JSON session store, log
  codex/             embedded rulebook, search/lookup for the GM
  harness/
    tools/           tool registry + every GM-facing tool (JSON-schema typed)
    llm/             LLM Client interface, fantasy adapter, scripted mock
    prompts/         GM prompt assembly (embedded defaults + config overrides)
    decide/          tev1/Ollama decision-model client (optional, fallback)
    testdata/        mock-session fixtures (script.json, script_ooc.json)
  session/           game loop: turns, tool dispatch, compaction; implements tui.Driver
  tui/               Bubbletea UI: chat, dice theatre, HUD, metadata, themes
docs/                architecture, rules digest, research notes
```

Layering: `session` is the composition root — it may import `tui`,
`harness`, `state`, `config` and the engine. Nothing else may import `tui`,
`harness`, `session`, or `cmd`; `tui` and `harness` stay independent of each
other (the `tui.Driver` contract is consumer-side), and `engine` imports only
stdlib + tiny deps.

## 3. Core principles

### 3.1 Determinism first

- All randomness flows through `engine/dice` with an explicit, seedable
  `*Rand`. The session seed is persisted; a session is replayable.
- Every engine operation returns a `Result` containing: human summary, machine
  facts (before/after deltas), dice audit entries, and the rule reference
  (e.g. "Mythras Imperative, Combat: How Combat Works") where applicable.
- State mutates only inside `engine` operations invoked via `resolve.Move`s.
  The GM's free text never mutates state; only tool calls do.

### 3.2 The GM is tool-driven

The LLM GM receives a system prompt plus a toolbox. Tools are the *only* way
mechanics happen. Tool handlers are pure engine calls; they return structured
`facts` the GM narrates. If the GM forgets a check, the harness nudges it (the
prompt's tool policy plus `decide`); the UI annotates every tool call so the
player can audit the game.

### 3.3 Graceful degradation

- Decision model (`tev1:4b` via Ollama) is optional. If unreachable, harness
  falls back to LLM judgment for adjudication; engine mechanics never depend
  on it.
- LLM provider via Charm's `fantasy`; the `llm.Client` interface isolates the
  adapter so tests use a scripted mock and providers can be swapped.

## 4. Engine design

### 4.1 Character (`engine/char`)

`Character` covers PCs and NPCs identically:

- Identity: name, culture, career, age, concept/notes, tags (PC/NPC/ally...).
- Characteristics: STR CON SIZ DEX INT POW CHA (ints), plus derived attributes
  (HP total & per hit location, damage modifier, power points, etc.) computed
  by `char.Derive`/`Refresh` — never stored stale; the stored derived
  snapshot is only a cache.
- Skills: map skill → base and current value (`char.BaseSkill` formulas);
  experience flags (fumbled, advanced).
- Passions, combat styles.
- Inventory: `Item{ID, Name, Qty, Tags, Equipped, Slot, Weapon, Armor, Shield,
  Notes}`. Weapons & armor carry game stats (damage, reach, AP cost, armor
  points).
- Conditions & wounds: per-hit-location HP, wound severity, prone/stunned/etc.
- Equipment is a carried-item list; money is narrative only (digest §8: the
  rulebook has no currency or price list).

### 4.2 Resolution (`engine/rules`)

- `SkillTest(rand, char, skill, grade) → Outcome` (d100 roll-under with
  Mythras success grades; exact grade names & thresholds from the digest).
- `Opposed(a, b)` / `Differential(a, b)` — combat and social opposed rolls.
- `ExperienceRoll` / `ApplyFumbleIncrease` — experience/advancement per
  digest (`resolve.SkillAdvance` sequences them).
- Difficulty = grade + situational modifiers; the facts state the applied
  difficulty and modified chance ("45% base → 65% under very easy
  difficulty"), and the audit carries the roll and the rulebook citation.

### 4.3 Combat (`engine/combat`)

- Turn/round tracker: encounter state machine (inactive → encounter → rounds).
- AP economy: refresh per round; every action costs AP; tools reject
  impossible actions ("not enough AP", "prone", "unconscious").
- `ResolveAttack` is one atomic operation: attack roll → parry/dodge reaction
  (the tool's `reaction` argument: the GM asks the player for PCs, picks for
  NPCs; empty means none) → hit location roll → damage roll (with damage
  modifier & weapon table) → armor → wound severity & effects. Returns full
  audit.
- Ranged: range bands, ammunition consumption (arrows in quiver decrement).
- Healing/recovery operations.

### 4.4 World (`engine/world`)

- Cultures (what a character/NPC is; dialogue/behaviour hints for GM),
  factions & reputations, locations, world flags (e.g. "metal scarce:
  war economy") consulted by availability tools (`can_acquire` checks rarity,
  economy flags), clocks (tension tracks that advance over sessions), and an
  in-game calendar.

### 4.5 Moves (`engine/resolve`)

A `Move` is a typed, auditable game operation:
`RollDice`, `SkillTest`, `OpposedTest`, `LuckPoint`, `CreateCharacter`,
`FinalizeCharacter`, `UpdateCharacter`, `SkillAdvance`, `StartEncounter`,
`SpendAP`, `NextRound`, `ResolveAttack`, `ApplySpecialEffect`, `ApplyDamage`,
`Heal`, `CombatEnd`, `InventoryAdd`, `InventoryRemove`, `Equip`,
`SetWorldFlag`, `AddClock`, `AdvanceClock`, `AdvanceTime`, `SetScene`,
`Note`, `EndRecap`, ...
Each move: validate against state → execute → return `Result` with audit →
appended to the session log. Moves are the engine API *and* the mock-session
test surface.

## 5. Persistence (`state`)

- Sessions live in `~/.local/share/myul/sessions/<session-id>/`:
  - `state.json` — complete game state (schema-versioned), written atomically
    (temp file + rename) after each accepted move; keeps last N snapshots.
  - `log.jsonl` — append-only audit log: every move, roll, tool call, message.
  - `transcript.md` — rendered play log (regenerable from log.jsonl).
- `Store` interface: `Save/Load/List/Delete` plus `Record`/`ReadLog` for
  `log.jsonl`; JSON impl now, bbolt can slot in later without touching
  callers. Autosave hook after each move.
- Everything needed to resume a game must be in `state.json` — no hidden state.

## 6. Harness

### 6.1 Tools

`Tool{Name, Description, Schema, Handler}`. Registry exposes schemas to
the LLM and dispatches calls.

Tool results: `{ ok, summary, facts[], audit }`. Errors are structured
(`IMPOSSIBLE: ...` with reason) so the GM can narrate a refusal. The complete
37-tool catalogue (names, args, backing moves) lives in
`docs/design/harness-surface.md` and is enforced by the registry tests and
the mock-session coverage rule; families in brief:

- Rules & advice: `rules_lookup`, `rules_section`, `decide` (advisory).
- Dice & tests: `roll_dice`, `skill_test`, `opposed_test`, `luck_point`.
- Characters: `create_character` (PC or NPC via `pc` flag), `finalize_character`,
  `get_character`, `update_character`, `list_characters`, `skill_advance`.
- Inventory: `inventory_add`, `inventory_remove`, `equip`, `check_encumbrance`
  (advisory — the rulebook has no encumbrance rules).
- Combat: `combat_start`, `combat_status`, `spend_ap`, `can_act`, `next_round`,
  `resolve_attack`, `apply_special_effect`, `apply_damage`, `heal`, `combat_end`.
- World & continuity: `world_get`, `world_set_flag`, `can_acquire`,
  `add_clock`, `advance_clock`, `advance_time`, `set_scene`, `note`,
  `session_recap`, `end_session_recap`.

### 6.2 LLM layer (`harness/llm`)

- `Client` interface: `Chat(ctx, ChatRequest{Messages, Tools}) → ChatResponse`
  and `Stream(ctx, req, emit)` (incremental text deltas + tool calls).
  `fantasy` adapter is the production impl; `ScriptedClient` replays canned
  turns for the mock session.
- Conversation policy: system prompt (persona, voice, mechanics discipline,
  tool policy, session flow, plus the recap and character/world summaries),
  rolling compaction (summarised older turns + recent window, implemented in
  `internal/session`) so long games fit.

### 6.3 Decision model (`harness/decide`)

- tev1 via Ollama's dedicated **`POST /v1/systemone`** endpoint (per the
  model card; `/api/chat` is unsuitable — see docs/research/llm-stack.md §3).
  Default: localhost:11434, model `tev1:4b`; configurable.
- Used for cheap adjudication: "does this action need a skill check? which
  skill and grade?" — returns structured verdict JSON; cached; fully optional.
- Every decision-model verdict is logged with its confidence and can be
  overridden by GM or player.

### 6.4 Prompts (`harness/prompts`)

- Embedded default system prompt (persona: long-form, intrigue-first GM;
  never rolls for the player without tools; never invents mechanical outcomes).
- `[prompts] system` names a template on disk that fully replaces the
  embedded default (same placeholders) and takes precedence when set.

## 7. TUI (`internal/tui`)

Bubbletea (see research notes for pinned major version), Lipgloss, Bubbles.

- **Chat view** (primary): rendered Markdown (glamour v2 renderer — glow is
  CLI-only, see docs/research/llm-stack.md §4), scrolling viewport, streaming
  GM text, IC and OOC channels with a segmented switch; player input line
  (with history); `$VISUAL` editor via `tea.ExecProcess` for long messages.
- **Dice theatre**: rolls animate (number cycling, per-die snap, result
  hold via `tea.Tick`) with dramatic styling; result banners (Critical /
  Success / Failure / Fumble). Toggleable for accessibility
  (`ui.dice_animation`).
- **Event annotations**: every tool call renders as a compact engine strip
  (⚙ name → outcome) distinct from narrative text.
- **Metadata panel**: right-hand structured panel (HP per location, AP,
  inventory, scene, clocks) — off by default, toggled with a key.
- **Header**: session name, segmented IC/OOC switch, model name, plus
  `save –` / `decision –` indicator slots.
- **Palette**: exactly 16 colours; built-in themes (`mythras`, `obsidian`,
  `parchment`) plus `terminal` theme honouring user's ANSI 16. Adaptive to
  colour profile (light/dark).
- Keybindings: `ctrl+t` (IC/OOC), `ctrl+e` ($VISUAL editor), `ctrl+m`
  (metadata panel), `ctrl+l` (clear transcript), `enter`/`esc`/`q`,
  `pgup`/`pgdn`; slash commands (`/roll`, `/save`, `/meta`, `/theme`,
  `/editor`, `/recap`, `/help`, ...).

## 8. CLI (`cmd/myul`, cobra)

- `myul` (default: resume last session or start new — interactive picker),
  `myul new [name]`, `myul resume [session-id]`, `myul sessions`, `myul delete`,
  `myul config` (init/show path), `myul doctor` (check Ollama/tev1/provider),
  `myul version` (build date via ldflags), `myul export <session>`.

## 9. Configuration (Viper, TOML)

`~/.config/myul/config.toml` (XDG); `--config` override; search order:
flag > env (`MYUL_*`) > config file > defaults.

Key sections: `[llm]` (provider, model, api_key_env, base_url), `[decision]`
(enabled, url, model), `[game]` (seed, autosave, house rules toggles),
`[ui]` (theme, dice_animation, metadata_default, colour mode),
`[prompts]` (paths overriding embedded prompts).

## 10. Testing strategy

- Unit tests per engine package with fixed seeds; table-driven.
- Pinned tests for codex search rankings (`TestSearchGolden`) and prompt
  assembly — inline expectation tables, no golden files.
- **Mock session** (`internal/harness/mocksession_test.go` +
  `testdata/mocksession/script.json`, fixture format `seed`/`turns`/`expect`):
  `llm.ScriptedClient` replays canned GM turns through
  `tools.Registry.Dispatch` over a `resolve.State` — character creation,
  exploration, skill tests, combat encounter, inventory and world-flag
  changes, advancement — asserting the resulting final state. Every
  registered tool must appear at least once in a script's
  `expect.tools_used` (coverage check walks the registry).
- TUI model unit tests: Update/View transitions without a real terminal.
- `go test ./...` must pass with no network and no Ollama (fakes only).

## 11. Build & release

- Static binary `myul`: `CGO_ENABLED=0`, `-s -w`, version/date ldflags.
- goreleaser (`.goreleaser.yaml`), golangci-lint (`.golangci.yml`), `just`
  recipes for build/test/lint.
- Licence: EUPL-1.2 (`LICENSE`, SPDX headers in source).

## 12. Docs & agent guidance

- `AGENTS.md` — concrete development guidelines for humans and coding agents.
- `.opencode/skills/` — project skills: `engine-move`, `harness-tool`,
  `tui-component`, `rules-codex` (recipes for the common extension tasks).
- `docs/rules/` — rulebook digest + machine tables (derived from
  `Mythras-Imperative.md`; the PDF/MD are the source of truth).

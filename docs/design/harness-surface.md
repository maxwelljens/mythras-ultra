# Harness Tool Surface — Design Note

Contract for `internal/harness/tools` implementers and the mock session.
`docs/ARCHITECTURE.md` §6.1 lists the tool *families*; this file is the
authoritative catalogue: names, arguments, engine moves behind them, and the
facts each result must carry. When adding a tool: update this file first.

## Tool contract (recap of .opencode/skills/harness-tool)

```go
type Tool struct {
    Name        string
    Description string          // shown to the GM; say WHEN to use it
    Schema      map[string]any  // JSON Schema for arguments
    Handler     func(ctx context.Context, s *resolve.State, args json.RawMessage) (Result, error)
}
type Result struct {
    OK      bool            `json:"ok"`
    Summary string          `json:"summary"`            // one-line narratable fact
    Facts   []string        `json:"facts"`              // GM narrates these, never substitutes
    Audit   *resolve.Audit  `json:"audit,omitempty"`    // dice trail + rule citation
}
```

Errors from engine sentinels are converted to `Result{OK:false}` with
`Summary: "IMPOSSIBLE: <reason>"` (never a Go error to the model — the GM must
narrate refusal/complication). Unusable arguments (bad JSON, a missing
required field, an unknown field) become
`"IMPOSSIBLE: invalid arguments for <tool>: <detail>"` for the same reason.
Go errors are reserved for harness faults (unknown tool name, nil state, a
broken codex corpus).

## Conventions

- All tools are deterministic given `State` + session RNG. No clock reads, no
  network (except `decide`, which is advisory and never mutates).
- Naming: `<noun>_<verb>` (snake_case). Same noun groups share arg shapes:
  every character-targeting tool takes `{"actor": "<id>"}` where relevant;
  NPCs and PCs share the `characters` table (PCs flagged `pc: true`).
- IDs: engine-generated (`pc-1`, `npc-1`, `item-1`...). The GM passes IDs from
  earlier results; if it guesses an unknown ID the tool returns
  `IMPOSSIBLE: unknown character "..." (known: ...)` listing real IDs.
- Requiredness follows the engine: scalar args without `?` are required,
  except `weapon` (`""` means unarmed). Array args are optional unless the
  call is meaningless without them — only `options` and `participants` are
  required, and `reason` on world tools is recommended but optional.
- Every mutating tool's `Result.Facts` must state the *after* values
  ("arrows in quiver: 7 → 6") so the GM can narrate accurately.

## Catalogue

### Rules & advice
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `rules_lookup` | `query`, `limit?` | codex.Search | top sections + snippets |
| `rules_section` | `doc`, `heading` | codex.Sections | full section text |
| `decide` | `question`, `options[]` | decide.Client (advisory) | verdict + confidence + `advisory: true` |

### Dice & tests
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `roll_dice` | `expr`, `reason` | dice.Roll | per-die values, total |
| `skill_test` | `actor`, `skill`, `grade?`, `modifiers[]` | rules.SkillTestCharacter | roll vs effective %, outcome grade |
| `opposed_test` | `actor_a`, `skill_a`, `actor_b`, `skill_b`, `modifiers[]` | resolve.OpposedTest | both rolls, winner, margin |
| `luck_point` | `actor`, `purpose` | resolve.LuckPoint (rules.SpendLuck) | spend/burn record |

### Characters & NPCs
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `create_character` | `name`, `pc`, `culture`, `career`, `age?`, `characteristics?{...}`, `skills?{}` | resolve.CreateCharacter (char.Generate → char.Refresh) | full derived sheet |
| `finalize_character` | `actor`, `equipment[]`, `passions[]` | resolve.FinalizeCharacter | HP/PP/DM totals, carried-item count (advisory) |
| `get_character` | `actor` | — (read) | full sheet, conditions, inventory |
| `update_character` | `actor`, `changes{}` (concept, appearance, notes) | resolve.UpdateCharacter | diff |
| `list_characters` | `filter?` | — (read) | ids, names, tags, status line |
| `skill_advance` | `actor`, `skill`, `free?` | resolve.SkillAdvance (rules.ExperienceRoll / ApplyFumbleIncrease) | old → new % |

### Inventory & equipment
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `inventory_add` | `actor`, `item`, `qty?`, `tags[]` | resolve.InventoryAdd | carry total |
| `inventory_remove` | `actor`, `item`, `qty?` | resolve.InventoryRemove | carry total |
| `equip` | `actor`, `item`, `slot?` | resolve.Equip | what is now wielded/worn |
| `check_encumbrance` | `actor` | resolve.CheckCarrying | ADVISORY only — Mythras Imperative has no encumbrance rules (docs/rules/digest.md §2, Step 9: Equipment); returns carried bulk as GM judgement input |

### Combat
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `combat_start` | `participants[]` | combat.Start | initiative/AP order, round 1 |
| `combat_status` | — | — (read) | per-combatant AP/HP/conditions |
| `spend_ap` | `actor`, `action` | combat.SpendAP | AP left; IMPOSSIBLE if insufficient/prone |
| `can_act` | `actor`, `action` | combat.CanAct | advisory boolean + reasons |
| `next_round` | — | combat.Encounter.NextRound | new round, Action Points refreshed for every combatant, conditions tick |
| `resolve_attack` | `attacker`, `weapon?`, `defender`, `reaction? ("parry"|"dodge"|"none")` | combat.ResolveAttack | attack roll, reaction roll, hit location, damage, armour, wound severity + effects, `available_special_effects` |
| `apply_special_effect` | `effect`, `attacker?`, `defender?`, `target?`, `location?` | combat.ApplySpecialEffect | applied effect mechanics (impale, knockback...); GM-adjudicated effects return `adjudicated: true` + narration facts |
| `apply_damage` | `actor`, `location?, amount` | combat.ApplyDamage | new location HP, wound severity, effects |
| `heal` | `actor`, `method` | combat.Heal | HP restored over time |
| `combat_end` | — | combat.End | summary of encounter |

### World & continuity
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `world_get` | `key?` | — (read) | world summary / specific flag |
| `world_set_flag` | `key`, `value`, `reason?` | world.SetFlag | old → new |
| `can_acquire` | `item`, `rarity`, `region?` | world.CanAcquire | yes/no + governing flags (economy, war...) |
| `add_clock` | `name`, `segments`, `on_complete?` | resolve.AddClock | new clock: name, capacity, completion hooks — refuses a duplicate name |
| `advance_clock` | `clock`, `segments`, `reason?` | world.AdvanceClock | clock state, triggered events — the clock must exist (create it with `add_clock` first) |
| `advance_time` | `days?, hours?` | resolve.AdvanceTime | date before → after, season/calendar events |
| `set_scene` | `location`, `description?`, `present[]` | resolve.SetScene | scene header |
| `note` | `topic`, `text`, `tags[]` | resolve.Note | note id |
| `session_recap` | — | — (read) | recap for context rebuild |
| `end_session_recap` | `summary` | resolve.EndRecap | stored recap |

## Mock session (`internal/harness/testdata/mocksession/`)

```json
{
  "seed": 42,
  "turns": [
    { "response": { "message": { "role": "assistant", "content": "...",
      "tool_calls": [ { "id": "t1", "name": "create_character", "args": { ... } } ] } } }
  ],
  "expect": { "tools_used": ["..."], "impossible": ["..."] }
}
```

- `turns` is exactly `llm.ScriptedTurn`'s JSON: each entry is one canned
  `ChatResponse` (`response.message`), replayed by `llm.ScriptedClient`.
- `args` are literal and real for the linear arc: character, item and note
  IDs are the engine's own (`pc-1`, `npc-1`, `item-N`, `note-1`) in creation
  order — no placeholder rewriting.
- `expect.tools_used` must equal the tools the replay actually dispatched
  (asserted per script). `expect.impossible` lists the tools that must
  produce at least one structured `IMPOSSIBLE:` refusal — their successful
  calls are allowed (e.g. `spend_ap` twice with points, a third time
  without); every other dispatch must be OK.
- Coverage rule (enforced by a test): the union of `tools_used` across the
  scripts must equal the registry's tool set, so a new tool without a
  scripted call fails the build.
- `script.json` creates its own progress clock with `add_clock` before
  `advance_clock` fills it.
- `script.json` replays decide against a stubbed `/v1/systemone` server (all
  results OK); `script_ooc.json` uses a disabled decision client, so its
  `decide` call returns the unavailable IMPOSSIBLE result.

Sessions:
- `script.json` — full campaign arc: greeting → character creation
  (create/finalize/get/list/update) → exploration with rules lookups, skill
  tests and world flags → combat encounter (start, AP, rounds, attack,
  special effect, damage, healing, end) → inventory → advancement →
  decide → recaps. Exercises all 37 tools.
- `script_ooc.json` — OOC clarifications, impossible actions (exhausted
  Action Points, unknown character, missing argument), the decide fallback
  and a rules lookup in doubt.

## Event flow to the TUI

`internal/session` implements the TUI's `Driver` interface: each assistant
text delta → `EventNarrative`; each tool call → `EventToolAnnotation` (tool
name + Result.Summary, OK flag); each roll inside a Result.Audit →
`EventDice`; harness faults → `EventError`. The GM's narration must never
contradict `Result.Facts` — the prompt forbids it and the annotation strip
shows the player the authoritative facts.

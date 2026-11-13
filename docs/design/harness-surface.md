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
narrate refusal/complication). Go errors are reserved for harness faults.

## Conventions

- All tools are deterministic given `State` + session RNG. No clock reads, no
  network (except `decide_*` and `doctor`, which are advisory and never mutate).
- Naming: `<noun>_<verb>` (snake_case). Same noun groups share arg shapes:
  every character-targeting tool takes `{"actor": "<id>"}` where relevant;
  NPCs and PCs share the `characters` table (PCs flagged `pc: true`).
- IDs: engine-generated (`pc-1`, `npc-1`, `item-1`...). The GM passes IDs from
  earlier results; if it guesses an unknown ID the tool returns
  `IMPOSSIBLE: unknown character "..." (known: ...)` listing real IDs.
- Every mutating tool's `Result.Facts` must state the *after* values
  ("arrows in quiver: 7 → 6") so the GM can narrate accurately.

## Catalogue

### Rules & advice
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `rules_lookup` | `query`, `limit?` | codex.Search | top sections + snippets |
| `rules_section` | `doc`, `heading` | codex.Section | full section text |
| `decide` | `question`, `options[]` | decide.Client (advisory) | verdict + confidence + "advisory only" |

### Dice & tests
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `roll_dice` | `expr`, `reason` | dice.Roll | per-die values, total |
| `skill_test` | `actor`, `skill`, `grade?`, `modifiers[]` | rules.SkillTest | roll vs effective %, outcome grade |
| `opposed_test` | `actor_a`, `skill_a`, `actor_b`, `skill_b`, `modifiers[]` | rules.OpposedTest | both rolls, winner, margin |
| `luck_point` | `actor`, `purpose` | rules.LuckPoint | spend/burn record |

### Characters & NPCs
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `create_character` | `name`, `pc`, `culture`, `career`, `age?`, `characteristics?{...}`, `skills?{}` | char.New + rules.Derive | full derived sheet |
| `finalize_character` | `actor`, `equipment[]`, `passions[]` | resolve.FinalizeCharacter | HP/PP/DM totals, encumbrance |
| `get_character` | `actor` | — (read) | full sheet, conditions, inventory |
| `update_character` | `actor`, `changes{}` (notes, appearance, allegiances...) | resolve.UpdateCharacter | diff |
| `list_characters` | `filter?` | — (read) | ids, names, tags, status line |
| `skill_advance` | `actor`, `skill`, `xp` | rules.AdvanceSkill | old → new % |

### Inventory & equipment
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `inventory_add` | `actor`, `item`, `qty?`, `stats?` | char.Inventory | carry total |
| `inventory_remove` | `actor`, `item`, `qty?` | char.Inventory | carry total |
| `equip` | `actor`, `item`, `slot` | char.Equip | what is now wielded/worn |
| `check_encumbrance` | `actor` | char.Carrying | ADVISORY only — Mythras Imperative has no encumbrance rules (docs/rules/digest.md §9); returns carried bulk as GM judgement input |

### Combat
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `combat_start` | `participants[]` | combat.Start | initiative/AP order, round 1 |
| `combat_status` | — | — (read) | per-combatant AP/HP/conditions |
| `spend_ap` | `actor`, `action`, `cost` | combat.SpendAP | AP left; IMPOSSIBLE if insufficient/prone |
| `can_act` | `actor`, `action` | combat.CanAct | advisory boolean + reasons |
| `resolve_attack` | `attacker`, `weapon`, `defender`, `reaction? ("parry"|"dodge"|"none")` | combat.ResolveAttack | attack roll, reaction roll, hit location, damage, armour, wound severity + effects |
| `apply_damage` | `actor`, `location`, `amount`, `type?` | combat.ApplyDamage | new location HP, wound severity, effects |
| `heal` | `actor`, `method` | combat.Heal | HP restored over time |
| `combat_end` | — | combat.End | summary of encounter |

### World & continuity
| Tool | Args | Engine call | Key facts returned |
|---|---|---|---|
| `world_get` | `key?` | — (read) | world summary / specific flag |
| `world_set_flag` | `key`, `value`, `reason` | world.SetFlag | old → new |
| `can_acquire` | `item`, `rarity`, `region?` | world.Availability | yes/no + governing flags (economy, war...) |
| `advance_clock` | `clock`, `segments`, `reason` | world.AdvanceClock | clock state, triggered events |
| `advance_time` | `duration` | world.AdvanceTime | date/season, travel side effects |
| `set_scene` | `location`, `description?`, `present[]` | resolve.SetScene | scene header |
| `note` | `topic`, `text`, `tags[]` | resolve.Note | note id |
| `session_recap` | — | — (read) | recap for context rebuild |
| `end_session_recap` | `summary` | resolve.EndRecap | stored recap |

## Mock session (`testdata/mocksession/script.json`)

```json
{
  "seed": 42,
  "turns": [
    { "response": { "message": { "role": "assistant", "content": "...",
      "tool_calls": [ { "id": "t1", "name": "create_character", "args": { ... } } ] } } }
  ],
  "expect": { "state_file": "final_state.golden.json", "tools_used": ["..."] }
}
```

Coverage rule (enforced by a test): every registered tool name must appear in
`tools_used` of at least one mock session script. Sessions:
- `script.json` — full campaign arc: greeting → world-creation questions →
  character creation → exploration + skill tests → encounter with combat →
  inventory/world-flag changes → healing → advancement → recap.
- `script_ooc.json` — OOC clarifications, impossible actions, decide-tool use.

## Event flow to the TUI

`internal/session` implements the TUI's `Driver` interface: each assistant
text delta → `EventNarrative`; each tool call → `EventToolAnnotation` (tool
name + Result.Summary, OK flag); each roll inside a Result.Audit →
`EventDice`; harness faults → `EventError`. The GM's narration must never
contradict `Result.Facts` — the prompt forbids it and the annotation strip
shows the player the authoritative facts.

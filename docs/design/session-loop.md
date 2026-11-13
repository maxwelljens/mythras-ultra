# Session Loop — Design Note

How `internal/session` orchestrates a game turn. It is the only place where
the LLM, the harness tools, the engine and the TUI meet. Contract for the
session-orchestrator wave; keep in sync with `harness-surface.md`.

## Responsibilities

`session.Session` owns: `*resolve.State`, `state.Store`, `llm.Client`,
`tools.Registry`, `decide.Client` (optional), the conversation history, and
the event stream to the UI. It implements the TUI's `tui.Driver` interface
(consumer-side contract lives in `internal/tui/events.go`):

- `Send(ctx, channel, text)` — player input (IC or OOC).
- `Events() <-chan Event` — one stable channel, closed by `Close()`.
- `Metadata() tui.Metadata` — snapshot for the panel (from `resolve.State`).
- `Close()` — flush state + log, close channel.

## Turn loop

1. Player `Send(text)` on IC:
   a. Echo the player's message back as `EventNarrative` (the TUI renders it
      as "You" via exact-text echo matching — echo the text VERBATIM).
   b. Append to conversation as `RoleUser`. If a slash command (`/roll`,
      `/save`, `/recap`, `/meta`, ...) it is handled locally instead: `/roll`
      runs a `RollDice` move, `/save` saves, `/recap` returns the recap,
      `/meta` toggles the panel.
   c. Call `llm.Client.Chat` (or `Stream`) with: system prompt
      (prompts.Assemble: persona + recap + character/world summaries),
      conversation history, and the registry's `[]llm.ToolSpec`.
2. While the response contains tool calls:
   a. Dispatch each `ToolCall` to `tools.Registry` against the live
      `*resolve.State`; every result is `Record`ed in the session log.
   b. Emit `EventToolAnnotation` (tool name + Result.Summary, OK flag) and,
      for each `Audit.Rolls`, an `EventDice` (Expr/Dice/Modifier/Total/
      Outcome) so the dice theatre plays.
   c. Append `RoleTool` messages, re-call the LLM. Bound the loop (e.g. 32
      iterations) — on overflow, summarise and stop with an `EventError`.
3. Assistant text: stream as `EventNarrative` deltas; on completion append
   to history.
4. OOC channel: same loop but tag the message `[ooc]`; the prompt instructs
   the GM to reply out of character and perform no moves unless the player
   confirms.

## Determinism & audit

- Only tool dispatch mutates state (via `resolve.Apply`); the loop never
  edits `resolve.State` directly. After each accepted move: autosave
  (`config.game.autosave`) + `Store.Record`.
- `decide` questions (advisory) are logged like tool calls with
  `advisory: true`; the GM may be nudged to call the right tool when a
  player action looks mechanical but no test was requested (soft policy —
  log a note, never force).

## Context compaction

Long games exceed context. Policy: system prompt carries `Recaps` (from
`EndRecap` moves) + current character/world summaries; conversation history
is windowed to the last N messages (config), older turns replaced by a
rolling summary produced through the same `llm.Client` (or kept verbatim if
the model context is large enough — decide by tokens from `Usage`).

## Failure policy

- LLM error → `EventError` + history rollback (the player can retry; no
  partial tool state: moves are atomic).
- Tool error → structured `IMPOSSIBLE:` result goes back to the model; the
  GM narrates the refusal. Never abort the turn on a single tool error.
- Decide unavailable → continue (it is optional by design).

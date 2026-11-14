# Session Loop — Design Note

How `internal/session` orchestrates a game turn. It is the only place where
the LLM, the harness tools, the engine and the TUI meet. Contract for the
session-orchestrator wave; keep in sync with `harness-surface.md`.

## Responsibilities

`session.Session` owns: `*resolve.State`, `state.Store`, `llm.Client`,
`tools.Registry`, `decide.Client` (optional), the conversation history, and
the event stream to the UI. It implements the TUI's `tui.Driver` interface
(consumer-side contract lives in `internal/tui/events.go`):

- `Send(ctx, channel, text)` — player input (IC or OOC). Text is never
  parsed for commands: prose that looks like `/save` reaches the model as
  prose.
- `Command(name, args...)` — one named local command (`roll`, `save`,
  `recap`, `meta`), enqueued on the same single-runner goroutine as
  messages. Commands carry no player text: they produce no echo, never
  reach the model, and run on the session's current channel; an unknown
  name comes back as an `EventError` listing the known commands. The TUI
  drives it from the command palette (ctrl+p); view-only palette actions
  (character sheet, clear, help, editor, quit, channel switch, themes)
  stay inside the TUI and never cross this interface.
- `SetChannel(channel)` — remembers the player's IC/OOC mode: it stores the
  channel and writes it to `meta.json` immediately, so `Resume` reopens on
  the last-used mode (`Options.InitialChannel` reads it via
  `Session.LastChannel`). Called best-effort by the TUI on every switch
  (ctrl+t, the palette's channel entry) and by `Send` itself; a UI hint, so
  a failure is never surfaced, and a no-op after `Close()`.
- `Events() <-chan Event` — one stable channel, closed by `Close()`.
- `Metadata() tui.Metadata` — snapshot for the panel (from `resolve.State`).
- `Close()` — final save, write `transcript.md`, close the channel.

## Turn loop

1. Player `Send(text)` on IC:
   a. Echo the player's message back as `EventNarrative` (the TUI renders it
      as "You" via exact-text echo matching — echo the text VERBATIM).
   b. Append to conversation as `RoleUser` and run the model turn. There is
      no slash-command parsing anywhere: `/save`-looking prose goes to the
      model as prose. Commands arrive separately through `Command(name,
      args…)` (the palette, or the input line's `expr ▸` argument mode for
      `roll`): `roll` runs a `RollDice`-tool dispatch, `save` saves,
      `recap` returns the recaps, `meta` prints the panel snapshot (the
      panel itself toggles with `ctrl+m`, the sheet opens in the TUI).
   c. Call `llm.Client.Chat` (or `Stream`) with: system prompt
      (prompts.Assemble: persona + recap + character/world summaries),
      conversation history, and the registry's `[]llm.ToolSpec`. Each call
      is announced with `EventThinking` first; the turn ends with exactly
      one `EventTurnEnd` (success or rollback alike) — the TUI brackets a
      header throbber on them.
2. While the response contains tool calls:
   a. Dispatch each `ToolCall` to `tools.Registry` against the live
      `*resolve.State`; every result is `Record`ed in the session log.
   b. Emit `EventToolAnnotation` (tool name + Result.Summary, OK flag) and,
      for each `Audit.Rolls`, an `EventDice` (Expr/Dice/Modifier/Total/
      Outcome) so the dice theatre plays.
   c. Append `RoleTool` messages, re-call the LLM. Bound the loop
      (`Config.MaxToolRounds`, `config.game.max_tool_rounds`, default 200)
      — on overflow, drop the unanswered calls and stop with an `EventError`
      naming the configured cap.
3. Assistant text: stream as `EventNarrative` batches — at each `\n\n`
   boundary, eagerly once an unbroken run reaches `partialNarrativeMin`
   bytes, and for the remainder when the reply ends — each flagged
   `Continuation` when it extends the entry already on screen (the TUI
   appends it, so mid-paragraph text is no longer invisible while
   paragraphs stay whole entries). A leading `[ic]` / `[ooc]` marker the
   model wrote inline is stripped before emission (the TUI derives the
   channel from `Event.Channel` — model output only, never the player's
   echo). Display and recording are separate: the paragraph is recorded as
   ONE chat line when its entry closes — at the boundary or at the end of
   the reply, error path included — so replay keeps producing
   whole-paragraph entries. On completion append to history.
4. OOC channel: same loop but tag the message `[ooc]`; the prompt instructs
   the GM to reply out of character and perform no moves unless the player
   confirms.

## Chat persistence & replay

The conversation and the play text survive the process:

- **History (`chat.json`)** — the session marshals `[]llm.Message` and
  writes it atomically (`Store.SaveChat`) at every autosaved turn end, on
  the `save` command, and on `Close`. `Resume` unmarshals it into the history before
  the runner starts; missing or corrupt bytes continue with an empty
  history plus one system event (the prompt's recaps and summaries still
  carry the context).
- **Play lines (`log.jsonl`)** — player echoes (recorded verbatim), GM
  narrative (after `[ic]`/`[ooc]` marker sanitisation) and error text are
  recorded as `Kind: "chat"` entries (`state.ChatLine`: channel, player,
  kind, text). System confirmations, thinking, turn-end, tool
  annotations and dice are not chat lines: they are transient, and the
  moves behind them are already in the log. Recording happens inline on
  the runner goroutine, so chat and move entries interleave in true play
  order.
- **Replay** — `Resume` rebuilds the recorded play into events (original
  order and timestamps) and queues them; the runner delivers the queue
  before it processes any input. Chat lines replay as the events they were
  recorded from; every recorded `Audit.Rolls` replays as an `EventDice`,
  and with `Config.ReplayTools` (`ui.replay_tools`, default off) the tool
  annotation that preceded those dice replays in front of them. All of
  them carry `Event.Replay`, so the TUI renders a roll as its finished
  banner instead of performing the dice theatre again. Replay bypasses
  `emitChat`, so it never re-records — only freshly produced events touch
  the log. `transcript.md` renders the same interleaving: `**You:** `
  prefixed player lines, GM text verbatim, move annotations alongside.
- **Preferences (`meta.json`)** — the IC/OOC mode last in use, written
  atomically on every switch (`Store.SaveMeta`) and read at construction
  (`Store.LoadMeta`); absent or corrupt bytes degrade to the in-character
  channel rather than failing a resume.

## Determinism & audit

- Only tool dispatch mutates state (via `resolve.Apply`); the loop never
  edits `resolve.State` directly. After each accepted move: autosave
  (`config.game.autosave`) + `Store.Record`.
- `decide` questions (advisory) are logged like tool calls with
  `advisory: true`. Nothing forces a tool call: the prompt's tool policy asks
  the GM to test consequential actions, and the loop never insists (soft
  policy — the only enforced stop is the tool-round bound above).

## Context compaction

Long games exceed context. Policy: system prompt carries `Recaps` (from
`EndRecap` moves) + current character/world summaries; conversation history
is windowed to the last `Config.HistoryWindow` messages (default 32), the
older turns replaced by one summary produced through the same `llm.Client`.
If that summarisation call fails, everything is kept verbatim and a note
lands in the transcript — compaction is an optimisation, never a data loss.
The window counts messages, not tokens: `Usage` is advisory display data
only — the last call's input tokens feed the footer's context readout
(`Metadata.ContextUsage`; the window comes from `llm.context_window` or
the embedded catalog) and never drive compaction.

## Failure policy

- LLM error → `EventError` + history rollback (the player can retry; no
  partial tool state: moves are atomic).
- Tool error → structured `IMPOSSIBLE:` result goes back to the model; the
  GM narrates the refusal. Never abort the turn on a single tool error.
- Decide unavailable → continue (it is optional by design).

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
  `recap`, `meta`, plus a pending confirmation's `confirm` answer — the
  channel check's hold or the golden rule's regeneration), enqueued on
  the same single-runner goroutine as messages. Commands carry no player
  text: they produce no echo, never reach the model, and run on the
  session's current channel; an unknown
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

## Status line

The TUI renders `header, body, status line, input, footer` — the status
line is ALWAYS one row (empty when idle) so the layout never jumps, and
it sits outside the overlay switch, so sheet, palette, help and panel
views only replace the body and still show it. The header keeps its
static segments (session, IC/OOC switch, model, save –/decision –
placeholders); the throbber moved off it — the status line is the only
chrome progress element (the dice theatre stays in the transcript: it is
content, not chrome).

Its content is the throbber's spinner frame plus a pinned label,
left-aligned and clipped to the window (never wrapped):
`Thinking...` for `EventThinking` and the streaming-writing state (the
dot frames vs the cursor block still distinguish them), `Deciding...` for
`EventDecideStart`. The frame sets are the thinking dots, the writing
cursor block and the deciding quadrant orbit; the deciding style is the
theme's Highlight slot (Accent = thinking, Success = writing).

Two brackets drive it, both emitted from the runner goroutine in event
order:

- **Model turn** — `EventThinking` before each model call, one
  `EventTurnEnd` when the turn completes or aborts (error path included;
  `EventError` also clears the line). Thinking → writing on the first
  narrative delta, back to thinking on a tool annotation.
- **Decision-model call** — `EventDecideStart` immediately BEFORE each
  `s.decision.Decide` (the channel gate and the golden-rule check) and
  `EventDecideStop` immediately AFTER it returns, before any
  `EventConfirm`, echo or turn event the verdict leads to. The TUI moves
  from any state (idle included — the channel gate runs before a turn
  exists) to deciding on the start, to thinking when `EventThinking`
  arrives (the model wins), and to idle on the stop: a confirm wait that
  follows therefore shows an EMPTY line — the prompt in the input line is
  the signal that the player must act. During the off-screen golden
  rewrite `EventThinking` already fires, so the line reads `Thinking...`.
  Neither status kind carries transcript content; a pair is emitted only
  around a call that actually runs — a check that is off, or a missing
  decision client, never asks (an unreachable one does: the call ran and
  failed, and the line goes idle again).

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
      one `EventTurnEnd` (success or rollback alike) — the TUI brackets
      the status line on them (see "Status line" below).
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
   whole-paragraph entries. On completion the golden-rule check below
   gates the reply, then it enters history.
4. OOC channel: same loop but tag the message `[ooc]`; the prompt instructs
   the GM to reply out of character and perform no moves unless the player
   confirms.

## Channel check (confirm)

Every player `Send` is checked before its echo, on the runner goroutine and
synchronously — ahead of step 1a. With
`[decision] channel_check` on (default), the runner asks the decision model
`decide.Question{State: <mode sentence + the message>, Question: "Which
channel does this player message belong to?", Options: ["in-character",
"out-of-character"]}` — the current channel names the option that counts as
correct. The message is held only when all three hold: the verdict differs
from the channel, its confidence reaches `channelCheckMinConfidence` (0.7),
and the text is at least `channelCheckMinLen` (32) bytes; a decision model
that is disabled, unreachable or erroring passes silently, so the check can
never block a message or fail a turn. The `Decide` call itself is bracketed
with `EventDecideStart`/`EventDecideStop` (see "Status line"), so the
status line reads `Deciding...` for exactly its duration and is empty again
before the hold or the echo lands.

A held message is not echoed, not appended to history and not recorded: the
runner emits one `EventConfirm` (Markdown = the exact "N/y" question for the
current channel, `Event.ConfirmText` = the held text) and waits for
`Command("confirm", "y"|"n")`. **y** continues exactly as a Send that was
never held (echo → history → turn); **n** discards it without a trace.
Inputs arriving meanwhile keep queueing on the buffered channel as before,
`Close` drops a pending hold without hanging, and a confirm with no hold
behind it comes back as a "nothing pending" error line. The TUI answers from
the input line: while the confirm stands the prompt reads `N/y ▸ `, y/Y
sends "y", n/N/enter/esc sends "n" and returns the held text to the input
line unsent, every other key is ignored (ctrl+c still quits).

## Golden rule check (regen)

Every **in-character** assistant reply is checked after its tool calls have
dispatched (dice and annotation events already on screen) and before it
enters history, with `[decision] check_gm_output` on (default). The runner
asks the decision model
`decide.Question{State: <the player's latest message + the reply's message
sequence>, Question: "Does this Game Master reply decide what the player's
character does — choosing or narrating the character's actions the player
never chose? It complies if the player keeps all choices.", Options:
["complies","violates"]}`. The reply is held only when the verdict is
"violates" and its confidence reaches `channelCheckMinConfidence` (0.7);
an empty reply, an OOC turn, a disabled check, and a decision model that
is disabled, unreachable or erroring all pass silently, so the check can
never fail a turn. The reply's message sequence in the state keeps every
check a fresh question for decide's verdict cache — a rewrite is judged,
not the first verdict replayed. Every `Decide` call of the loop is
bracketed with `EventDecideStart`/`EventDecideStop` (see "Status line"),
so the line reads `Deciding...` while it runs — over a reply that is
already streaming as `Thinking...` — and goes empty before the regen
prompt or the turn end that follows.

A held reply files one `EventConfirmRegen` (Markdown = the exact "Y/n"
question, no held text — whatever stands on screen is what "n" keeps) and
waits for `Command("confirm", "y"|"n")` like the channel check's hold,
with the inverse default: **Enter regenerates here, Enter discards
there** (the input line shows "Y/n ▸ " over the hold's "N/y ▸ ", both
modes keyed from the same confirm command). **y** re-asks the model
OFF-SCREEN — `history + [assistant{original text, original toolCalls},
user{corrective nudge}]`, no tools advertised, narrative suppressed so
only the `EventThinking` bracket shows — then swaps the rewrite into the
transcript as ONE `EventNarrative` with `Replace` set: it carries the new
`MsgSeq`, the TUI removes every narrative entry of the replaced sequence
and files the full text at the first one's position. The replaced
message's recorded chat lines are tombstoned with an append-only `retract`
line, the rewrite is recorded as one chat line with its new MsgSeq, and
the rewrite is re-checked through the same gate — still violating means
the same prompt again (bounded only by the player), clean ends the loop.
The rewrite's own tool calls are stripped (the originals already ran), and
discarded drafts never enter history: the final message lands as
`assistant{final text, ORIGINAL toolCalls}` plus its tool results as
usual. **n** keeps the message as-is with no mark. A rewrite that ends up
standing leaves one muted `note` chat line (`↩ reply rewritten (golden
rule)`), replayed as a system event; transcript and replay are two-phase —
the retract tombstones hide exactly the lines of their MsgSeq, everything
else (legacy MsgSeq-0 lines included) renders as recorded. Each completed
check is audited as a `golden_rule_check` LogEntry
`{msg_seq, verdict, confidence, decision, attempt}` which the transcript
renderer skips: the display shows only the note.

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
  kind, text). System confirmations, thinking, turn-end, decision-model
  start/stop, tool annotations and dice are not chat lines: they are
  transient, and the
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

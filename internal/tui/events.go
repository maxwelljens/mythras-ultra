package tui

import (
	"context"
	"time"
)

// Channel identifies which conversation channel a message or event belongs to.
type Channel int

const (
	// ChannelIC is the in-character channel: the game itself.
	ChannelIC Channel = iota
	// ChannelOOC is the out-of-character channel: table talk.
	ChannelOOC
)

// String returns the short display label of the channel ("IC"/"OOC").
func (c Channel) String() string {
	switch c {
	case ChannelIC:
		return "IC"
	case ChannelOOC:
		return "OOC"
	default:
		return "?"
	}
}

// EventKind classifies what an event carries: it selects how the app
// renders a transcript entry, and the two status kinds (EventThinking,
// EventTurnEnd) drive the header throbber instead of the transcript.
type EventKind int

const (
	// EventNarrative is story text in Markdown, rendered with glamour.
	EventNarrative EventKind = iota
	// EventToolAnnotation is a compact engine tool-call strip.
	EventToolAnnotation
	// EventDice is a finished roll rendered as a banner.
	EventDice
	// EventSystem is an informational system line.
	EventSystem
	// EventError is a failure surfaced to the player.
	EventError
	// EventThinking reports that a model call is in flight — the harness
	// emits it before every llm.Chat/Stream, including re-invocations
	// after tool rounds. It carries no transcript content: the header
	// throbber shows the thinking state.
	EventThinking
	// EventTurnEnd reports that a turn finished, successfully or not. It
	// carries no transcript content: the header throbber returns to idle.
	EventTurnEnd
	// EventConfirm asks the player to answer a pending confirmation —
	// the channel check's "N/y" question about a held message. Markdown
	// is the question and ConfirmText the held text the answer decides
	// about; while it stands the input line is in confirm mode and the
	// runner waits for Command("confirm", "y"|"n").
	EventConfirm
)

// Annotation describes one engine tool call for the annotation strip.
type Annotation struct {
	// ToolName is the tool that ran (for example "resolve:advance-clock").
	ToolName string
	// Summary is a one-line outcome description.
	Summary string
	// OK reports whether the call succeeded.
	OK bool
}

// DiceEvent describes a finished roll. The TUI only renders it: the numbers
// come from the engine and are never recomputed here.
type DiceEvent struct {
	// Expr is the roll expression as written (for example "3d6+2").
	Expr string
	// Dice holds the individual die results, in roll order.
	Dice []int
	// Modifier is the flat modifier applied to the roll.
	Modifier int
	// Total is the final result.
	Total int
	// Outcome is the qualitative result: "critical", "success", "failure",
	// "fumble", or "" when the roll has no qualitative outcome.
	Outcome string
}

// NameValue is one labelled character-sheet row: a name ("STR", "Dodge")
// and its display-ready value, both formatted by the session that built it.
type NameValue struct {
	Name  string
	Value string
}

// SheetGroup is one titled section of sheet rows — a skill category
// ("Standard Skills", "Professional Skills") as the engine reports it.
type SheetGroup struct {
	// Name is the section title the overlay shows.
	Name string
	// Rows are the section's rows, in the shared rated order.
	Rows []NameValue
}

// InventoryItem is one carried item as the sheet's inventory table shows
// it: the name, how many are carried, whether it is worn or wielded and
// in which slot (empty for worn items).
type InventoryItem struct {
	Name     string
	Qty      int
	Equipped bool
	Slot     string
}

// Sheet is the full character sheet behind the sheet overlay: display-ready,
// consumer-side data assembled by the session from engine state (the TUI
// never derives numbers). Characteristics keep the fixed STR CON SIZ DEX INT
// POW CHA order; Derived lists HP, Damage Modifier, Power Points, Luck and
// Experience Bonus as the engine reports them; Styles and Passions are
// sorted by value descending, then name. Skills arrive either sectioned by
// category in SkillGroups, or flat in Skills (sorted the same way) when the
// source knows no categories. The zero value means "no character yet" and
// renders the overlay's empty state.
type Sheet struct {
	// Name, Culture, Career, Age and Concept identify the character.
	// Age is "" when the character records no age.
	Name, Culture, Career, Age, Concept string
	// Characteristics are the seven characteristics in fixed order.
	Characteristics []NameValue
	// Derived are the pooled derived readouts (HP, Damage Modifier,
	// Power Points, Luck, Experience Bonus).
	Derived []NameValue
	// HPCurrent and HPMax are the pooled hit points behind the Derived
	// HP row, for the overlay's bar (both zero when unknown).
	HPCurrent, HPMax int
	// Skills lists assigned skills in one flat section; SkillGroups
	// sections the same skills by category (whichever is set renders —
	// groups win when both are).
	Skills      []NameValue
	SkillGroups []SheetGroup
	// Styles lists combat styles; Passions lists passions.
	Styles, Passions []NameValue
	// Inventory is the carried-item table (quantity and equip markers).
	Inventory []InventoryItem
	// Locations are the hit locations with live HP.
	Locations []LocationHP
	// Conditions lists active conditions (poisoned, prone, …).
	Conditions []string
}

// LocationHP is one hit location's hit points as the metadata panel displays
// them. Severity classifies the wound: "" (unhurt), "wound", "severe" or
// "critical" — anything else renders with the neutral panel colour.
type LocationHP struct {
	// Name is the location's display name (for example "Right Arm").
	Name string
	// Current and Max are the location's current and maximum HP. A Current
	// above Max renders as a full bar.
	Current, Max int
	// Severity is "", "wound", "severe" or "critical".
	Severity string
}

// Vitals is the character block shown at the top of the metadata panel. The
// stat strings (HP, AP, PowerPoints, Luck) are display-ready as the harness
// formats them — the TUI never derives them.
type Vitals struct {
	// Name is the character's name.
	Name string
	// Culture and Career label the character's background line.
	Culture, Career string
	// HP, AP, PowerPoints and Luck are the pooled stat readouts (for
	// example "34/40"); empty when unknown.
	HP, AP, PowerPoints, Luck string
	// Locations lists hit locations with per-location HP bars.
	Locations []LocationHP
	// Conditions lists active conditions (poisoned, prone, …).
	Conditions []string
}

// ContextUsage is the footer's context readout: the input tokens the most
// recent model call reported (Tokens — the context size the model just
// saw) against the model's window (Window, 0 when unknown). It is advisory
// display data: compaction still counts messages, never tokens.
type ContextUsage struct {
	// Tokens is the last reported input-token count; 0 before the first
	// model call, which renders nothing.
	Tokens int
	// Window is the model's context window in tokens (0 = unknown, so
	// the footer shows tokens without a percentage).
	Window int
}

// Metadata is a coarse, display-ready snapshot of the session for the
// right-hand metadata panel. It is pure presentation data — no engine types,
// no behaviour — so the harness/session layer can assemble it from state
// without the TUI importing anything below it.
type Metadata struct {
	// Character is the player character block.
	Character Vitals
	// Sheet is the full character-sheet block behind the sheet overlay;
	// its zero value means "no character yet".
	Sheet Sheet
	// Inventory is the terse item list.
	Inventory []string
	// Scene is the current scene or location sentence.
	Scene string
	// Present lists who is in the scene with the character.
	Present []string
	// Clocks lists running clocks as display lines.
	Clocks []string
	// Flags lists set world or session flags.
	Flags []string
	// SessionName and InGameDate label the session and the calendar.
	SessionName, InGameDate string
	// ContextUsage feeds the footer's context readout (see the type).
	ContextUsage ContextUsage
}

// Event is one item on the transcript, delivered by the harness through
// Driver.Events. Markdown holds narrative/system/error text; Annotation and
// Dice are non-nil for their respective kinds. At is the event timestamp
// (zero is accepted and left unrendered for now).
type Event struct {
	Kind       EventKind
	Channel    Channel
	Markdown   string
	Annotation *Annotation
	Dice       *DiceEvent
	At         time.Time
	// Player marks the player's own message: set by the harness on the
	// echo it emits and on lines replayed from the log. The TUI dims
	// player text and tags its run with "You" (a GM run with "GM"), so
	// authorship survives a save/resume where no send history exists to
	// match against.
	Player bool
	// Replay marks an event rebuilt from log.jsonl on resume. The TUI
	// renders it as history: a replayed roll files as its finished
	// banner at once instead of performing the dice-theatre animation
	// again, and replayed lines never disturb the throbber or the echo
	// matching (both only act during a live turn).
	Replay bool
	// Continuation marks a narrative batch that EXTENDS the transcript
	// entry already open rather than starting one: streamed text of the
	// same paragraph arrives in several batches, and the TUI appends
	// them to the last narrative entry on the same channel (starting a
	// fresh entry instead when that entry cannot take it).
	Continuation bool
	// ConfirmText is the held player message an [EventConfirm] asks
	// about: answering "n" puts it back into the input line unsent. It
	// is only meaningful for EventConfirm; every other kind leaves it
	// empty.
	ConfirmText string
}

// Driver is the contract between the TUI and the game harness. The interface
// lives here, on the consumer side: the harness package (built separately)
// provides the implementation. The TUI never talks to the LLM directly.
//
// Contract:
//
//   - Send queues the player's text on ch and returns promptly; it should
//     not block on the model. A non-nil error is shown in the transcript as
//     an error line. On success the harness emits the player's message back
//     through Events (an EventNarrative on the same channel) — the TUI does
//     not echo input itself, the harness owns the transcript.
//   - SetChannel reports the input line's current channel after every mode
//     switch (ctrl+t, the palette's channel switch) so the harness can
//     remember the last-used mode across sessions. It is a UI hint, never
//     game state: the app calls it best-effort and absorbs its error
//     instead of surfacing it, and after Close it must be a no-op.
//   - A model turn is bracketed for the header throbber: the harness emits
//     EventThinking before each model call and exactly one EventTurnEnd
//     when the turn completes or aborts (error path included). Neither
//     carries transcript content, and both arrive in order with every
//     other event of that turn.
//   - Events returns the event stream; every call must return the same
//     channel, which the app reads once at construction. The app re-arms a
//     read after each event, so events must be delivered one at a time.
//     Closing the channel (from Close) signals shutdown: the app stops
//     listening and notes it in the transcript.
//   - Close releases the driver's resources and closes the channel from
//     Events. The app never calls it: ownership stays with whoever created
//     the driver (the CLI closes it when the program exits).
//   - A command is a local, named action the palette offers, or an
//     answer to a pending EventConfirm (save, recap, roll, meta,
//     confirm): Command enqueues it and returns promptly; it must not
//     block on the model or on the runner. Commands carry no player text,
//     produce no echo, and never reach the model — an unknown name comes
//     back through Events as an error line listing the known commands.
//     After Close it returns a closed-session error (like Send does).
//     TUI-local view actions (the sheet overlay, clear, theme,
//     help, quit, the channel switch) never cross this interface — they
//     are executed by the app itself.
//   - Metadata returns the snapshot behind the metadata panel. It must be
//     cheap and non-blocking (a read-through of state, never a model call)
//     and safe on the update loop: the app refreshes it on every processed
//     event, whenever the panel is toggled, and on a slow ~2s tick. The TUI
//     only renders what it returns, including the zero value.
//
// Send must be safe to call from the command goroutine Bubble Tea runs tea.Cmd
// functions on, concurrently with Events; SetChannel is called on the update
// loop and must be cheap and non-blocking. Command shares Send's goroutine
// rules: the app calls it from a tea.Cmd, never from the update loop.
type Driver interface {
	// Send queues the player's text on ch for the harness.
	Send(ctx context.Context, ch Channel, text string) error
	// Command queues one named local command (save, recap, roll, meta,
	// confirm) for the harness on the player's current channel.
	Command(name string, args ...string) error
	// SetChannel records the player's current IC/OOC mode. Implementations
	// persist it so a resumed session reopens on the same channel; the app
	// calls it best-effort after every mode switch and never surfaces its
	// error, so it must be a no-op once the driver is closed.
	SetChannel(ch Channel) error
	// Events returns the stream of harness events (same channel per call).
	Events() <-chan Event
	// Metadata returns the current display snapshot for the metadata panel.
	Metadata() Metadata
	// Close releases the driver and closes the Events channel.
	Close() error
}

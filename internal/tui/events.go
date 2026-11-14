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

// Metadata is a coarse, display-ready snapshot of the session for the
// right-hand metadata panel. It is pure presentation data — no engine types,
// no behaviour — so the harness/session layer can assemble it from state
// without the TUI importing anything below it.
type Metadata struct {
	// Character is the player character block.
	Character Vitals
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
//   - Metadata returns the snapshot behind the metadata panel. It must be
//     cheap and non-blocking (a read-through of state, never a model call)
//     and safe on the update loop: the app refreshes it on every processed
//     event, whenever the panel is toggled, and on a slow ~2s tick. The TUI
//     only renders what it returns, including the zero value.
//
// Send must be safe to call from the command goroutine Bubble Tea runs tea.Cmd
// functions on, concurrently with Events.
type Driver interface {
	// Send queues the player's text on ch for the harness.
	Send(ctx context.Context, ch Channel, text string) error
	// Events returns the stream of harness events (same channel per call).
	Events() <-chan Event
	// Metadata returns the current display snapshot for the metadata panel.
	Metadata() Metadata
	// Close releases the driver and closes the Events channel.
	Close() error
}

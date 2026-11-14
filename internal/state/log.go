package state

import (
	"encoding/json"
	"time"
)

// LogEntry is one line of a session's append-only log.jsonl: the move as
// the caller marshalled it, the Result it produced (or the failure), keyed
// by the state's EventSeq at the time — or, for Kind "chat", the play text
// in Chat. The log is the audit trail the play transcript regenerates
// from — never the source of game truth.
type LogEntry struct {
	// Seq is State.EventSeq when the entry was recorded (0 for entries
	// recorded before any accepted move, e.g. a refusal on a fresh
	// session).
	Seq uint64 `json:"seq"`
	// At is when the entry was recorded; Record stamps it with the store
	// clock when the caller leaves it zero.
	At time.Time `json:"at"`
	// Kind is the move's Kind — the GM-facing tool name ("skill_test").
	Kind string `json:"kind"`
	// Move is the marshalled Move arguments (json.RawMessage so the log
	// keeps exactly what was sent, whatever move type it names).
	Move json.RawMessage `json:"move,omitempty"`
	// Result is the marshalled resolve.Result on success.
	Result json.RawMessage `json:"result,omitempty"`
	// Error holds the failure message when the move was refused; empty on
	// success. A refused move changed nothing (resolve.Apply rolls back),
	// so the log records the refusal for audit only.
	Error string `json:"error,omitempty"`
	// Chat holds the play text of a chat line (Kind "chat"): the player's
	// echo or the GM's narration/error as the TUI showed it. Nil for
	// every other entry kind — entries written before chat lines existed
	// simply have no field, and ReadLog reads them unchanged.
	Chat *ChatLine `json:"chat,omitempty"`
}

// ChatLine is the neutral DTO a chat entry (LogEntry.Kind "chat") carries:
// the play text itself plus who said it, on which channel. It is pure
// data — the state layer knows nothing about the LLM or the TUI, so the
// session layer fills it in from its own types and replays it back.
type ChatLine struct {
	// Channel is the conversation channel label ("IC"/"OOC").
	Channel string `json:"channel,omitempty"`
	// Player marks the player's own line (the echo) as opposed to the
	// GM's text.
	Player bool `json:"player"`
	// Kind is the event the line was recorded from: "narrative" or
	// "error". Empty reads as "narrative".
	Kind string `json:"kind,omitempty"`
	// Text is the play text — the player's input verbatim, the GM's
	// narrative after channel-tag sanitisation.
	Text string `json:"text"`
}

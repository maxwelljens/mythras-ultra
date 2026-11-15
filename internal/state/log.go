package state

import (
	"encoding/json"
	"time"
)

// GoldenRuleCheckKind is the LogEntry.Kind of the session's golden-rule
// decision audit: one entry per completed check of an assistant reply,
// carrying {msg_seq, verdict, confidence, decision, attempt} in Move. It is
// raw audit only — [RenderTranscript] skips this kind, so the play shows
// the rewrite's note instead of the machinery behind it.
const GoldenRuleCheckKind = "golden_rule_check"

// Recorded chat kinds beyond "narrative" and "error" (all written by the
// session layer, all neutral strings the state layer itself understands).
const (
	// ChatKindRetract is the ChatLine.Kind of an append-only tombstone:
	// its MsgSeq names the assistant message whose recorded chat lines
	// are superseded by a rewrite — the transcript and the replay hide
	// exactly those lines and keep every other line. Text is empty.
	ChatKindRetract = "retract"
	// ChatKindNote is the ChatLine.Kind of a muted remark the play shows
	// as its own line (a system event on replay) — e.g. the note left
	// behind when the player accepts a rewritten reply.
	ChatKindNote = "note"
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
	// "error", plus the session's own line kinds "retract" (a tombstone
	// hiding the lines of one MsgSeq) and "note" (a muted remark; see
	// [ChatKindRetract] and [ChatKindNote]). Empty reads as "narrative".
	Kind string `json:"kind,omitempty"`
	// MsgSeq identifies the assistant message this line's text came from:
	// monotonic per assistant message within the session, 0 for lines
	// that predate the sequence (legacy logs) and for player echoes,
	// which belong to no model message. A "retract" line's MsgSeq names
	// the message it hides; a rewrite carries a fresh MsgSeq of its own.
	MsgSeq int `json:"msg_seq,omitempty"`
	// Text is the play text — the player's input verbatim, the GM's
	// narrative after channel-tag sanitisation. Empty on a "retract"
	// tombstone.
	Text string `json:"text"`
}

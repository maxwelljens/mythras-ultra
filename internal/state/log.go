package state

import (
	"encoding/json"
	"time"
)

// LogEntry is one line of a session's append-only log.jsonl: the move as
// the caller marshalled it, the Result it produced (or the failure), keyed
// by the state's EventSeq at the time. The log is the audit trail the
// transcript regenerates from — never the source of game truth.
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
}

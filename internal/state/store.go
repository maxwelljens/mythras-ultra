// Package state persists play sessions: the Store interface, an atomic JSON
// implementation under sessions/<id>/, the append-only move log and the
// conversation snapshot in chat.json (ARCHITECTURE.md §5).
//
// This layer owns wall time — its clock is injectable for tests — and it is
// the trust boundary for files on disk: Load validates session IDs (no path
// traversal), refuses unknown schema versions instead of reinterpreting
// them, and runs resolve.State.Normalize so pointer identity inside a
// loaded state is sound before anything touches it. The engine itself never
// reads a clock; everything needed to resume a game lives in state.json
// (game facts) and chat.json (the conversation) — no hidden state.
package state

import (
	"errors"
	"time"

	"github.com/mythras-ultra/myul/internal/engine/resolve"
)

// Sentinel errors returned by Store implementations (wrapped with %w so
// callers can branch with errors.Is).
var (
	// ErrNotFound reports an operation on a session that does not exist
	// (Load, Delete).
	ErrNotFound = errors.New("state: session not found")

	// ErrSchemaVersion reports state.json carrying a SchemaVersion this
	// build does not know — an older or newer schema than
	// resolve.SchemaVersion. The store refuses to load (and refuses to
	// overwrite) rather than reinterpret or corrupt unfamiliar data.
	ErrSchemaVersion = errors.New("state: unsupported schema version")

	// ErrExists reports a save that would overwrite a different session:
	// sessions/<id>/state.json already holds a state whose own ID is not
	// the one being saved (an identifier collision). Resume-overwrite of
	// the same session is not an error; clobbering another session is.
	ErrExists = errors.New("state: session directory already holds a different session")

	// ErrInvalidID reports a session ID that cannot name a directory
	// safely: empty, ".", "..", or containing a path separator or NUL.
	ErrInvalidID = errors.New("state: invalid session ID")

	// ErrInvalidState reports a nil *resolve.State, or one without a
	// session ID to key it by.
	ErrInvalidState = errors.New("state: invalid state")
)

// SnapshotKeep is how many timestamped state-<seq>.json copies Save
// retains beside the live state.json (the newest N; older ones are pruned
// after each save).
const SnapshotKeep = 10

// SessionInfo is one row of the session picker: identity, seed and the
// timestamps the persistence layer stamped (the engine never reads clocks).
type SessionInfo struct {
	// ID is the session identifier and directory name.
	ID string `json:"id"`
	// Name is what the session is called.
	Name string `json:"name"`
	// Seed is the RNG seed the session replays from.
	Seed int64 `json:"seed"`
	// CreatedAt and UpdatedAt are the persistence-stamped times.
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store persists and reads sessions. The JSON implementation lives in
// jsonstore.go; a bbolt implementation can slot in later without touching
// callers (ARCHITECTURE.md §5).
type Store interface {
	// Save writes the complete state atomically (temp file + fsync +
	// rename) and keeps the last SnapshotKeep timestamped snapshots.
	Save(*resolve.State) error
	// Load reads a session's state, verifies its schema version and runs
	// resolve.State.Normalize (re-binding encounter participants to the
	// canonical characters). Errors: ErrNotFound, ErrSchemaVersion.
	Load(id string) (*resolve.State, error)
	// List returns every session's info, newest UpdatedAt first.
	List() ([]SessionInfo, error)
	// Delete removes a session and everything under it. ErrNotFound when
	// it does not exist.
	Delete(id string) error
	// Record appends one entry to the session's log.jsonl.
	Record(id string, e LogEntry) error
	// ReadLog returns every recorded entry in order. A missing log is an
	// empty result, not an error; a truncated final line (crash mid-write)
	// is skipped rather than failing the read.
	ReadLog(id string) ([]LogEntry, error)
	// SaveChat writes the session's conversation-history bytes to
	// chat.json atomically (temp file + fsync + rename, like state.json).
	// The bytes are opaque here — the caller marshals them.
	SaveChat(id string, data []byte) error
	// LoadChat reads the bytes SaveChat wrote. ErrNotFound when the
	// session has no chat.json, and also when its contents are corrupt
	// (a truncated or hand-mangled file degrades to "no saved
	// conversation" instead of failing the resume).
	LoadChat(id string) ([]byte, error)
}

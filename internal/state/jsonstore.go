package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mythras-ultra/myul/internal/engine/resolve"
)

// JSONStore is the on-disk Store: sessions live under
// <root>/sessions/<id>/ with state.json (atomic, with the newest
// SnapshotKeep timestamped snapshots), log.jsonl (append-only),
// chat.json (the conversation, atomic) and meta.json (the session's UI
// preferences, atomic).
//
// The now function stamps Save's UpdatedAt/CreatedAt and Record's At — the
// STORE may use clocks, the ENGINE may not; pass a fixed clock in tests.
type JSONStore struct {
	root string
	now  func() time.Time

	// beforeRename is a test fault-injection hook invoked between the
	// flush and the rename of an atomic write; nil in production.
	beforeRename func() error
}

// NewJSONStore returns a store rooted at root (sessions appear under
// root/sessions). A nil now falls back to time.Now — the default for
// production; tests inject a fixed clock.
func NewJSONStore(root string, now func() time.Time) *JSONStore {
	if now == nil {
		now = time.Now
	}
	return &JSONStore{root: root, now: now}
}

// sessionDir maps a session ID to its directory, refusing IDs that could
// escape the sessions root (path traversal at the trust boundary).
func (j *JSONStore) sessionDir(id string) (string, error) {
	invalid := id == "" || id == "." || id == ".." ||
		strings.ContainsRune(id, '/') || strings.ContainsRune(id, '\\') ||
		strings.ContainsRune(id, 0) || filepath.Base(id) != id
	if invalid {
		return "", fmt.Errorf("%w %q", ErrInvalidID, id)
	}
	return filepath.Join(j.root, "sessions", id), nil
}

// Save writes the state atomically and rotates snapshots:
//
//  1. validate the state's identity and stamp wall time (UpdatedAt always,
//     CreatedAt on first save);
//  2. check an existing state.json — refuse to overwrite a different
//     session's directory (ErrExists) or an unknown schema
//     (ErrSchemaVersion), or to clobber unreadable bytes (fail loud so the
//     data survives for manual recovery);
//  3. write state.json via temp file + fsync + rename;
//  4. write the same bytes as state-<seq>.json and prune to SnapshotKeep.
//
// A failing write leaves the previous state.json intact — the rename never
// runs, and the temp file is removed.
func (j *JSONStore) Save(st *resolve.State) error {
	if st == nil {
		return fmt.Errorf("%w: nil state", ErrInvalidState)
	}
	if st.ID == "" {
		return fmt.Errorf("%w: state has no session ID", ErrInvalidState)
	}
	dir, err := j.sessionDir(st.ID)
	if err != nil {
		return err
	}
	st.UpdatedAt = j.now()
	if st.CreatedAt.IsZero() {
		st.CreatedAt = j.now()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("state: create session dir: %w", err)
	}

	statePath := filepath.Join(dir, "state.json")
	if err := j.checkExisting(statePath, st.ID); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("state: encode session %q: %w", st.ID, err)
	}
	data = append(data, '\n')
	if err := j.writeFileAtomic(statePath, data); err != nil {
		return err
	}

	seq, err := nextSeq(dir)
	if err != nil {
		return fmt.Errorf("state: scan snapshots for %q: %w", st.ID, err)
	}
	snapPath := filepath.Join(dir, fmt.Sprintf("state-%06d.json", seq))
	if err := j.writeFileAtomic(snapPath, data); err != nil {
		return fmt.Errorf("state: snapshot %q: %w", filepath.Base(snapPath), err)
	}
	return pruneSnapshots(dir)
}

// checkExisting validates what state.json already holds before it is
// overwritten. A missing file means first save; anything else must name the
// same session at the same schema version.
func (j *JSONStore) checkExisting(path, id string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("state: read existing state: %w", err)
	}
	var meta struct {
		ID            string `json:"id"`
		SchemaVersion int    `json:"schemaVersion"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return fmt.Errorf("state: existing state.json is unreadable (refusing to overwrite): %w", err)
	}
	if meta.ID != "" && meta.ID != id {
		return fmt.Errorf("%w: directory holds session %q, save requested for %q", ErrExists, meta.ID, id)
	}
	if meta.SchemaVersion != resolve.SchemaVersion {
		return fmt.Errorf("%w: existing state.json has schema version %d, this build supports %d",
			ErrSchemaVersion, meta.SchemaVersion, resolve.SchemaVersion)
	}
	return nil
}

// Load reads sessions/<id>/state.json, refuses unknown schema versions,
// re-binds the session ID when an ID-less state is filed under a directory,
// and runs resolve.State.Normalize so encounter pointer identity is sound
// before the state reaches the engine.
func (j *JSONStore) Load(id string) (*resolve.State, error) {
	dir, err := j.sessionDir(id)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %q", ErrNotFound, id)
		}
		return nil, fmt.Errorf("state: read session %q: %w", id, err)
	}
	var st resolve.State
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("state: decode session %q: %w", id, err)
	}
	if st.SchemaVersion != resolve.SchemaVersion {
		return nil, fmt.Errorf("%w: session %q has schema version %d, this build supports %d (refusing to load rather than reinterpret it)",
			ErrSchemaVersion, id, st.SchemaVersion, resolve.SchemaVersion)
	}
	if st.ID == "" {
		st.ID = id
	} else if st.ID != id {
		return nil, fmt.Errorf("%w: directory %q holds session %q", ErrInvalidState, id, st.ID)
	}
	if err := st.Normalize(); err != nil {
		return nil, fmt.Errorf("state: session %q failed integrity checks: %w", id, err)
	}
	return &st, nil
}

// List returns every session under root/sessions, newest UpdatedAt first.
// Directories without state.json are not sessions and are skipped;
// unreadable state.json is an error — a corrupt session must surface, not
// vanish from the picker. A missing sessions root is an empty list.
func (j *JSONStore) List() ([]SessionInfo, error) {
	root := filepath.Join(j.root, "sessions")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("state: list sessions: %w", err)
	}
	var out []SessionInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, e.Name(), "state.json"))
		if err != nil {
			if os.IsNotExist(err) {
				continue // not a session (no state written yet)
			}
			return nil, fmt.Errorf("state: read session %q: %w", e.Name(), err)
		}
		var info struct {
			ID            string    `json:"id"`
			Name          string    `json:"name"`
			Seed          int64     `json:"seed"`
			CreatedAt     time.Time `json:"createdAt"`
			UpdatedAt     time.Time `json:"updatedAt"`
			SchemaVersion int       `json:"schemaVersion"`
		}
		if err := json.Unmarshal(raw, &info); err != nil {
			return nil, fmt.Errorf("state: decode session %q: %w", e.Name(), err)
		}
		if info.SchemaVersion != resolve.SchemaVersion {
			// The session exists but this build cannot play it; surface
			// it with its identity so the picker can say why.
			return nil, fmt.Errorf("%w: session %q has schema version %d, this build supports %d",
				ErrSchemaVersion, info.ID, info.SchemaVersion, resolve.SchemaVersion)
		}
		if info.ID == "" {
			info.ID = e.Name()
		}
		out = append(out, SessionInfo{
			ID:        info.ID,
			Name:      info.Name,
			Seed:      info.Seed,
			CreatedAt: info.CreatedAt,
			UpdatedAt: info.UpdatedAt,
		})
	}
	sort.Slice(out, func(i, k int) bool {
		if !out[i].UpdatedAt.Equal(out[k].UpdatedAt) {
			return out[i].UpdatedAt.After(out[k].UpdatedAt)
		}
		return out[i].ID < out[k].ID
	})
	return out, nil
}

// Delete removes the session directory and everything in it (state,
// snapshots, log, chat). ErrNotFound when the session has never been
// saved.
func (j *JSONStore) Delete(id string) error {
	dir, err := j.sessionDir(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %q", ErrNotFound, id)
		}
		return fmt.Errorf("state: stat session %q: %w", id, err)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("state: delete session %q: %w", id, err)
	}
	return nil
}

// Record appends one entry to sessions/<id>/log.jsonl: one JSON object per
// line, fsynced so a crash keeps earlier entries. A zero At is stamped with
// the store clock. The directory is created on demand — losing an audit
// entry because no state has been saved yet would be data loss.
func (j *JSONStore) Record(id string, e LogEntry) error {
	dir, err := j.sessionDir(id)
	if err != nil {
		return err
	}
	if e.At.IsZero() {
		e.At = j.now()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("state: create session dir: %w", err)
	}
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("state: encode log entry for %q: %w", id, err)
	}
	line = append(line, '\n')
	path := filepath.Join(dir, "log.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("state: open log for %q: %w", id, err)
	}
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		return fmt.Errorf("state: append log for %q: %w", id, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("state: flush log for %q: %w", id, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("state: close log for %q: %w", id, err)
	}
	return nil
}

// ReadLog parses every entry of sessions/<id>/log.jsonl in order. A missing
// log returns an empty result (no moves recorded yet). A truncated final
// line — the crash signature of an append interrupted mid-write — is
// skipped; corruption anywhere else is an error, never silently dropped.
func (j *JSONStore) ReadLog(id string) ([]LogEntry, error) {
	dir, err := j.sessionDir(id)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "log.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("state: read log for %q: %w", id, err)
	}
	lines := bytes.Split(raw, []byte("\n"))
	// Index of the last non-empty line: only a failure there may be a
	// crash-truncated tail worth tolerating.
	lastNonEmpty := -1
	for i, ln := range lines {
		if len(bytes.TrimSpace(ln)) > 0 {
			lastNonEmpty = i
		}
	}
	var out []LogEntry
	for i, ln := range lines {
		if len(bytes.TrimSpace(ln)) == 0 {
			continue
		}
		var e LogEntry
		if err := json.Unmarshal(ln, &e); err != nil {
			if i == lastNonEmpty {
				break // truncated tail from a crash mid-append
			}
			return nil, fmt.Errorf("state: log for %q line %d: %w", id, i+1, err)
		}
		out = append(out, e)
	}
	return out, nil
}

// SaveChat writes data to sessions/<id>/chat.json with the same atomic
// dance as state.json (temp file + fsync + rename), so a crash mid-save
// keeps the previous conversation intact. The bytes are opaque — the
// caller (session) marshals the history; the store only persists them.
func (j *JSONStore) SaveChat(id string, data []byte) error {
	dir, err := j.sessionDir(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("state: create session dir: %w", err)
	}
	if err := j.writeFileAtomic(filepath.Join(dir, "chat.json"), data); err != nil {
		return fmt.Errorf("state: save chat for %q: %w", id, err)
	}
	return nil
}

// LoadChat reads sessions/<id>/chat.json. A missing file — and bytes that
// do not parse, the corrupt-tail signature of a file mangled outside the
// atomic write — both report ErrNotFound, so a resume degrades to an
// empty history instead of failing on a broken conversation snapshot.
// I/O problems other than absence (permissions, the sessions root
// unreadable) surface as errors: those must not look like "no history".
func (j *JSONStore) LoadChat(id string) ([]byte, error) {
	dir, err := j.sessionDir(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "chat.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %q has no chat.json", ErrNotFound, id)
		}
		return nil, fmt.Errorf("state: read chat for %q: %w", id, err)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("%w: %q chat.json is corrupt", ErrNotFound, id)
	}
	return data, nil
}

// SaveMeta writes data to sessions/<id>/meta.json with the same atomic
// dance as chat.json (temp file + fsync + rename), so a crash mid-save
// keeps the previous preferences intact. The bytes are opaque — the caller
// (session) marshals them; the store only persists them.
func (j *JSONStore) SaveMeta(id string, data []byte) error {
	dir, err := j.sessionDir(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("state: create session dir: %w", err)
	}
	if err := j.writeFileAtomic(filepath.Join(dir, "meta.json"), data); err != nil {
		return fmt.Errorf("state: save meta for %q: %w", id, err)
	}
	return nil
}

// LoadMeta reads sessions/<id>/meta.json. A missing file — and bytes that
// do not parse, the corrupt-file signature of a file mangled outside the
// atomic write — both report ErrNotFound, so a resume degrades to "no
// saved preferences" instead of failing. I/O problems other than absence
// (permissions, the sessions root unreadable) surface as errors: those
// must not look like "no preferences".
func (j *JSONStore) LoadMeta(id string) ([]byte, error) {
	dir, err := j.sessionDir(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %q has no meta.json", ErrNotFound, id)
		}
		return nil, fmt.Errorf("state: read meta for %q: %w", id, err)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("%w: %q meta.json is corrupt", ErrNotFound, id)
	}
	return data, nil
}

// writeFileAtomic writes data to path the way this store persists state:
// a temp file in the same directory, written and fsynced, then renamed over
// the target. A crash or failure at any step leaves the previous file
// untouched; the temp file is removed on error.
func (j *JSONStore) writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return fmt.Errorf("state: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("state: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("state: fsync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("state: close temp file: %w", err)
	}
	if j.beforeRename != nil {
		if err := j.beforeRename(); err != nil {
			_ = os.Remove(tmpName)
			return err
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("state: replace %s: %w", filepath.Base(path), err)
	}
	return nil
}

// nextSeq returns one past the highest state-<seq>.json in dir, so snapshot
// names are monotonic within a session (a resumed session continues the
// sequence; no clock is involved in naming).
func nextSeq(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	max := 0
	for _, e := range entries {
		n, ok := snapshotSeq(e.Name())
		if ok && n > max {
			max = n
		}
	}
	return max + 1, nil
}

// pruneSnapshots keeps the newest SnapshotKeep snapshots in dir, deleting
// older state-<seq>.json copies. Deletion errors propagate — a full disk or
// permission problem must not look like a successful rotation.
func pruneSnapshots(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("state: scan snapshots: %w", err)
	}
	var seqs []int
	for _, e := range entries {
		if n, ok := snapshotSeq(e.Name()); ok {
			seqs = append(seqs, n)
		}
	}
	if len(seqs) <= SnapshotKeep {
		return nil
	}
	sort.Ints(seqs)
	for _, n := range seqs[:len(seqs)-SnapshotKeep] {
		path := filepath.Join(dir, fmt.Sprintf("state-%06d.json", n))
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("state: prune snapshot %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

// snapshotSeq parses "state-<n>.json" names, reporting false for anything
// else (state.json, temp files).
func snapshotSeq(name string) (int, bool) {
	if !strings.HasPrefix(name, "state-") || !strings.HasSuffix(name, ".json") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "state-"), ".json"))
	if err != nil {
		return 0, false
	}
	return n, true
}

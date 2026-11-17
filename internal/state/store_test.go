package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/resolve"
)

// testStore returns a store under t.TempDir with a mutable fixed clock;
// advance it with the returned setter between saves.
func testStore(t *testing.T) (*JSONStore, *time.Time) {
	t.Helper()
	now := time.Date(2026, time.November, 14, 12, 0, 0, 0, time.UTC)
	store := NewJSONStore(t.TempDir(), func() time.Time { return now })
	return store, &now
}

// makeState builds a rich, resumable session: refreshed character, one
// accepted dice move (persisted RNG stream + EventSeq), a running encounter
// (participant identity!), scene and a recap.
func makeState(t *testing.T, id string) *resolve.State {
	t.Helper()
	st := resolve.NewState("The Ash Road", 42)
	st.ID = id
	c := &char.Character{
		ID: "pc-1", Name: "Ynara", PC: true,
		Chars: char.Characteristics{STR: 14, CON: 13, SIZ: 12, DEX: 15, INT: 12, POW: 14, CHA: 10},
	}
	if err := c.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	st.Characters[c.ID] = c
	for _, m := range []resolve.Move{
		resolve.RollDice{Expr: "2d6+1", Reason: "migration probe"},
		resolve.StartEncounter{Participants: []string{"pc-1"}},
		resolve.SetScene{Location: "The Broken Causeway", Description: "Rain.", Present: []string{"pc-1"}},
		resolve.EndRecap{Summary: "Session one: the ferryman lied."},
	} {
		if _, err := resolve.Apply(st, m); err != nil {
			t.Fatalf("%s: %v", m.Kind(), err)
		}
	}
	return st
}

// stateBytes renders a state the way Save persists it.
func stateBytes(t *testing.T, st *resolve.State) []byte {
	t.Helper()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return append(data, '\n')
}

// TestSaveLoadRoundTrip is the persistence contract: what Save wrote is
// exactly what Load returns — byte-identical after re-serialisation, with
// characters, encounter identity, RNG stream and counters intact.
func TestSaveLoadRoundTrip(t *testing.T) {
	store, _ := testStore(t)
	st := makeState(t, "s-round")
	if err := store.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := store.Load("s-round")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(stateBytes(t, st), stateBytes(t, loaded)) {
		t.Error("state did not round-trip losslessly through the store")
	}
	if !reflect.DeepEqual(st.Characters["pc-1"], loaded.Characters["pc-1"]) {
		t.Error("character drifted across save/load")
	}
	if len(loaded.RNGState) == 0 || string(loaded.RNGState) != string(st.RNGState) {
		t.Error("RNG stream not persisted")
	}
	if loaded.EventSeq != st.EventSeq || loaded.Seed != 42 {
		t.Errorf("counters = seq %d seed %d, want %d/%d", loaded.EventSeq, loaded.Seed, st.EventSeq, st.Seed)
	}
	if loaded.Encounter == nil || loaded.Encounter.Participants[0] != loaded.Characters["pc-1"] {
		t.Error("Load did not re-bind encounter participant identity")
	}
	if len(loaded.Recaps) != 1 || loaded.Scene.Location != "The Broken Causeway" {
		t.Errorf("scene/recaps = %+v / %v", loaded.Scene, loaded.Recaps)
	}
}

// TestRNGContinuityThroughStore is the resumption guarantee end to end:
// rolls before a Save + rolls after a Load equal one uninterrupted run.
func TestRNGContinuityThroughStore(t *testing.T) {
	const seed = 20261114
	roll := func(st *resolve.State, n int) []int {
		t.Helper()
		var totals []int
		for i := 0; i < n; i++ {
			res, err := resolve.Apply(st, resolve.RollDice{Expr: "1d100"})
			if err != nil {
				t.Fatalf("roll: %v", err)
			}
			totals = append(totals, res.Audit.Rolls[0].Total)
		}
		return totals
	}
	reference := roll(resolve.NewState("resume", seed), 12)

	store, _ := testStore(t)
	st := resolve.NewState("resume", seed)
	st.ID = "s-resume"
	head := roll(st, 5)
	if err := store.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}
	resumed, err := store.Load("s-resume")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tail := roll(resumed, 7)

	combined := append(append([]int{}, head...), tail...)
	if !reflect.DeepEqual(combined, reference) {
		t.Errorf("resumed sequence = %v, want uninterrupted %v", combined, reference)
	}
}

// TestSaveAtomicFailureKeepsPreviousState proves the atomic-write promise:
// a failure between flush and rename leaves state.json byte-identical, no
// temp litter and no snapshot; a retry then succeeds.
func TestSaveAtomicFailureKeepsPreviousState(t *testing.T) {
	store, _ := testStore(t)
	st := makeState(t, "s-atomic")
	if err := store.Save(st); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	statePath := filepath.Join(store.root, "sessions", "s-atomic", "state.json")
	original, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	snapshotsBefore := countSnapshots(t, filepath.Dir(statePath))

	// Advance the state, then fail the write at the last moment.
	if _, err := resolve.Apply(st, resolve.RollDice{Expr: "1d6"}); err != nil {
		t.Fatalf("move: %v", err)
	}
	store.beforeRename = func() error { return errors.New("injected write fault") }
	if err := store.Save(st); err == nil {
		t.Fatal("Save = nil with fault injected, want error")
	}
	store.beforeRename = nil

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.json after fault: %v", err)
	}
	if !bytes.Equal(original, after) {
		t.Error("failed write clobbered the previous state.json")
	}
	if got := countSnapshots(t, filepath.Dir(statePath)); got != snapshotsBefore {
		t.Errorf("snapshots = %d after failed save, want %d", got, snapshotsBefore)
	}
	for _, name := range dirNames(t, filepath.Dir(statePath)) {
		if strings.Contains(name, ".tmp") {
			t.Errorf("temp litter left behind: %s", name)
		}
	}

	// The fault cleared: the same save goes through.
	if err := store.Save(st); err != nil {
		t.Fatalf("retry Save: %v", err)
	}
	fresh, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if bytes.Equal(original, fresh) {
		t.Error("successful save did not update state.json")
	}
	if got := countSnapshots(t, filepath.Dir(statePath)); got != snapshotsBefore+1 {
		t.Errorf("snapshots = %d after successful save, want %d", got, snapshotsBefore+1)
	}
}

// TestSnapshotPruning pins the rotation: more than SnapshotKeep saves keep
// exactly the newest SnapshotKeep snapshots, and the newest matches the
// live state.
func TestSnapshotPruning(t *testing.T) {
	store, now := testStore(t)
	st := makeState(t, "s-rotate")
	dir := filepath.Join(store.root, "sessions", "s-rotate")
	const saves = SnapshotKeep + 5
	for i := 0; i < saves; i++ {
		now2 := now.Add(time.Duration(i+1) * time.Minute)
		store.now = func() time.Time { return now2 }
		if err := store.Save(st); err != nil {
			t.Fatalf("save %d: %v", i+1, err)
		}
	}
	got := countSnapshots(t, dir)
	if got != SnapshotKeep {
		t.Errorf("snapshots = %d after %d saves, want exactly %d", got, saves, SnapshotKeep)
	}
	names := dirNames(t, dir)
	// The newest sequence survives, the oldest (000001) is pruned.
	if !containsName(names, "state-000015.json") {
		t.Errorf("newest snapshot missing: %v", names)
	}
	if containsName(names, "state-000001.json") {
		t.Errorf("oldest snapshot not pruned: %v", names)
	}
	live, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	kept, err := os.ReadFile(filepath.Join(dir, "state-000015.json"))
	if err != nil {
		t.Fatalf("read newest snapshot: %v", err)
	}
	if !bytes.Equal(live, kept) {
		t.Error("newest snapshot differs from the live state.json")
	}
}

// TestSaveStampsClock checks the store's clock duty: UpdatedAt on every
// save, CreatedAt once, and Save never invents engine-side time.
func TestSaveStampsClock(t *testing.T) {
	store, now := testStore(t)
	st := makeState(t, "s-clock")
	t0 := *now
	if err := store.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !st.CreatedAt.Equal(t0) || !st.UpdatedAt.Equal(t0) {
		t.Errorf("timestamps = created %v updated %v, want %v", st.CreatedAt, st.UpdatedAt, t0)
	}
	t1 := t0.Add(time.Hour)
	store.now = func() time.Time { return t1 }
	st.Name = "renamed"
	if err := store.Save(st); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	if !st.CreatedAt.Equal(t0) {
		t.Errorf("CreatedAt moved: %v", st.CreatedAt)
	}
	if !st.UpdatedAt.Equal(t1) {
		t.Errorf("UpdatedAt = %v, want %v", st.UpdatedAt, t1)
	}
}

// TestLoadErrors covers the refusal ladder: unknown IDs (no traversal),
// missing sessions, wrong schema versions, and integrity failures from
// Normalize.
func TestLoadErrors(t *testing.T) {
	store, _ := testStore(t)

	if _, err := store.Load("s-nothing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing session error = %v, want ErrNotFound", err)
	}
	for _, bad := range []string{"", ".", "..", "../evil", "a/b", `a\b`} {
		if _, err := store.Load(bad); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Load(%q) error = %v, want ErrInvalidID", bad, err)
		}
	}

	// A newer schema is refused with a clear message, never reinterpreted.
	dir := filepath.Join(store.root, "sessions", "s-future")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	future := `{"schemaVersion": 999, "id": "s-future", "name": "tomorrow"}`
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(future), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := store.Load("s-future")
	if !errors.Is(err, ErrSchemaVersion) {
		t.Fatalf("error = %v, want ErrSchemaVersion", err)
	}
	if !strings.Contains(err.Error(), "999") || !strings.Contains(err.Error(), "supports") {
		t.Errorf("message = %q, want the found and supported versions", err)
	}

	// A future session also surfaces from the picker rather than vanishing.
	if _, err := store.List(); !errors.Is(err, ErrSchemaVersion) {
		t.Errorf("List with future session error = %v, want ErrSchemaVersion", err)
	}

	// Unreadable bytes: refuse to load — and refuse to overwrite on save.
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{corrupt"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := store.Load("s-future"); err == nil || errors.Is(err, ErrSchemaVersion) {
		t.Errorf("corrupt load error = %v, want a decode error", err)
	}
	corrupt := makeState(t, "s-future")
	if err := store.Save(corrupt); err == nil || strings.Contains(err.Error(), "schema") {
		t.Errorf("overwrite of corrupt state error = %v, want a refusal", err)
	}
}

// TestSaveIdentityGuards covers Save's argument and collision checks: nil
// or ID-less states, unsafe IDs, a directory that already belongs to a
// different session (ErrExists) and a foreign schema (ErrSchemaVersion).
func TestSaveIdentityGuards(t *testing.T) {
	store, _ := testStore(t)

	if err := store.Save(nil); !errors.Is(err, ErrInvalidState) {
		t.Errorf("nil state error = %v, want ErrInvalidState", err)
	}
	if err := store.Save(&resolve.State{}); !errors.Is(err, ErrInvalidState) {
		t.Errorf("ID-less state error = %v, want ErrInvalidState", err)
	}
	st := makeState(t, "a/b")
	if err := store.Save(st); !errors.Is(err, ErrInvalidID) {
		t.Errorf("unsafe ID error = %v, want ErrInvalidID", err)
	}

	// sessions/s-host already holds another session's state.
	host := filepath.Join(store.root, "sessions", "s-host")
	if err := os.MkdirAll(host, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	foreign := resolve.NewState("other", 7)
	foreign.ID = "s-elsewhere"
	if err := os.WriteFile(filepath.Join(host, "state.json"), stateBytes(t, foreign), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mine := makeState(t, "s-host")
	if err := store.Save(mine); !errors.Is(err, ErrExists) {
		t.Errorf("collision error = %v, want ErrExists", err)
	}
	// The foreign session's bytes survive untouched.
	got, err := os.ReadFile(filepath.Join(host, "state.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, stateBytes(t, foreign)) {
		t.Error("collision overwrote the other session")
	}

	// A different schemaVersion in the directory refuses a downgrade save.
	future := filepath.Join(store.root, "sessions", "s-old")
	if err := os.MkdirAll(future, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(future, "state.json"),
		[]byte(`{"schemaVersion": 999, "id": "s-old"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	old := makeState(t, "s-old")
	if err := store.Save(old); !errors.Is(err, ErrSchemaVersion) {
		t.Errorf("schema downgrade error = %v, want ErrSchemaVersion", err)
	}
}

// TestListAndDelete covers the picker and removal, with the injected clock
// ordering sessions newest first.
func TestListAndDelete(t *testing.T) {
	store, now := testStore(t)

	// Fresh store: no sessions root yet.
	if list, err := store.List(); err != nil || len(list) != 0 {
		t.Fatalf("fresh List = %v, %v; want empty", list, err)
	}

	first := makeState(t, "s-alpha")
	if err := store.Save(first); err != nil {
		t.Fatalf("save alpha: %v", err)
	}
	later := now.Add(30 * time.Minute)
	store.now = func() time.Time { return later }
	second := makeState(t, "s-beta")
	second.Name = "Second Road"
	if err := store.Save(second); err != nil {
		t.Fatalf("save beta: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List = %d entries, want 2", len(list))
	}
	if list[0].ID != "s-beta" || list[1].ID != "s-alpha" {
		t.Errorf("order = %s, %s; want newest first", list[0].ID, list[1].ID)
	}
	if list[0].Name != "Second Road" || list[0].Seed != 42 || !list[0].UpdatedAt.Equal(later) {
		t.Errorf("beta info = %+v", list[0])
	}
	if list[1].CreatedAt.IsZero() {
		t.Error("CreatedAt missing from List")
	}

	if err := store.Delete("s-alpha"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Load("s-alpha"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load after delete error = %v, want ErrNotFound", err)
	}
	if err := store.Delete("s-alpha"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete error = %v, want ErrNotFound", err)
	}
	list, err = store.List()
	if err != nil || len(list) != 1 || list[0].ID != "s-beta" {
		t.Errorf("List after delete = %v, %v", list, err)
	}
	if err := store.Delete("../evil"); !errors.Is(err, ErrInvalidID) {
		t.Errorf("Delete unsafe ID error = %v, want ErrInvalidID", err)
	}
}

// TestRecordReadLog covers append-only logging: field fidelity, clock
// stamping of zero At, append across calls, a missing log reading as empty,
// and recording before any save still working (audit never lost).
func TestRecordReadLog(t *testing.T) {
	store, now := testStore(t)
	st := makeState(t, "s-log")
	if err := store.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	move, err := json.Marshal(resolve.RollDice{Expr: "1d6", Reason: "omen"})
	if err != nil {
		t.Fatalf("marshal move: %v", err)
	}
	result, err := json.Marshal(resolve.Result{Summary: "2d6 → 7", Facts: []string{"total: 7"}})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	t0 := *now
	if err := store.Record("s-log", LogEntry{Seq: 1, Kind: "roll_dice", Move: move, Result: result}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := store.Record("s-log", LogEntry{
		Seq: 2, Kind: "skill_test",
		Move:  json.RawMessage(`{"actor":"pc-1","skill":"Brawn"}`),
		Error: "IMPOSSIBLE: unknown character",
	}); err != nil {
		t.Fatalf("Record 2: %v", err)
	}

	entries, err := store.ReadLog("s-log")
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ReadLog = %d entries, want 2", len(entries))
	}
	if entries[0].Seq != 1 || entries[0].Kind != "roll_dice" || entries[0].Error != "" {
		t.Errorf("entry 0 = %+v", entries[0])
	}
	if entries[0].At.IsZero() || !entries[0].At.Equal(t0) {
		t.Errorf("entry 0 At = %v, want stamped %v", entries[0].At, t0)
	}
	if !bytes.Equal(entries[0].Move, move) || !bytes.Equal(entries[0].Result, result) {
		t.Errorf("entry 0 payloads drifted:\nmove: %s\nresult: %s", entries[0].Move, entries[0].Result)
	}
	if entries[1].Seq != 2 || entries[1].Error == "" || entries[1].Result != nil {
		t.Errorf("entry 1 = %+v", entries[1])
	}

	// A session with no log yet reads as empty, not an error.
	if err := store.Save(makeState(t, "s-quiet")); err != nil {
		t.Fatalf("save quiet: %v", err)
	}
	if got, err := store.ReadLog("s-quiet"); err != nil || got != nil {
		t.Errorf("ReadLog on fresh session = %v, %v; want nil, nil", got, err)
	}
	// Recording before any save still works (append-only audit survives).
	if err := store.Record("s-newborn", LogEntry{Kind: "note", Error: "first refusal"}); err != nil {
		t.Fatalf("Record without save: %v", err)
	}
	if got, err := store.ReadLog("s-newborn"); err != nil || len(got) != 1 {
		t.Errorf("ReadLog = %v, %v", got, err)
	}
	if err := store.Record("../evil", LogEntry{}); !errors.Is(err, ErrInvalidID) {
		t.Errorf("unsafe ID error = %v, want ErrInvalidID", err)
	}
}

// TestReadLogTolerance covers the crash signature: a truncated final line
// is skipped, corruption anywhere else surfaces as an error.
func TestReadLogTolerance(t *testing.T) {
	store, _ := testStore(t)
	st := makeState(t, "s-crash")
	if err := store.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}
	logPath := filepath.Join(store.root, "sessions", "s-crash", "log.jsonl")
	good := `{"seq":1,"at":"2026-11-14T12:00:00Z","kind":"roll_dice"}` + "\n"
	// Simulate a crash mid-append: a partial line with no newline.
	partial := `{"seq":2,"at":"2026-11-14T12:00:01Z","kind":"skill_te`
	if err := os.WriteFile(logPath, []byte(good+partial), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	entries, err := store.ReadLog("s-crash")
	if err != nil {
		t.Fatalf("ReadLog must tolerate a truncated tail: %v", err)
	}
	if len(entries) != 1 || entries[0].Seq != 1 {
		t.Errorf("entries = %+v, want just the intact line", entries)
	}

	// Corruption in the middle is not a crash tail: it must surface.
	middle := good + `{"broken` + "\n" + good
	if err := os.WriteFile(logPath, []byte(middle), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if _, err := store.ReadLog("s-crash"); err == nil {
		t.Error("corrupt middle line was silently skipped")
	}
}

// TestSaveChatLoadChat covers the conversation snapshot: byte-exact
// round-trip, overwrite, unsafe IDs, and LoadChat's ladder — absent and
// corrupt files both report ErrNotFound (a resume degrades to an empty
// history), while a real I/O problem stays a real error.
func TestSaveChatLoadChat(t *testing.T) {
	store, _ := testStore(t)

	// Nothing written yet: ErrNotFound, not an empty success.
	if _, err := store.LoadChat("s-chat"); !errors.Is(err, ErrNotFound) {
		t.Errorf("LoadChat on a fresh session error = %v, want ErrNotFound", err)
	}
	for _, bad := range []string{"", ".", "..", "../evil", "a/b"} {
		if _, err := store.LoadChat(bad); !errors.Is(err, ErrInvalidID) {
			t.Errorf("LoadChat(%q) error = %v, want ErrInvalidID", bad, err)
		}
		if err := store.SaveChat(bad, nil); !errors.Is(err, ErrInvalidID) {
			t.Errorf("SaveChat(%q) error = %v, want ErrInvalidID", bad, err)
		}
	}

	// Round-trip: the store returns exactly the bytes it was given.
	data := []byte(`[{"role":"user","content":"I wake the relic."}]`)
	if err := store.SaveChat("s-chat", data); err != nil {
		t.Fatalf("SaveChat: %v", err)
	}
	got, err := store.LoadChat("s-chat")
	if err != nil {
		t.Fatalf("LoadChat: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("LoadChat = %s, want %s", got, data)
	}
	// Saving without a prior state.Save still lands (like Record).
	if err := store.SaveChat("s-newborn", []byte(`[]`)); err != nil {
		t.Fatalf("SaveChat without save: %v", err)
	}

	// Corrupt bytes (the truncated-file signature) read as ErrNotFound —
	// tolerant recovery, never a decode crash at resume.
	chatPath := filepath.Join(store.root, "sessions", "s-chat", "chat.json")
	if err := os.WriteFile(chatPath, []byte(`{"role": "use`), 0o644); err != nil {
		t.Fatalf("corrupt chat.json: %v", err)
	}
	if _, err := store.LoadChat("s-chat"); !errors.Is(err, ErrNotFound) {
		t.Errorf("LoadChat on corrupt file error = %v, want ErrNotFound", err)
	}
}

// TestSaveChatAtomicFailure proves the atomic-write promise for chat.json:
// a failure between flush and rename leaves the previous bytes untouched
// with no temp litter, and a retry then succeeds.
func TestSaveChatAtomicFailure(t *testing.T) {
	store, _ := testStore(t)
	original := []byte(`["turn one"]`)
	if err := store.SaveChat("s-chat-atomic", original); err != nil {
		t.Fatalf("first SaveChat: %v", err)
	}
	chatPath := filepath.Join(store.root, "sessions", "s-chat-atomic", "chat.json")

	store.beforeRename = func() error { return errors.New("injected write fault") }
	if err := store.SaveChat("s-chat-atomic", []byte(`["turn two"]`)); err == nil {
		t.Fatal("SaveChat = nil with fault injected, want error")
	}
	store.beforeRename = nil

	after, err := os.ReadFile(chatPath)
	if err != nil {
		t.Fatalf("read chat.json after fault: %v", err)
	}
	if !bytes.Equal(after, original) {
		t.Errorf("failed write clobbered chat.json: %s", after)
	}
	for _, name := range dirNames(t, filepath.Dir(chatPath)) {
		if strings.Contains(name, ".tmp") {
			t.Errorf("temp litter left behind: %s", name)
		}
	}
	if err := store.SaveChat("s-chat-atomic", []byte(`["turn two"]`)); err != nil {
		t.Fatalf("retry SaveChat: %v", err)
	}
	got, err := store.LoadChat("s-chat-atomic")
	if err != nil {
		t.Fatalf("LoadChat after retry: %v", err)
	}
	if string(got) != `["turn two"]` {
		t.Errorf("LoadChat = %s, want the retried bytes", got)
	}
}

// TestSaveMetaLoadMeta covers the session-preference snapshot: byte-exact
// round-trip, overwrite, unsafe IDs, and LoadMeta's ladder — absent and
// corrupt files both report ErrNotFound (a resume degrades to the default
// preferences), while a real I/O problem stays a real error.
func TestSaveMetaLoadMeta(t *testing.T) {
	store, _ := testStore(t)

	// Nothing written yet: ErrNotFound, not an empty success.
	if _, err := store.LoadMeta("s-meta"); !errors.Is(err, ErrNotFound) {
		t.Errorf("LoadMeta on a fresh session error = %v, want ErrNotFound", err)
	}
	for _, bad := range []string{"", ".", "..", "../evil", "a/b"} {
		if _, err := store.LoadMeta(bad); !errors.Is(err, ErrInvalidID) {
			t.Errorf("LoadMeta(%q) error = %v, want ErrInvalidID", bad, err)
		}
		if err := store.SaveMeta(bad, nil); !errors.Is(err, ErrInvalidID) {
			t.Errorf("SaveMeta(%q) error = %v, want ErrInvalidID", bad, err)
		}
	}

	// Round-trip: the store returns exactly the bytes it was given.
	data := []byte(`{"channel":"OOC"}`)
	if err := store.SaveMeta("s-meta", data); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}
	got, err := store.LoadMeta("s-meta")
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("LoadMeta = %s, want %s", got, data)
	}
	// Overwrite keeps the latest preferences (a mode switch mid-session).
	if err := store.SaveMeta("s-meta", []byte(`{"channel":"IC"}`)); err != nil {
		t.Fatalf("SaveMeta overwrite: %v", err)
	}
	got, err = store.LoadMeta("s-meta")
	if err != nil {
		t.Fatalf("LoadMeta after overwrite: %v", err)
	}
	if string(got) != `{"channel":"IC"}` {
		t.Errorf("LoadMeta = %s, want the overwritten bytes", got)
	}
	// Saving without a prior state.Save still lands (like SaveChat).
	if err := store.SaveMeta("s-newborn", data); err != nil {
		t.Fatalf("SaveMeta without save: %v", err)
	}

	// Corrupt bytes read as ErrNotFound — tolerant recovery, never a
	// decode crash at resume.
	metaPath := filepath.Join(store.root, "sessions", "s-meta", "meta.json")
	if err := os.WriteFile(metaPath, []byte(`{"channel": "OC`), 0o644); err != nil {
		t.Fatalf("corrupt meta.json: %v", err)
	}
	if _, err := store.LoadMeta("s-meta"); !errors.Is(err, ErrNotFound) {
		t.Errorf("LoadMeta on corrupt file error = %v, want ErrNotFound", err)
	}
}

// TestRecordReadLogChatEntry pins the chat entry round-trip: fields come
// back exactly, in order, beside move entries that read with Chat nil
// (old lines unaffected by the additive field).
func TestRecordReadLogChatEntry(t *testing.T) {
	store, _ := testStore(t)
	if err := store.Record("s-chatlog", LogEntry{
		Kind:   "roll_dice",
		Move:   json.RawMessage(`{"expr":"1d6"}`),
		Result: json.RawMessage(`{"ok":true}`),
	}); err != nil {
		t.Fatalf("Record move: %v", err)
	}
	line := &ChatLine{Channel: "OOC", Player: true, Kind: "narrative", Text: "we rest until dusk\n"}
	if err := store.Record("s-chatlog", LogEntry{Kind: "chat", Chat: line}); err != nil {
		t.Fatalf("Record chat: %v", err)
	}
	entries, err := store.ReadLog("s-chatlog")
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ReadLog = %d entries, want 2", len(entries))
	}
	if entries[0].Chat != nil {
		t.Errorf("move entry carries a chat line: %+v", entries[0].Chat)
	}
	if !reflect.DeepEqual(entries[1].Chat, line) {
		t.Errorf("chat entry = %+v, want %+v", entries[1].Chat, line)
	}
	if entries[1].Kind != "chat" {
		t.Errorf("kind = %q, want chat", entries[1].Kind)
	}
}

// TestDefaultClock checks the constructor fallback (production uses
// time.Now when no clock is injected).
func TestDefaultClock(t *testing.T) {
	store := NewJSONStore(t.TempDir(), nil)
	if err := store.Save(makeState(t, "s-now")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	st, err := store.Load("s-now")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.UpdatedAt.IsZero() || time.Since(st.UpdatedAt) > time.Hour {
		t.Errorf("UpdatedAt = %v, want a real recent stamp", st.UpdatedAt)
	}
}

// countSnapshots returns how many state-<seq>.json files dir holds.
func countSnapshots(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	for _, name := range dirNames(t, dir) {
		if strings.HasPrefix(name, "state-") && strings.HasSuffix(name, ".json") {
			n++
		}
	}
	return n
}

// dirNames lists a directory's entry names.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// containsName reports membership of a file name list.
func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

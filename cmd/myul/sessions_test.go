// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mythras-ultra/myul/internal/config"
	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/resolve"
	"github.com/mythras-ultra/myul/internal/state"
)

// sessionEnv points the config and data directories at fresh temp dirs and
// clears the ambient MYUL_* keys, so the commands under test only ever see
// what this test builds.
func sessionEnv(t *testing.T) {
	t.Helper()
	blankEnv(t)
	t.Setenv("MYUL_DATA_DIR", t.TempDir())
	t.Setenv("MYUL_CONFIG_DIR", t.TempDir())
}

// executeIn runs the root command with an explicit stdin — delete's typed
// confirmation reads it, and tests must never inherit a live terminal.
func executeIn(t *testing.T, in io.Reader, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetIn(in)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// seedSessions writes three sessions through the real JSONStore — each with
// a refreshed PC, two moves applied through resolve.Apply and recorded in
// log.jsonl, and a later second Save for UpdatedAt. Creation order is
// alpha, beta, gamma, so a listing sorted by UpdatedAt descending must
// reverse it (gamma first).
func seedSessions(t *testing.T) {
	t.Helper()
	now := time.Date(2026, time.November, 14, 9, 0, 0, 0, time.UTC)
	store := state.NewJSONStore(config.DataDir(), func() time.Time { return now })
	specs := []struct {
		id, name string
		seed     int64
		recap    string
	}{
		{"s-alpha", "Salt and Cinder", 7, "Session one: the salt ran out."},
		{"s-beta", "The Ash Road", 42, "Session one: the ferryman lied."},
		{"s-gamma", "Winter's Reach", 1234, "Session one: the pass closed."},
	}
	for _, spec := range specs {
		now = now.Add(time.Hour)
		st := resolve.NewState(spec.name, spec.seed)
		st.ID = spec.id
		c := &char.Character{
			ID: "pc-1", Name: "Ynara", PC: true,
			Chars: char.Characteristics{STR: 14, CON: 13, SIZ: 12, DEX: 15, INT: 12, POW: 14, CHA: 10},
		}
		if err := c.Refresh(); err != nil {
			t.Fatalf("refresh %s: %v", spec.id, err)
		}
		st.Characters[c.ID] = c
		if err := store.Save(st); err != nil {
			t.Fatalf("Save %s: %v", spec.id, err)
		}
		for _, m := range []resolve.Move{
			resolve.RollDice{Expr: "1d6", Reason: "omen"},
			resolve.EndRecap{Summary: spec.recap},
		} {
			res, err := resolve.Apply(st, m)
			if err != nil {
				t.Fatalf("%s on %s: %v", m.Kind(), spec.id, err)
			}
			moveRaw, err := json.Marshal(m)
			if err != nil {
				t.Fatalf("marshal move: %v", err)
			}
			resRaw, err := json.Marshal(res)
			if err != nil {
				t.Fatalf("marshal result: %v", err)
			}
			if err := store.Record(spec.id, state.LogEntry{
				Seq: st.EventSeq, Kind: m.Kind(), Move: moveRaw, Result: resRaw,
			}); err != nil {
				t.Fatalf("Record %s: %v", spec.id, err)
			}
		}
		now = now.Add(30 * time.Minute)
		if err := store.Save(st); err != nil {
			t.Fatalf("second Save %s: %v", spec.id, err)
		}
	}
}

// currentSessions reopens the store the commands use, for assertions about
// what survived them.
func currentSessions(t *testing.T) []state.SessionInfo {
	t.Helper()
	infos, err := state.NewJSONStore(config.DataDir(), nil).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return infos
}

func sessionIDs(infos []state.SessionInfo) []string {
	ids := make([]string, 0, len(infos))
	for _, s := range infos {
		ids = append(ids, s.ID)
	}
	return ids
}

func TestSessionsList(t *testing.T) {
	sessionEnv(t)
	seedSessions(t)

	out, err := execute(t, "sessions")
	if err != nil {
		t.Fatalf("sessions error = %v", err)
	}
	for _, want := range []string{
		"ID", "NAME", "SEED", "UPDATED",
		"s-gamma", "Winter's Reach", "2026-11-14T13:30:00Z",
		"s-beta", "The Ash Road", "2026-11-14T12:00:00Z",
		"s-alpha", "Salt and Cinder", "2026-11-14T10:30:00Z",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sessions output missing %q:\n%s", want, out)
		}
	}
	// Newest UpdatedAt first: gamma, then beta, then alpha.
	ig, ib, ia := strings.Index(out, "s-gamma"), strings.Index(out, "s-beta"), strings.Index(out, "s-alpha")
	if ig >= ib || ib >= ia {
		t.Errorf("sessions order = gamma@%d beta@%d alpha@%d, want newest first:\n%s", ig, ib, ia, out)
	}
}

func TestSessionsListJSON(t *testing.T) {
	sessionEnv(t)
	seedSessions(t)

	out, err := execute(t, "sessions", "--json")
	if err != nil {
		t.Fatalf("sessions --json error = %v", err)
	}
	var infos []state.SessionInfo
	if err := json.Unmarshal([]byte(out), &infos); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}
	want := []string{"s-gamma", "s-beta", "s-alpha"}
	if len(infos) != len(want) {
		t.Fatalf("got %d sessions, want %d: %+v", len(infos), len(want), infos)
	}
	for i, id := range want {
		if infos[i].ID != id {
			t.Errorf("session %d = %q, want %q", i, infos[i].ID, id)
		}
	}
	if infos[1].Seed != 42 || infos[1].Name != "The Ash Road" {
		t.Errorf("s-beta info = %+v, want name The Ash Road seed 42", infos[1])
	}
}

func TestSessionsEmpty(t *testing.T) {
	sessionEnv(t)

	out, err := execute(t, "sessions")
	if err != nil {
		t.Fatalf("sessions on empty store error = %v (must exit 0)", err)
	}
	if !strings.Contains(out, "no sessions yet") {
		t.Errorf("empty listing = %q, want friendly hint", out)
	}

	out, err = execute(t, "sessions", "--json")
	if err != nil {
		t.Fatalf("sessions --json on empty store error = %v", err)
	}
	var infos []state.SessionInfo
	if err := json.Unmarshal([]byte(out), &infos); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}
	if len(infos) != 0 {
		t.Errorf("empty store JSON = %+v, want no sessions", infos)
	}
}

func TestDeleteUnknownID(t *testing.T) {
	sessionEnv(t)
	seedSessions(t)

	out, err := execute(t, "delete", "s-nope")
	if err == nil {
		t.Fatal("delete of unknown ID: want error, got nil")
	}
	if !strings.Contains(err.Error(), `unknown session "s-nope"`) {
		t.Errorf("error = %v, want unknown-session message", err)
	}
	if !strings.Contains(err.Error(), "s-alpha") || !strings.Contains(err.Error(), "s-beta") {
		t.Errorf("error = %v, want the known IDs listed", err)
	}
	if !strings.Contains(out, "Error:") {
		t.Errorf("output = %q, want a clean cobra Error: line", out)
	}
	if ids := sessionIDs(currentSessions(t)); len(ids) != 3 {
		t.Errorf("store after refused delete = %v, want all three sessions", ids)
	}
}

func TestDeleteRefusedWithoutConfirmation(t *testing.T) {
	sessionEnv(t)
	seedSessions(t)

	// Empty stdin: the confirmation can never be typed, so the delete
	// aborts (this is also the non-interactive path — EOF, never a hang).
	out, err := executeIn(t, strings.NewReader(""), "delete", "s-beta")
	if err == nil {
		t.Fatal("delete without confirmation: want error, got nil")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("error = %v, want aborted", err)
	}
	if !strings.Contains(out, `Type "delete s-beta"`) {
		t.Errorf("output = %q, want the typed-confirmation prompt", out)
	}
	if ids := sessionIDs(currentSessions(t)); len(ids) != 3 {
		t.Errorf("store after aborted delete = %v, want all three sessions", ids)
	}

	// A confirmation that does not match aborts just the same.
	if _, err := executeIn(t, strings.NewReader("delete s-alpha\n"), "delete", "s-beta"); err == nil {
		t.Error("delete with wrong confirmation: want error, got nil")
	}
	if ids := sessionIDs(currentSessions(t)); len(ids) != 3 {
		t.Errorf("store after mismatched delete = %v, want all three sessions", ids)
	}
}

func TestDeleteConfirmed(t *testing.T) {
	sessionEnv(t)
	seedSessions(t)

	out, err := executeIn(t, strings.NewReader("delete s-beta\n"), "delete", "s-beta")
	if err != nil {
		t.Fatalf("confirmed delete error = %v", err)
	}
	if !strings.Contains(out, "deleted s-beta") {
		t.Errorf("output = %q, want confirmation of deletion", out)
	}
	ids := sessionIDs(currentSessions(t))
	if len(ids) != 2 || strings.Contains(strings.Join(ids, ","), "s-beta") {
		t.Errorf("store after delete = %v, want s-beta gone", ids)
	}
}

func TestDeleteForce(t *testing.T) {
	sessionEnv(t)
	seedSessions(t)

	out, err := execute(t, "delete", "s-beta", "--force")
	if err != nil {
		t.Fatalf("delete --force error = %v", err)
	}
	if !strings.Contains(out, "deleted s-beta") {
		t.Errorf("output = %q, want confirmation of deletion", out)
	}
	ids := sessionIDs(currentSessions(t))
	if len(ids) != 2 || strings.Contains(strings.Join(ids, ","), "s-beta") {
		t.Errorf("store after delete --force = %v, want s-beta gone", ids)
	}
	// --force on an unknown ID still fails, listing the known ones.
	if _, err := execute(t, "delete", "s-nope", "--force"); err == nil {
		t.Error("delete --force unknown ID: want error, got nil")
	}
}

func TestExportTranscript(t *testing.T) {
	sessionEnv(t)
	seedSessions(t)

	out, err := execute(t, "export", "s-beta")
	if err != nil {
		t.Fatalf("export error = %v", err)
	}
	for _, want := range []string{
		"# The Ash Road — Play Log\n",
		"- Session: s-beta\n",
		"- Seed: 42\n",
		"- **roll_dice** — ",
		"  - roll: 1d6 → ",
		"- **end_session_recap** — ",
		"## Session Recap\n",
		"1. Session one: the ferryman lied.\n",
		"## Final Characters\n",
		"- **Ynara** (pc-1, PC)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("export output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "the salt ran out") || strings.Contains(out, "the pass closed") {
		t.Errorf("export mixed in another session's log:\n%s", out)
	}
}

func TestExportState(t *testing.T) {
	sessionEnv(t)
	seedSessions(t)

	out, err := execute(t, "export", "s-beta", "--state")
	if err != nil {
		t.Fatalf("export --state error = %v", err)
	}
	var st resolve.State
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatalf("decode state: %v\n%s", err, out)
	}
	if st.ID != "s-beta" || st.Name != "The Ash Road" || st.Seed != 42 {
		t.Errorf("exported state = id %q name %q seed %d", st.ID, st.Name, st.Seed)
	}
	if st.SchemaVersion != resolve.SchemaVersion {
		t.Errorf("schema version = %d, want %d", st.SchemaVersion, resolve.SchemaVersion)
	}
}

func TestExportOutFile(t *testing.T) {
	sessionEnv(t)
	seedSessions(t)

	path := filepath.Join(t.TempDir(), "play.md")
	out, err := execute(t, "export", "s-beta", "--out", path)
	if err != nil {
		t.Fatalf("export --out error = %v", err)
	}
	if !strings.Contains(out, "wrote "+path) {
		t.Errorf("output = %q, want wrote-%s notice", out, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}
	if !strings.Contains(string(data), "# The Ash Road — Play Log") {
		t.Errorf("export file = %q, want the transcript header", string(data))
	}
}

func TestExportUnknownID(t *testing.T) {
	sessionEnv(t)
	seedSessions(t)

	out, err := execute(t, "export", "s-nope")
	if err == nil {
		t.Fatal("export of unknown ID: want error, got nil")
	}
	if !errors.Is(err, state.ErrNotFound) {
		t.Errorf("error = %v, want it to wrap state.ErrNotFound", err)
	}
	if !strings.Contains(out, "Error:") {
		t.Errorf("output = %q, want a clean cobra Error: line", out)
	}
}

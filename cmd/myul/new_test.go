// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createSession runs `myul new --no-tui` in the test's isolated
// environment and returns the printed session id.
func createSession(t *testing.T, name string) string {
	t.Helper()
	out, err := execute(t, "new", "--name", name, "--seed", "42", "--no-tui")
	if err != nil {
		t.Fatalf("new --no-tui error = %v (output %q)", err, out)
	}
	return parseCreatedID(t, out)
}

// parseCreatedID pulls the id out of flushNoTUI's
// `created session <id> (<name>)` line.
func parseCreatedID(t *testing.T, out string) string {
	t.Helper()
	rest, ok := strings.CutPrefix(out, "created session ")
	if !ok {
		t.Fatalf("new --no-tui output = %q, want a created-session line", out)
	}
	id, _, _ := strings.Cut(rest, " (")
	if id == "" {
		t.Fatalf("new --no-tui output %q carries no session id", out)
	}
	return id
}

// TestNewNoTUI is the scripting path: flags only, real store, session
// flushed to disk under <data dir>/sessions/<id>/ and no TUI.
func TestNewNoTUI(t *testing.T) {
	dataDir, _ := isolatedDirs(t)

	out, err := execute(t, "new", "--name", "The Ashen Coast", "--seed", "42", "--no-tui")
	if err != nil {
		t.Fatalf("new --no-tui error = %v (output %q)", err, out)
	}
	id := parseCreatedID(t, out)
	if !strings.Contains(out, `"The Ashen Coast"`) {
		t.Errorf("new --no-tui output missing the session name: %q", out)
	}

	dir := filepath.Join(dataDir, "sessions", id)
	for _, name := range []string{"state.json", "log.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("new --no-tui did not write %s: %v", name, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	var st struct {
		Seed int64 `json:"seed"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("decode state.json: %v", err)
	}
	if st.Seed != 42 {
		t.Errorf("state.json seed = %d, want 42", st.Seed)
	}
}

// TestNewNameFromArgument covers the positional [name] form.
func TestNewNameFromArgument(t *testing.T) {
	isolatedDirs(t)
	out, err := execute(t, "new", "From Argument", "--no-tui")
	if err != nil {
		t.Fatalf("new <name> --no-tui error = %v (output %q)", err, out)
	}
	if id := parseCreatedID(t, out); id == "" {
		t.Errorf("no session id in %q", out)
	}
	if !strings.Contains(out, `"From Argument"`) {
		t.Errorf("output missing the session name: %q", out)
	}
}

// TestNewConflictingName: the [name] argument and --name must agree.
func TestNewConflictingName(t *testing.T) {
	isolatedDirs(t)
	out, err := execute(t, "new", "One", "--name", "Two", "--no-tui")
	if err == nil {
		t.Fatalf("conflicting names: want error, got nil (output %q)", out)
	}
	if !strings.Contains(err.Error(), "name given twice") {
		t.Errorf("error = %q, want a conflicting-name complaint", err)
	}
}

// TestNewWithoutNoTUI defines the non-interactive branch of `myul new`
// without --no-tui: in a pipe there is no terminal for the form or the
// TUI, so the command fails cleanly instead of hanging — first on the
// missing model configuration, then on the missing terminal once a model
// is configured — and leaves no half-made session behind.
func TestNewWithoutNoTUI(t *testing.T) {
	t.Run("model missing", func(t *testing.T) {
		dataDir, _ := isolatedDirs(t)
		out, err := execute(t, "new", "--name", "Pipe")
		if err == nil {
			t.Fatalf("new without --no-tui: want error, got nil (output %q)", out)
		}
		if !strings.Contains(err.Error(), "no language model configured") {
			t.Errorf("error = %q, want the missing-model complaint", err)
		}
		assertNoSessions(t, dataDir)
	})

	t.Run("model configured", func(t *testing.T) {
		dataDir, configDir := isolatedDirs(t)
		writeConfig(t, filepath.Join(configDir, "config.toml"),
			"[llm]\nprovider = \"openaicompat\"\nmodel = \"test-model\"\n")
		out, err := execute(t, "new", "--name", "Pipe")
		if err == nil {
			t.Fatalf("new without --no-tui: want error, got nil (output %q)", out)
		}
		if !strings.Contains(err.Error(), "interactive terminal is required") {
			t.Errorf("error = %q, want the terminal complaint", err)
		}
		assertNoSessions(t, dataDir)
	})
}

// TestNewNoTUNeedsNoCredentials: a headless session never talks to the
// model, so a provider that requires an API key must not block creation
// (the client is only built for play).
func TestNewNoTUNeedsNoCredentials(t *testing.T) {
	_, configDir := isolatedDirs(t)
	writeConfig(t, filepath.Join(configDir, "config.toml"),
		"[llm]\nprovider = \"openai\"\nmodel = \"gpt-test\"\napi_key_env = \"MYUL_TEST_MISSING_KEY\"\n")

	out, err := execute(t, "new", "No Key Needed", "--no-tui")
	if err != nil {
		t.Fatalf("new --no-tui with an unreachable provider setup: %v (output %q)", err, out)
	}
	if id := parseCreatedID(t, out); id == "" {
		t.Errorf("no session id in %q", out)
	}
}

// assertNoSessions fails when the data dir gained a session directory.
func assertNoSessions(t *testing.T, dataDir string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dataDir, "sessions"))
	if err != nil {
		return // no sessions directory at all — exactly what we want
	}
	if len(entries) > 0 {
		t.Errorf("failed run left %d session(s) behind", len(entries))
	}
}

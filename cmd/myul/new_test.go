// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"encoding/json"
	"errors"
	"io/fs"
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
// `created session <id> (<name>)` line — anywhere in the output, since
// the deferred-LLM path prints its config guidance first.
func parseCreatedID(t *testing.T, out string) string {
	t.Helper()
	_, rest, found := strings.Cut(out, "created session ")
	if !found {
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

// TestNewSkipLLM: the non-interactive custom path — `myul new --skip-llm`
// writes the template config exactly once, creates the session headless
// (the --no-tui seam: in a pipe a TUI attempt would fail on the missing
// terminal, so a clean exit proves it was never launched) and prints the
// config path, the keys to set and the resume guidance.
func TestNewSkipLLM(t *testing.T) {
	dataDir, configDir := isolatedDirs(t)
	configFile := filepath.Join(configDir, "config.toml")

	out, err := execute(t, "new", "--name", "Deferred", "--seed", "7", "--skip-llm")
	if err != nil {
		t.Fatalf("new --skip-llm error = %v (output %q)", err, out)
	}
	if id := parseCreatedID(t, out); id == "" {
		t.Errorf("no session id in %q", out)
	}
	raw, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("template config not written: %v", err)
	}
	if !strings.Contains(out, configFile) {
		t.Errorf("output missing the config path %q: %q", configFile, out)
	}
	if !strings.Contains(out, "[llm] provider, model, api_key_env, base_url") {
		t.Errorf("output missing the keys-to-set guidance: %q", out)
	}
	if !strings.Contains(out, "myul resume") {
		t.Errorf("output missing the resume guidance: %q", out)
	}

	// A second run must leave the (now user-owned) file untouched.
	marker := string(raw) + "\n# customised by the user\n"
	if err := os.WriteFile(configFile, []byte(marker), 0o644); err != nil {
		t.Fatalf("mark config: %v", err)
	}
	out2, err := execute(t, "new", "--name", "Deferred Again", "--seed", "7", "--skip-llm")
	if err != nil {
		t.Fatalf("second new --skip-llm error = %v (output %q)", err, out2)
	}
	after, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("re-read config: %v", err)
	}
	if string(after) != marker {
		t.Error("second --skip-llm run overwrote the existing config file")
	}
	if !strings.Contains(out2, "using existing config") {
		t.Errorf("second run does not report reusing the config: %q", out2)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "sessions"))
	if err != nil {
		t.Fatalf("sessions dir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d sessions, want both --skip-llm runs to create one", len(entries))
	}
}

// TestNewProviderModelFlagsKeepWorking: --provider/--model settle the LLM
// step (guided without prompts), so the scripting path is unchanged — the
// session is created and no config template is written.
func TestNewProviderModelFlagsKeepWorking(t *testing.T) {
	_, configDir := isolatedDirs(t)

	out, err := execute(t, "new", "--name", "Flagged", "--seed", "42",
		"--provider", "openaicompat", "--model", "flag-model", "--no-tui")
	if err != nil {
		t.Fatalf("new --provider/--model error = %v (output %q)", err, out)
	}
	if id := parseCreatedID(t, out); id == "" {
		t.Errorf("no session id in %q", out)
	}
	if _, err := os.Stat(filepath.Join(configDir, "config.toml")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("provider/model flags must not write a config file, stat = %v", err)
	}
}

// TestNewMissingAPIKeyFailsActionably: with provider/model configured but
// the effective api-key variable unset, play refuses before any session
// exists — naming the variable and the config file, exit 1, no stack.
func TestNewMissingAPIKeyFailsActionably(t *testing.T) {
	dataDir, configDir := isolatedDirs(t)
	writeConfig(t, filepath.Join(configDir, "config.toml"),
		"[llm]\nprovider = \"deepseek\"\nmodel = \"deepseek-chat\"\n")
	// The catalog default for deepseek; forced unset so the ambient
	// environment cannot satisfy the probe.
	t.Setenv("DEEPSEEK_API_KEY", "")

	out, err := execute(t, "new", "--name", "No Key")
	if err == nil {
		t.Fatalf("new with an unset API key: want error, got nil (output %q)", out)
	}
	msg := err.Error()
	if !strings.Contains(msg, "$DEEPSEEK_API_KEY") {
		t.Errorf("error = %q, want it to name $DEEPSEEK_API_KEY", msg)
	}
	if !strings.Contains(msg, filepath.Join(configDir, "config.toml")) {
		t.Errorf("error = %q, want it to name the config path", msg)
	}
	assertNoSessions(t, dataDir)
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

// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestResumeUnknownSession: an id the store does not know fails with a
// clean message that lists the ids that do exist (exit 1).
func TestResumeUnknownSession(t *testing.T) {
	isolatedDirs(t)
	known := createSession(t, "Known")

	out, err := execute(t, "resume", "no-such-id")
	if err == nil {
		t.Fatalf("resume <unknown>: want error, got nil (output %q)", out)
	}
	msg := err.Error()
	if !strings.Contains(msg, `unknown session "no-such-id"`) {
		t.Errorf("error = %q, want the unknown-session complaint", msg)
	}
	if !strings.Contains(msg, known) || !strings.Contains(msg, "known:") {
		t.Errorf("error = %q, want it to list the known session ids", msg)
	}
}

// TestResumeEmptyStore: `myul resume` with nothing saved points at
// `myul new` instead of failing obscurely.
func TestResumeEmptyStore(t *testing.T) {
	isolatedDirs(t)
	out, err := execute(t, "resume")
	if err == nil {
		t.Fatalf("resume on an empty store: want error, got nil (output %q)", out)
	}
	if !strings.Contains(err.Error(), "no sessions yet") {
		t.Errorf("error = %q, want the empty-store hint", err)
	}
}

// TestResumeKnownSessionNeedsTerminal: a known id passes the store check
// and is only refused for the missing TUI — proving resume resolves the
// id before validating play conditions.
func TestResumeKnownSessionNeedsTerminal(t *testing.T) {
	_, configDir := isolatedDirs(t)
	writeConfig(t, filepath.Join(configDir, "config.toml"),
		"[llm]\nprovider = \"openaicompat\"\nmodel = \"test-model\"\n")
	id := createSession(t, "Resumable")

	out, err := execute(t, "resume", id)
	if err == nil {
		t.Fatalf("resume without a terminal: want error, got nil (output %q)", out)
	}
	if !strings.Contains(err.Error(), "interactive terminal is required") {
		t.Errorf("error = %q, want the terminal complaint", err)
	}
}

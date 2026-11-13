package prompts

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testVars returns distinctive values so the test fails loudly if a
// placeholder stops rendering or renders the wrong field.
func testVars() Vars {
	return Vars{
		SessionName:      "The Salt-Crowned Coast",
		Recap:            "You escaped the burning shrine with the relic.",
		CharacterSummary: "Wren, a Marentian scout; Stealth 75%, wounded left arm.",
		WorldSummary:     "Two rival cults manoeuvre for the relic; the harbour is closed.",
		Date:             "12 Fireday, 1620 ST",
	}
}

func TestAssembleDefaultPrompt(t *testing.T) {
	v := testVars()
	out, err := Assemble(Overrides{}, v)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	// The house content that must never be dropped: persona, voice,
	// mechanics discipline, tool policy and session flow.
	for _, want := range []string{
		"Mythras Imperative",
		"masterful GM",
		"second person, present tense",
		"never put words in their mouth",
		"Mechanics discipline (non-negotiable)",
		"rules_lookup",
		"IMPOSSIBLE:",
		"Adjudicate → look up → resolve → narrate the facts",
		"Tool policy",
		"Session flow",
		"Greet the player and ask what kind of fantasy game they",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("default prompt missing required content %q", want)
		}
	}

	// Every template placeholder must render its Vars value...
	for _, want := range []string{
		v.SessionName, v.Recap, v.CharacterSummary, v.WorldSummary, v.Date,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered prompt does not contain %q", want)
		}
	}
	// ...and no template syntax may survive rendering.
	if strings.Contains(out, "{{") {
		t.Errorf("rendered prompt still contains template syntax:\n%s", out)
	}

	// Assembly is deterministic.
	again, err := Assemble(Overrides{}, v)
	if err != nil {
		t.Fatalf("Assemble (second call): %v", err)
	}
	if out != again {
		t.Error("Assemble output differs between identical calls")
	}
}

func TestAssembleOverridePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gm.md")
	const body = "Bespoke GM for {{.SessionName}} on {{.Date}}."
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	out, err := Assemble(Overrides{SystemPromptPath: path}, testVars())
	if err != nil {
		t.Fatalf("Assemble with override: %v", err)
	}
	want := "Bespoke GM for The Salt-Crowned Coast on 12 Fireday, 1620 ST."
	if out != want {
		t.Errorf("override output = %q, want %q", out, want)
	}
}

func TestAssembleOverrideMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.md")
	_, err := Assemble(Overrides{SystemPromptPath: path}, testVars())
	if err == nil {
		t.Fatal("Assemble with missing override succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error = %v, want wrapped fs.ErrNotExist", err)
	}
}

func TestAssembleTemplateError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.md")
	// Unknown field: parses, fails on execution.
	if err := os.WriteFile(path, []byte("{{.NotAField}}"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := Assemble(Overrides{SystemPromptPath: path}, testVars())
	if err == nil {
		t.Fatal("Assemble with bad template succeeded, want error")
	}
	if !strings.Contains(err.Error(), "render system prompt") {
		t.Errorf("error = %v, want render context", err)
	}
	if !strings.Contains(err.Error(), "NotAField") {
		t.Errorf("error = %v, want the offending field named", err)
	}
}

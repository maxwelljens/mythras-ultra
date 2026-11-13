// Package prompts assembles the game master's system prompt: an embedded
// default persona (persona, voice, mechanics discipline, tool policy,
// session context and session flow) with optional per-install overrides
// loaded from disk. Assembly is a pure function of the override and the
// [Vars], so the same inputs always produce the same prompt — golden tests
// depend on that.
//
// The package deliberately knows nothing about configuration: callers (the
// app layer) translate their config into [Overrides] and [Vars]. Tool-policy
// changes belong in default_system.md alongside the registry they describe
// (see .opencode/skills/harness-tool).
package prompts

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
	"text/template"
)

// defaultSystemPrompt is the built-in GM system prompt, embedded from
// default_system.md in this directory.
//
//go:embed default_system.md
var defaultSystemPrompt string

// Vars is the session data rendered into the system prompt template. Fields
// map one-to-one onto the template placeholders ({{.SessionName}} and kin);
// a field left empty renders as empty text, and the caller decides what
// "(none)"-style placeholder to supply for missing sections.
type Vars struct {
	// SessionName names the campaign session being played.
	SessionName string
	// Recap summarises what has happened so far, for continuity across
	// sessions and compaction.
	Recap string
	// CharacterSummary describes the player's character: concept, key
	// skills, current condition.
	CharacterSummary string
	// WorldSummary describes the world state the GM must honour: cultures,
	// factions, open threads, clocks.
	WorldSummary string
	// Date is the in-game calendar date at the start of the turn.
	Date string
}

// Overrides selects where the system prompt template comes from. The zero
// value uses the embedded default.
type Overrides struct {
	// SystemPromptPath is a path to a file on disk holding a full
	// replacement system prompt (text/template syntax, same placeholders as
	// the embedded default). When empty, the embedded default is used.
	SystemPromptPath string
}

// Assemble renders the GM system prompt: it loads the template from
// ov.SystemPromptPath when set, otherwise from the embedded default, then
// executes it with v. The output is deterministic for identical inputs.
// Errors (unreadable override, unparsable template, failed execution) are
// wrapped with the template's name and never partially rendered.
func Assemble(ov Overrides, v Vars) (string, error) {
	source, name := defaultSystemPrompt, "embedded default_system.md"
	if ov.SystemPromptPath != "" {
		name = ov.SystemPromptPath
		data, err := os.ReadFile(ov.SystemPromptPath)
		if err != nil {
			return "", fmt.Errorf("prompts: read system prompt override %q: %w", ov.SystemPromptPath, err)
		}
		source = string(data)
	}
	tmpl, err := template.New(name).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", fmt.Errorf("prompts: parse system prompt template %s: %w", name, err)
	}
	var rendered strings.Builder
	if err := tmpl.Execute(&rendered, v); err != nil {
		return "", fmt.Errorf("prompts: render system prompt %s: %w", name, err)
	}
	return rendered.String(), nil
}

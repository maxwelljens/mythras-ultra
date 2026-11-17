// Package tools is the GM-facing tool registry: it advertises the tool
// catalogue to the LLM as JSON Schemas and dispatches the model's tool calls
// against the live *resolve.State (ARCHITECTURE.md §6.1,
// docs/design/harness-surface.md — the authoritative catalogue).
//
// Tools are the only way mechanics happen (AGENTS.md golden rule 1): a
// handler never implements rules, it parses and validates arguments and then
// calls one resolve move or query. Dispatch translates every engine refusal
// and every argument-validation failure into a structured "IMPOSSIBLE: ..."
// result the GM can narrate — Go errors are reserved for unknown tool names
// and harness faults, so a single bad call never aborts the turn
// (session-loop.md, failure policy).
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/engine/resolve"
)

// ErrInvalidArgs reports a tool call whose arguments could not be used: malformed
// JSON, an unknown field, or a missing/blank required field. Handlers wrap it
// with %w and the detail message; [Registry.Dispatch] turns it into
// "IMPOSSIBLE: invalid arguments for <tool>: <detail>" so the GM can fix the
// call and retry instead of crashing the loop.
var ErrInvalidArgs = errors.New("tools: invalid arguments")

// ErrHarnessFault marks a failure of the harness itself (a broken codex
// corpus, a nil state) rather than a game-rules refusal. Handlers wrap it
// with %w; [Registry.Dispatch] propagates it as a Go error — the session loop
// reports harness faults as errors, never as IMPOSSIBLE results.
var ErrHarnessFault = errors.New("harness fault")

// Tool is one GM-facing operation: its name, the description the model sees
// (saying WHEN to use it), the strict JSON Schema for its arguments, and the
// handler that runs it against a session state.
type Tool struct {
	// Name is the registered tool name (snake_case, matching the catalogue
	// in docs/design/harness-surface.md and the backing move's Kind).
	Name string
	// Description tells the GM what the tool does, when to use it, and that
	// the returned Facts are authoritative for narration.
	Description string
	// Schema is the JSON Schema describing the tool's arguments object.
	Schema map[string]any
	// Handler executes one call. It returns a Result on success (OK true),
	// an error wrapping [ErrInvalidArgs] for unusable arguments or an
	// engine sentinel error for a rules refusal — [Registry.Dispatch]
	// performs the translation; a handler only returns [ErrHarnessFault]
	// for failures of the harness itself.
	Handler func(ctx context.Context, s *resolve.State, args json.RawMessage) (Result, error)
}

// Result is one tool outcome: {ok, summary, facts, audit} exactly as
// ARCHITECTURE.md §6.1 specifies. On failure (OK false) Summary carries the
// structured "IMPOSSIBLE: ..." refusal the GM narrates; Facts state the
// authoritative after-values the GM must never contradict.
type Result struct {
	OK      bool           `json:"ok"`
	Summary string         `json:"summary"`
	Facts   []string       `json:"facts"`
	Audit   *resolve.Audit `json:"audit,omitempty"`
}

// factsNote is appended to every tool description: the model is told, per
// tool, that the returned Facts — not its own invention — are what it
// narrates (harness-surface.md, Conventions).
const factsNote = " Result facts are authoritative for narration — never contradict them."

// fromMove projects a successful move Result onto the tool Result.
func fromMove(res resolve.Result) Result {
	return Result{OK: true, Summary: res.Summary, Facts: res.Facts, Audit: res.Audit}
}

// decodeArgs strictly decodes one call's arguments into v: malformed JSON,
// unknown fields and type mismatches become [ErrInvalidArgs] with the
// decoder's message as detail. Absent arguments are treated as {} so no-arg
// tools work when the model omits the arguments object.
func decodeArgs(raw json.RawMessage, v any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidArgs, err)
	}
	return nil
}

// requireField reports a required string argument that is missing or blank,
// as [ErrInvalidArgs] naming the field so the GM knows what to supply.
func requireField(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: missing required field %q", ErrInvalidArgs, field)
	}
	return nil
}

// requireList reports a required array argument that is missing or empty.
func requireList(field string, n int) error {
	if n == 0 {
		return fmt.Errorf("%w: missing required field %q", ErrInvalidArgs, field)
	}
	return nil
}

// engineReason renders an engine error's message as an IMPOSSIBLE reason:
// the leading engine package prefix ("resolve: ", "combat: ", …) is stripped
// so the summary reads like the catalogue's refusal format —
// e.g. ErrUnknownCharacter already lists the known IDs after its prefix.
func engineReason(err error) string {
	msg := err.Error()
	for _, p := range enginePrefixes {
		if strings.HasPrefix(msg, p) {
			return strings.TrimPrefix(msg, p)
		}
	}
	return msg
}

// enginePrefixes are the package prefixes of every engine sentinel message.
var enginePrefixes = []string{
	"resolve: ", "combat: ", "rules: ", "world: ", "char: ", "dice: ", "codex: ",
}

// Strict JSON Schema builders shared by the catalogue: every schema is an
// object with typed properties, explicit required fields and
// additionalProperties false.

// objSchema builds the top-level arguments schema for a tool.
func objSchema(props map[string]any, required ...string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	s := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// strProp builds a string property.
func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

// enumProp builds a string property restricted to the printed vocabulary.
func enumProp(desc string, values ...string) map[string]any {
	p := strProp(desc)
	p["enum"] = values
	return p
}

// intProp builds an integer property with the given inclusive minimum.
func intProp(desc string, min int) map[string]any {
	return map[string]any{"type": "integer", "description": desc, "minimum": min}
}

// boolProp builds a boolean property.
func boolProp(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

// strArrProp builds an array-of-strings property.
func strArrProp(desc string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": desc,
		"items":       map[string]any{"type": "string"},
	}
}

// arrProp builds an array property with the given item schema.
func arrProp(desc string, items map[string]any) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": desc,
		"items":       items,
	}
}

// objProp builds a nested object property with its own strict schema.
func objProp(desc string, props map[string]any, required ...string) map[string]any {
	p := objSchema(props, required...)
	p["description"] = desc
	return p
}

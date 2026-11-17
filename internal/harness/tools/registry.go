package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/maxwelljens/mythras-ultra/internal/codex"
	"github.com/maxwelljens/mythras-ultra/internal/engine/resolve"
	"github.com/maxwelljens/mythras-ultra/internal/harness/decide"
	"github.com/maxwelljens/mythras-ultra/internal/harness/llm"
)

// Registry holds the tool catalogue in registration order and dispatches the
// model's tool calls against a session state. Build one with [NewRegistry]
// before play begins: the registry is read-only afterwards — Register is not
// safe to call concurrently with Dispatch or Tools (harness-surface.md).
type Registry struct {
	tools     []Tool
	byName    map[string]Tool
	corpus    *codex.Corpus
	corpusErr error
	decide    *decide.Client
}

// NewRegistry returns a registry holding every tool in the catalogue
// (docs/design/harness-surface.md — the single source of truth for the tool
// set). decideClient powers the advisory decide tool; nil (or a client whose
// model is unavailable) degrades that one tool to an IMPOSSIBLE result while
// the rest of the registry works. The embedded rulebook corpus is loaded
// once here; a corpus failure surfaces from the rules tools as a harness
// fault rather than being swallowed.
func NewRegistry(decideClient *decide.Client) *Registry {
	corpus, err := codex.NewCorpus()
	r := &Registry{
		byName:    make(map[string]Tool),
		corpus:    corpus,
		corpusErr: err,
		decide:    decideClient,
	}
	var all []Tool
	all = append(all, rulesTools(corpus, err)...)
	all = append(all, decideTool(decideClient))
	all = append(all, diceTools()...)
	all = append(all, characterTools()...)
	all = append(all, inventoryTools()...)
	all = append(all, combatTools()...)
	all = append(all, worldTools()...)
	for _, t := range all {
		// The catalogue is static; a Register failure here is a
		// programming error the registry tests catch, not a runtime
		// condition a caller could recover from.
		if err := r.Register(t); err != nil {
			panic(err)
		}
	}
	return r
}

// Register adds one tool to the catalogue, rejecting duplicate names and
// malformed tools (empty name or description, a schema that is not an
// object JSON Schema). Registration order fixes the order of [Registry.Tools].
func (r *Registry) Register(t Tool) error {
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("tools: tool name must not be empty")
	}
	if strings.TrimSpace(t.Description) == "" {
		return fmt.Errorf("tools: tool %q: empty description", t.Name)
	}
	if t.Handler == nil {
		return fmt.Errorf("tools: tool %q: nil handler", t.Name)
	}
	if t.Schema == nil || t.Schema["type"] != "object" {
		return fmt.Errorf("tools: tool %q: schema must be an object JSON Schema", t.Name)
	}
	if _, dup := r.byName[t.Name]; dup {
		return fmt.Errorf("tools: duplicate tool %q", t.Name)
	}
	r.tools = append(r.tools, t)
	r.byName[t.Name] = t
	return nil
}

// Tools returns the catalogue as llm.ToolSpecs in registration order for
// conversation assembly; the schemas pass through to the provider unchanged.
// The returned slice is a copy; the schema maps are shared and must be
// treated as read-only.
func (r *Registry) Tools() []llm.ToolSpec {
	out := make([]llm.ToolSpec, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, llm.ToolSpec{Name: t.Name, Description: t.Description, Schema: t.Schema})
	}
	return out
}

// Names returns every registered tool name in registration order.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.Name)
	}
	return out
}

// Dispatch runs one tool call against s.
//
// Error translation (harness-surface.md, Tool contract):
//
//   - unknown tool name, nil state, or a handler error wrapping
//     [ErrHarnessFault] → a Go error; these are harness faults the session
//     loop reports as errors;
//   - a handler error wrapping [ErrInvalidArgs] → Result{OK:false} with
//     Summary "IMPOSSIBLE: invalid arguments for <tool>: <detail>";
//   - any other handler error — every engine sentinel from a move or query —
//     → Result{OK:false} with Summary "IMPOSSIBLE: <reason>", the engine's
//     own message with its package prefix stripped (resolve's
//     ErrUnknownCharacter already lists the known IDs).
//
// The GM narrates refusals; a bad call never aborts the turn.
func (r *Registry) Dispatch(ctx context.Context, s *resolve.State, name string, args json.RawMessage) (Result, error) {
	t, ok := r.byName[name]
	if !ok {
		return Result{}, fmt.Errorf("tools: unknown tool %q (known: %s)", name, strings.Join(r.Names(), ", "))
	}
	if s == nil {
		return Result{}, fmt.Errorf("%w: nil *resolve.State", ErrHarnessFault)
	}
	res, err := t.Handler(ctx, s, args)
	if err == nil {
		return res, nil
	}
	switch {
	case errors.Is(err, ErrInvalidArgs):
		detail := strings.TrimPrefix(err.Error(), ErrInvalidArgs.Error())
		detail = strings.TrimPrefix(detail, ": ")
		return Result{OK: false, Summary: "IMPOSSIBLE: invalid arguments for " + name + ": " + detail}, nil
	case errors.Is(err, ErrHarnessFault):
		return Result{}, fmt.Errorf("tools: %s: %w", name, err)
	default:
		return Result{OK: false, Summary: "IMPOSSIBLE: " + engineReason(err)}, nil
	}
}

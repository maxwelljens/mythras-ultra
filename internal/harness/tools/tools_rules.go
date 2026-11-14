package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/mythras-ultra/myul/internal/codex"
	"github.com/mythras-ultra/myul/internal/engine/resolve"
	"github.com/mythras-ultra/myul/internal/harness/decide"
)

// unavailableText is the decide-tool refusal the GM narrates when the
// optional decision model cannot be consulted — decide is advisory by
// design, so its absence falls back to ordinary GM judgement
// (session-loop.md, failure policy).
const unavailableText = "IMPOSSIBLE: decision model unavailable — decide by the rules instead"

// rulesTools returns the rulebook lookup tools over the embedded corpus.
func rulesTools(corpus *codex.Corpus, corpusErr error) []Tool {
	return []Tool{
		rulesLookupTool(corpus, corpusErr),
		rulesSectionTool(corpus, corpusErr),
	}
}

// corpusFault converts a corpus load failure into the harness fault
// [Registry.Dispatch] propagates as a Go error instead of an IMPOSSIBLE
// result: a broken embed is the harness's fault, not a game-rules refusal.
func corpusFault(corpusErr error) error {
	return fmt.Errorf("%w: codex corpus unavailable: %w", ErrHarnessFault, corpusErr)
}

// rulesLookupTool builds the rules_lookup tool (codex.Search).
func rulesLookupTool(corpus *codex.Corpus, corpusErr error) Tool {
	return Tool{
		Name: "rules_lookup",
		Description: "Search the embedded rulebook for a rules question. " +
			"Use whenever a ruling is in doubt, before ruling from memory; " +
			"the top hits with snippets come back as facts." + factsNote,
		Schema: objSchema(map[string]any{
			"query": strProp("Whitespace-separated search terms, e.g. \"opposed rolls\"."),
			"limit": intProp("Maximum sections to return (default 8, capped at 50).", 1),
		}, "query"),
		Handler: func(_ context.Context, _ *resolve.State, args json.RawMessage) (Result, error) {
			if corpusErr != nil {
				return Result{}, corpusFault(corpusErr)
			}
			var a struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := decodeArgs(args, &a); err != nil {
				return Result{}, err
			}
			if err := requireField("query", a.Query); err != nil {
				return Result{}, err
			}
			hits := corpus.Search(a.Query, a.Limit)
			facts := make([]string, 0, len(hits)+1)
			for i, h := range hits {
				facts = append(facts, fmt.Sprintf("%d. %s (score %.2f): %s", i+1, h.Section.Path, h.Score, h.Snippet))
			}
			if len(hits) == 0 {
				facts = append(facts, "no sections matched — rephrase the query with rulebook terms")
			}
			return Result{
				OK:      true,
				Summary: fmt.Sprintf("%d section(s) matched %q", len(hits), a.Query),
				Facts:   facts,
			}, nil
		},
	}
}

// rulesSectionTool builds the rules_section tool (full text of one
// codex.Section, found by document and heading).
func rulesSectionTool(corpus *codex.Corpus, corpusErr error) Tool {
	return Tool{
		Name: "rules_section",
		Description: "Read one full rulebook section by document and heading. " +
			"Use when a rules_lookup hit needs the complete text; find headings " +
			"with rules_lookup first." + factsNote,
		Schema: objSchema(map[string]any{
			"doc":     strProp("Document file name, e.g. \"Mythras-Imperative.md\" (see rules_lookup hit paths)."),
			"heading": strProp("Exact section heading, e.g. \"Dice Used\"."),
		}, "doc", "heading"),
		Handler: func(_ context.Context, _ *resolve.State, args json.RawMessage) (Result, error) {
			if corpusErr != nil {
				return Result{}, corpusFault(corpusErr)
			}
			var a struct {
				Doc     string `json:"doc"`
				Heading string `json:"heading"`
			}
			if err := decodeArgs(args, &a); err != nil {
				return Result{}, err
			}
			if err := requireField("doc", a.Doc); err != nil {
				return Result{}, err
			}
			if err := requireField("heading", a.Heading); err != nil {
				return Result{}, err
			}
			docs := corpus.Documents()
			if !slices.Contains(docs, a.Doc) {
				return Result{}, fmt.Errorf("%w: unknown document %q (known: %s)",
					ErrInvalidArgs, a.Doc, strings.Join(docs, ", "))
			}
			want := strings.TrimSpace(a.Heading)
			var match *codex.Section
			for _, sec := range corpus.Sections() {
				if sec.Doc == a.Doc && strings.EqualFold(strings.TrimSpace(sec.Heading), want) {
					// Duplicate headings share a slug; the first
					// occurrence is the rulebook's own order.
					s := sec
					match = &s
					break
				}
			}
			if match == nil {
				return Result{}, fmt.Errorf("%w: no section %q in %s — use rules_lookup to find headings",
					ErrInvalidArgs, a.Heading, a.Doc)
			}
			return Result{
				OK:      true,
				Summary: fmt.Sprintf("section %q (%s), %d characters", match.Heading, match.Doc, len(match.Text)),
				Facts: []string{
					"doc: " + match.Doc,
					"heading: " + match.Heading,
					"path: " + match.Path,
					"text:\n" + match.Text,
				},
			}, nil
		},
	}
}

// decideTool builds the advisory decide tool. The tool degrades to an
// IMPOSSIBLE result when no decision client is wired or the model cannot be
// consulted — a verdict is a hint for the GM, never a mechanical outcome
// (decide package contract; every verdict carries "advisory: true").
func decideTool(dc *decide.Client) Tool {
	return Tool{
		Name: "decide",
		Description: "Ask the optional decision model a multiple-choice question. " +
			"Use for cheap advisory judgements only — does this action need a test, " +
			"which skill fits, what is the NPC's motive — never for a mechanical " +
			"outcome; when unavailable, decide by the rules yourself." + factsNote,
		Schema: objSchema(map[string]any{
			"question": strProp("The advisory question to judge, e.g. \"Which skill is tested?\"."),
			"options":  strArrProp("Candidate answers — at least two, non-blank and unique (2–24 options)."),
		}, "question", "options"),
		Handler: func(ctx context.Context, s *resolve.State, args json.RawMessage) (Result, error) {
			var a struct {
				Question string   `json:"question"`
				Options  []string `json:"options"`
			}
			if err := decodeArgs(args, &a); err != nil {
				return Result{}, err
			}
			if err := requireField("question", a.Question); err != nil {
				return Result{}, err
			}
			if err := requireList("options", len(a.Options)); err != nil {
				return Result{}, err
			}
			seen := make(map[string]bool, len(a.Options))
			for i, opt := range a.Options {
				if strings.TrimSpace(opt) == "" {
					return Result{}, fmt.Errorf("%w: options[%d] is blank", ErrInvalidArgs, i)
				}
				if seen[opt] {
					return Result{}, fmt.Errorf("%w: duplicate option %q", ErrInvalidArgs, opt)
				}
				seen[opt] = true
			}
			if dc == nil {
				return Result{OK: false, Summary: unavailableText}, nil
			}
			v, err := dc.Decide(ctx, decide.Question{
				State:    decideState(s),
				Question: a.Question,
				Options:  a.Options,
			})
			if err != nil {
				if errors.Is(err, decide.ErrBadResponse) {
					return Result{OK: false, Summary: "IMPOSSIBLE: decision model gave an unusable verdict — decide by the rules instead"}, nil
				}
				// ErrUnavailable (disabled, Ollama down, model not
				// pulled) and transport failures: decide is optional.
				return Result{OK: false, Summary: unavailableText}, nil
			}
			facts := []string{
				"advisory: true",
				"choice: " + v.Choice,
				fmt.Sprintf("confidence: %.2f", v.Confidence),
			}
			if v.Reasons != "" {
				facts = append(facts, "reasons: "+v.Reasons)
			}
			if v.Cached {
				facts = append(facts, "cached: true")
			}
			if v.Model != "" {
				facts = append(facts, "model: "+v.Model)
			}
			return Result{
				OK:      true,
				Summary: fmt.Sprintf("decide: %s (confidence %.2f) — advisory only", v.Choice, v.Confidence),
				Facts:   facts,
			}, nil
		},
	}
}

// decideState renders the short game context the decision model judges —
// the /v1/systemone "state" field needs something non-blank, so the session
// name and current scene stand in for the conversation.
func decideState(s *resolve.State) string {
	var parts []string
	if s.Name != "" {
		parts = append(parts, "session "+s.Name)
	}
	if s.Scene.Location != "" {
		parts = append(parts, "scene: "+s.Scene.Location)
	}
	if len(parts) == 0 {
		return "mythras ultra session"
	}
	return strings.Join(parts, "; ")
}

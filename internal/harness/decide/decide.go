// Package decide is the harness's optional "System 1" decision layer:
// cheap, local adjudication of advisory multiple-choice questions through the
// tev1 model served by Ollama's dedicated POST /v1/systemone endpoint
// (docs/research/llm-stack.md §3 — /api/chat is unsuitable for tev1, which
// replies in prose there).
//
// Everything this package returns is advisory. The GM decides, and no engine
// move may depend on a verdict (AGENTS.md golden rule 1). When Ollama, the
// model, or configuration is absent, calls fail with [ErrUnavailable] and the
// harness falls back to ordinary GM judgement.
package decide

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// Question is one multiple-choice question for the decision model.
type Question struct {
	// State is the short game context the model judges (what Ollama's
	// /v1/systemone calls "state"). It must not be blank; if a batch
	// grows past the 64 KiB request budget it is truncated to fit.
	State string
	// Question is the question text, sent as the choice question's
	// instructions.
	Question string
	// Options are the candidate answers — for example skill names, or
	// "yes"/"no". At least two, non-blank, and unique. tev1 was trained
	// on 2–24 options, so keep batches inside that range.
	Options []string
}

// Verdict is the decision model's answer to one [Question].
type Verdict struct {
	// Choice is the chosen option, verbatim one of [Question.Options].
	Choice string
	// Confidence is the model's distribution-concentration score from 0
	// to 1. Per the Ollama model card it is not the probability that
	// the answer is right.
	Confidence float64
	// Advisory is always true. A verdict is a hint the GM may accept or
	// override; engine determinism must never depend on a model
	// (AGENTS.md golden rule 1), so no caller may treat a verdict as a
	// mechanical outcome.
	Advisory bool
	// Reasons summarises the model's option probabilities, highest
	// first. The /v1/systemone API emits no free-text rationale, so
	// this is the only "why" available; it may be empty.
	Reasons string
	// Cached reports that the verdict came from the session cache
	// instead of a fresh model call.
	Cached bool
	// Model names the model that produced the verdict: the response's
	// model field, falling back to the configured model.
	Model string
}

// ErrUnavailable reports that the decision model could not be consulted:
// decide is disabled, Ollama is unreachable, /v1/systemone is missing (Ollama
// older than v0.35.0), or the configured model is not pulled. Callers should
// fall back to GM judgement and treat "no verdict" as the normal case.
var ErrUnavailable = errors.New("decide: decision model unavailable")

// ErrBadResponse reports that the decision model returned output this package
// cannot trust: malformed JSON, missing or malformed answers, a choice
// outside the offered options, or a response over the 64 KiB read cap.
var ErrBadResponse = errors.New("decide: bad decision-model response")

const (
	// maxQuestionsPerRequest is the /v1/systemone limit on named
	// questions per request; DecideBatch splits larger batches into
	// chunks of at most this many.
	maxQuestionsPerRequest = 64
	// maxRequestBytes is the 64 KiB limit on local request text and
	// schema; marshalSystemOne truncates the state to fit it.
	maxRequestBytes = 64 << 10
	// maxResponseBytes bounds how much of a response is read (64 KiB);
	// larger responses are rejected with [ErrBadResponse] instead of
	// being buffered.
	maxResponseBytes = 64 << 10
)

// Decide asks the decision model one question and returns its [Verdict].
// The verdict is always advisory ([Verdict.Advisory] is true). A repeated
// question is answered from the session cache with [Verdict.Cached] set.
//
// It returns [ErrUnavailable] when the decision model cannot be consulted
// and [ErrBadResponse] when its output is unusable; in both cases the
// returned verdict is the zero value.
func (c *Client) Decide(ctx context.Context, q Question) (Verdict, error) {
	verdicts, err := c.DecideBatch(ctx, []Question{q})
	if err != nil {
		return Verdict{}, err
	}
	return verdicts[0], nil
}

// DecideBatch adjudicates a batch of questions, returning one [Verdict] per
// question in input order. Questions are validated up front, served from the
// session cache when possible, and otherwise sent in consecutive runs sharing
// the same [Question.State] — one state per /v1/systemone request — split
// into chunks of at most 64 questions (the endpoint's cap), so batches of
// any size succeed. Questions whose text and options alone exceed the 64 KiB
// request budget fail with an error instead.
//
// The batch is all-or-nothing: on error no verdicts are returned, although
// verdicts fetched before the failure stay cached. An empty batch returns an
// empty slice and no error.
func (c *Client) DecideBatch(ctx context.Context, qs []Question) ([]Verdict, error) {
	if !c.cfg.Enabled {
		return nil, fmt.Errorf("%w: disabled in configuration", ErrUnavailable)
	}
	verdicts := make([]Verdict, len(qs))
	for i, q := range qs {
		if err := validateQuestion(q); err != nil {
			return nil, fmt.Errorf("decide: question %d: %w", i, err)
		}
	}
	var chunks []chunk
	for i, q := range qs {
		if v, ok := c.cache.get(cacheKey(q)); ok {
			verdicts[i] = v
			continue
		}
		if n := len(chunks); n > 0 &&
			chunks[n-1].state == q.State &&
			len(chunks[n-1].idxs) < maxQuestionsPerRequest {
			chunks[n-1].idxs = append(chunks[n-1].idxs, i)
		} else {
			chunks = append(chunks, chunk{state: q.State, idxs: []int{i}})
		}
	}
	for _, ch := range chunks {
		if err := c.ask(ctx, ch, qs, verdicts); err != nil {
			return nil, err
		}
	}
	return verdicts, nil
}

// Healthy probes the decision service: it fetches Ollama's model list from
// the configured base URL and checks that the configured model is present.
// It returns nil when healthy, [ErrUnavailable] otherwise, or the caller's
// own context error when the probe was cancelled.
func (c *Client) Healthy(ctx context.Context) error {
	if !c.cfg.Enabled {
		return fmt.Errorf("%w: disabled in configuration", ErrUnavailable)
	}
	data, status, err := c.roundTrip(ctx, http.MethodGet, "/api/tags", nil)
	if err != nil {
		if ctx.Err() == nil && !errors.Is(err, ErrUnavailable) {
			return fmt.Errorf("%w: probe model list: %w", ErrUnavailable, err)
		}
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("%w: GET %s/api/tags: status %d", ErrUnavailable, c.baseURL, status)
	}
	var tags struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &tags); err != nil {
		return fmt.Errorf("%w: parse model list: %w", ErrUnavailable, err)
	}
	names := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		if m.Name != "" {
			names = append(names, m.Name)
		} else {
			names = append(names, m.Model)
		}
	}
	if !modelPresent(c.cfg.Model, names) {
		available := "none"
		if len(names) > 0 {
			available = strings.Join(names, ", ")
		}
		return fmt.Errorf("%w: model %q is not present at %s (pull it with `ollama pull %s`); available: %s",
			ErrUnavailable, c.cfg.Model, c.baseURL, c.cfg.Model, available)
	}
	return nil
}

// chunk is a run of pending questions that share one request state.
type chunk struct {
	state string
	idxs  []int
}

// ask sends one chunk, writes each parsed verdict into verdicts at its
// original index, and stores it in the session cache.
func (c *Client) ask(ctx context.Context, ch chunk, qs []Question, verdicts []Verdict) error {
	req := systemOneRequest{
		Model:     c.cfg.Model,
		State:     ch.state,
		Questions: make(map[string]systemOneQuestion, len(ch.idxs)),
	}
	labels := make(map[string]int, len(ch.idxs))
	for n, idx := range ch.idxs {
		label := "q" + strconv.Itoa(n)
		req.Questions[label] = choiceQuestion(qs[idx])
		labels[label] = idx
	}
	resp, err := c.postSystemOne(ctx, req)
	if err != nil {
		return err
	}
	model := resp.Model
	if model == "" {
		model = c.cfg.Model
	}
	for label, idx := range labels {
		ans, ok := resp.Answers[label]
		if !ok {
			return fmt.Errorf("%w: response has no answer for question %q", ErrBadResponse, label)
		}
		v, err := verdictFrom(label, model, ans, qs[idx])
		if err != nil {
			return err
		}
		verdicts[idx] = v
		c.cache.put(cacheKey(qs[idx]), v)
	}
	return nil
}

// verdictFrom converts one /v1/systemone choice answer into a [Verdict],
// rejecting answers that do not match the question that was asked.
func verdictFrom(label, model string, ans systemOneAnswer, q Question) (Verdict, error) {
	if ans.Type != choiceType {
		return Verdict{}, fmt.Errorf("%w: answer %q has type %q, want %q", ErrBadResponse, label, ans.Type, choiceType)
	}
	if !slices.Contains(q.Options, ans.Choice) {
		return Verdict{}, fmt.Errorf("%w: answer %q chose %q, which is not among the offered options", ErrBadResponse, label, ans.Choice)
	}
	return Verdict{
		Choice:     ans.Choice,
		Confidence: ans.Confidence,
		Advisory:   true,
		Reasons:    formatProbabilities(ans.Probabilities),
		Model:      model,
	}, nil
}

// formatProbabilities renders an answer's option probabilities, highest
// first — for example "bug 97.81%, billing 1.25%". It returns an empty
// string when the response carries no probabilities.
func formatProbabilities(probs map[string]float64) string {
	if len(probs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(probs))
	for k := range probs {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		if c := cmp.Compare(probs[b], probs[a]); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(k)
		b.WriteString(" ")
		b.WriteString(strconv.FormatFloat(probs[k]*100, 'f', 2, 64))
		b.WriteString("%")
	}
	return b.String()
}

// validateQuestion rejects questions the /v1/systemone contract would
// refuse, before any network call.
func validateQuestion(q Question) error {
	if strings.TrimSpace(q.State) == "" {
		return errors.New("state must not be blank")
	}
	if strings.TrimSpace(q.Question) == "" {
		return errors.New("question must not be blank")
	}
	if len(q.Options) < 2 {
		return fmt.Errorf("need at least 2 options, got %d", len(q.Options))
	}
	seen := make(map[string]struct{}, len(q.Options))
	for i, opt := range q.Options {
		if strings.TrimSpace(opt) == "" {
			return fmt.Errorf("option %d is blank", i)
		}
		if _, dup := seen[opt]; dup {
			return fmt.Errorf("option %q is repeated", opt)
		}
		seen[opt] = struct{}{}
	}
	return nil
}

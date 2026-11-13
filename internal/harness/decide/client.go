package decide

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config configures a [Client]. Field defaults mirror internal/config's
// [decision] section (enabled, URL, model); this package deliberately does not
// import it — the application layer wires configuration in.
type Config struct {
	// Enabled turns the decision model on. When false, every call fails
	// immediately with [ErrUnavailable] and no network traffic happens.
	// The application's config default is true.
	Enabled bool
	// URL is the Ollama base URL. Empty means http://localhost:11434.
	URL string
	// Model is the decision model's tag. Empty means "tev1:4b".
	Model string
	// Timeout bounds each HTTP round trip. Zero means 10s.
	Timeout time.Duration
}

// Client talks to the tev1 decision model through Ollama's /v1/systemone
// endpoint. It is safe for concurrent use; verdicts are memoised in a
// bounded, session-scoped cache (see [New]). Create one with [New].
type Client struct {
	cfg     Config
	baseURL string
	http    *http.Client
	cache   *cache
}

// New returns a decision client for cfg, applying defaults for zero fields:
// URL http://localhost:11434, model "tev1:4b", and a 10s timeout — the same
// defaults internal/config gives the [decision] section. Enabled is taken as
// given (the config layer defaults it to true).
func New(cfg Config) *Client {
	if cfg.URL == "" {
		cfg.URL = "http://localhost:11434"
	}
	if cfg.Model == "" {
		cfg.Model = "tev1:4b"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Client{
		cfg:     cfg,
		baseURL: strings.TrimRight(cfg.URL, "/"),
		http:    &http.Client{Timeout: cfg.Timeout},
		cache:   newCache(cacheMaxEntries),
	}
}

// choiceType is the /v1/systemone question and answer type this package
// uses; every question is sent as a "choice" question, including yes/no
// pairs (two bare-label options).
const choiceType = "choice"

// systemOneRequest mirrors the POST /v1/systemone request schema, verified
// against https://docs.ollama.com/api/systemone: the model, one shared state,
// and 1–64 named questions.
type systemOneRequest struct {
	Model     string                       `json:"model"`
	State     string                       `json:"state"`
	Questions map[string]systemOneQuestion `json:"questions"`
}

// systemOneQuestion is a "choice" question: instructions plus criteria
// mapping each option key to a description (null selects a bare label).
type systemOneQuestion struct {
	Type         string             `json:"type"`
	Instructions string             `json:"instructions"`
	Criteria     map[string]*string `json:"criteria"`
}

// systemOneResponse mirrors the /v1/systemone success response: the answering
// model and the answers keyed by the request's question names. The documented
// "usage" block is not needed here.
type systemOneResponse struct {
	Model   string                     `json:"model"`
	Answers map[string]systemOneAnswer `json:"answers"`
}

// systemOneAnswer is one choice answer: the winning option key with its
// probabilities and concentration confidence (docs.ollama.com/api/systemone).
type systemOneAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

// choiceQuestion shapes q as a /v1/systemone choice question. Options are
// offered as bare labels (criteria null): this package has no per-option
// descriptions, and each option key is already the full answer text.
func choiceQuestion(q Question) systemOneQuestion {
	criteria := make(map[string]*string, len(q.Options))
	for _, opt := range q.Options {
		criteria[opt] = nil
	}
	return systemOneQuestion{
		Type:         choiceType,
		Instructions: q.Question,
		Criteria:     criteria,
	}
}

// postSystemOne encodes and sends one /v1/systemone request and decodes its
// response. Non-2xx responses become errors carrying the server's message;
// a 404 maps to [ErrUnavailable] because it means tev1's dedicated endpoint
// (or the model) is missing, not that the question was bad.
func (c *Client) postSystemOne(ctx context.Context, req systemOneRequest) (systemOneResponse, error) {
	body, err := marshalSystemOne(req)
	if err != nil {
		return systemOneResponse{}, err
	}
	data, status, err := c.roundTrip(ctx, http.MethodPost, "/v1/systemone", body)
	if err != nil {
		return systemOneResponse{}, err
	}
	if status < 200 || status >= 300 {
		msg := serverErrorMessage(data)
		if status == http.StatusNotFound {
			return systemOneResponse{}, fmt.Errorf(
				"%w: POST %s/v1/systemone is missing (Ollama older than v0.35.0, not an Ollama server, or model %q not pulled): %s",
				ErrUnavailable, c.baseURL, c.cfg.Model, msg)
		}
		return systemOneResponse{}, fmt.Errorf("decide: POST /v1/systemone: status %d: %s", status, msg)
	}
	var out systemOneResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return systemOneResponse{}, fmt.Errorf("%w: parse response: %w (body: %.200q)", ErrBadResponse, err, data)
	}
	if len(out.Answers) == 0 {
		if msg := serverErrorMessage(data); msg != "" {
			return systemOneResponse{}, fmt.Errorf("%w: server error: %s", ErrBadResponse, msg)
		}
		return systemOneResponse{}, fmt.Errorf("%w: response carries no answers (body: %.200q)", ErrBadResponse, data)
	}
	return out, nil
}

// marshalSystemOne encodes req, truncating the shared state on rune
// boundaries when the request text and schema would exceed the 64 KiB the
// endpoint allows — state is the only field shortening cannot silently
// change a question's meaning. If it still does not fit, it returns an
// error rather than send a request the server must reject.
func marshalSystemOne(req systemOneRequest) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("decide: encode request: %w", err)
	}
	if len(body) > maxRequestBytes {
		runes := []rune(req.State)
		for len(body) > maxRequestBytes && len(runes) > 0 {
			runes = runes[:len(runes)-min(len(body)-maxRequestBytes, len(runes))]
			req.State = string(runes)
			if body, err = json.Marshal(req); err != nil {
				return nil, fmt.Errorf("decide: encode request: %w", err)
			}
		}
		if strings.TrimSpace(req.State) == "" {
			// SystemOneContent must be a non-blank string; leave a
			// placeholder when truncation empties the state.
			req.State = "."
			if body, err = json.Marshal(req); err != nil {
				return nil, fmt.Errorf("decide: encode request: %w", err)
			}
		}
		if len(body) > maxRequestBytes {
			return nil, fmt.Errorf("decide: question text and options exceed %d bytes even with the state truncated", maxRequestBytes)
		}
	}
	return body, nil
}

// roundTrip sends one request to path on the configured base URL and returns
// the bounded response body and status code; callers interpret the status.
// Transport failures wrap [ErrUnavailable] unless the caller's own context
// ended first, in which case the context error propagates so cancellation
// stays distinguishable from an absent decision model.
func (c *Client) roundTrip(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("decide: build request for %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, 0, fmt.Errorf("%s %s: %w", method, path, err)
		}
		return nil, 0, fmt.Errorf("%w: %s %s: %w", ErrUnavailable, method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: read %s %s response: %w", ErrUnavailable, method, path, err)
	}
	if len(data) > maxResponseBytes {
		return nil, 0, fmt.Errorf("%w: %s %s response exceeds %d bytes", ErrBadResponse, method, path, maxResponseBytes)
	}
	return data, resp.StatusCode, nil
}

// serverErrorMessage extracts Ollama's {"error": "..."} message from a
// response body, falling back to a truncated body snippet for anything else.
func serverErrorMessage(data []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &e); err == nil && e.Error != "" {
		return e.Error
	}
	const maxSnippet = 200
	snippet := strings.TrimSpace(string(data))
	if r := []rune(snippet); len(r) > maxSnippet {
		snippet = string(r[:maxSnippet]) + "…"
	}
	if snippet == "" {
		return "empty response body"
	}
	return snippet
}

// modelPresent reports whether the configured model tag appears in Ollama's
// tag list, tolerating the untagged form (an omitted tag means :latest).
func modelPresent(configured string, names []string) bool {
	want := configured
	if !strings.Contains(want, ":") {
		want += ":latest"
	}
	for _, n := range names {
		if n == configured || n == want {
			return true
		}
	}
	return false
}

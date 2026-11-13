package decide

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// writeJSON writes v as a JSON response, ignoring the (unused) encode error.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// decodeSystemOneRequest decodes the request body into req, reporting
// success. On failure it answers 400 so the client unblocks.
func decodeSystemOneRequest(t *testing.T, w http.ResponseWriter, r *http.Request) (systemOneRequest, bool) {
	t.Helper()
	var req systemOneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.Errorf("decode /v1/systemone request: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return systemOneRequest{}, false
	}
	return req, true
}

// writeSystemOneAnswer answers every question in req with its "unique-"-prefixed
// option, mirroring the documented /v1/systemone success shape.
func writeSystemOneAnswer(t *testing.T, w http.ResponseWriter, req systemOneRequest) {
	t.Helper()
	answers := make(map[string]any, len(req.Questions))
	for label, q := range req.Questions {
		pick := ""
		for opt := range q.Criteria {
			if strings.HasPrefix(opt, "unique-") {
				pick = opt
				break
			}
		}
		if pick == "" {
			t.Errorf("question %q has no unique- option among its criteria", label)
		}
		answers[label] = map[string]any{
			"type":          choiceType,
			"choice":        pick,
			"probabilities": map[string]float64{pick: 1},
			"confidence":    1,
		}
	}
	writeJSON(w, map[string]any{
		"model":   "tev1:4b",
		"answers": answers,
		"usage":   map[string]int{"input_tokens": 1, "output_tokens": 1},
	})
}

// testQuestion builds question i in the given state with options the fake
// server can recognise per index.
func testQuestion(i int, state string) Question {
	return Question{
		State:    state,
		Question: fmt.Sprintf("Which option fits step %d?", i),
		Options:  []string{fmt.Sprintf("unique-%d", i), "fallback"},
	}
}

// newTestClient wires an enabled client at srv's URL with the default model.
func newTestClient(srv *httptest.Server) *Client {
	return New(Config{Enabled: true, URL: srv.URL})
}

// TestDecideHappyPath pins the exact tev1 /v1/systemone request shape
// (docs.ollama.com/api/systemone) and the parsing of its documented response.
func TestDecideHappyPath(t *testing.T) {
	q := Question{
		State:    "The thief eyes the jewelled idol while the guard sleeps.",
		Question: "Which skill is tested?",
		Options:  []string{"Stealthy", "Battle Dance"},
	}
	var mu sync.Mutex
	var got systemOneRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			t.Errorf("request = %s %s, want POST /v1/systemone", r.Method, r.URL.Path)
		}
		if ct, a := r.Header.Get("Content-Type"), r.Header.Get("Accept"); ct != "application/json" || a != "application/json" {
			t.Errorf("headers Content-Type=%q Accept=%q, want application/json both", ct, a)
		}
		req, ok := decodeSystemOneRequest(t, w, r)
		if !ok {
			return
		}
		mu.Lock()
		got = req
		mu.Unlock()
		writeJSON(w, map[string]any{
			"model": "tev1:4b",
			"answers": map[string]any{
				"q0": map[string]any{
					"type":          choiceType,
					"choice":        "Stealthy",
					"probabilities": map[string]float64{"Stealthy": 0.75, "Battle Dance": 0.25},
					"confidence":    0.625,
				},
			},
			"usage": map[string]int{"input_tokens": 42, "output_tokens": 1},
		})
	}))
	t.Cleanup(srv.Close)

	v, err := newTestClient(srv).Decide(t.Context(), q)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if v.Choice != "Stealthy" {
		t.Errorf("Choice = %q, want Stealthy", v.Choice)
	}
	if v.Confidence != 0.625 {
		t.Errorf("Confidence = %v, want 0.625", v.Confidence)
	}
	if v.Model != "tev1:4b" {
		t.Errorf("Model = %q, want tev1:4b", v.Model)
	}
	if !v.Advisory {
		t.Error("Advisory = false, must always be true: the GM decides, never the model")
	}
	if v.Cached {
		t.Error("first verdict must not be cached")
	}
	if want := "Stealthy 75.00%, Battle Dance 25.00%"; v.Reasons != want {
		t.Errorf("Reasons = %q, want %q", v.Reasons, want)
	}

	mu.Lock()
	defer mu.Unlock()
	if got.Model != "tev1:4b" {
		t.Errorf("request model = %q, want tev1:4b", got.Model)
	}
	if got.State != q.State {
		t.Errorf("request state = %q, want %q", got.State, q.State)
	}
	if len(got.Questions) != 1 {
		t.Fatalf("request has %d questions, want 1", len(got.Questions))
	}
	inner, ok := got.Questions["q0"]
	if !ok {
		t.Fatalf("question keys = %v, want [q0]", keysOf(got.Questions))
	}
	if inner.Type != choiceType || inner.Instructions != q.Question {
		t.Errorf("question = {type:%q instructions:%q}, want {%q %q}", inner.Type, inner.Instructions, choiceType, q.Question)
	}
	if len(inner.Criteria) != len(q.Options) {
		t.Errorf("criteria has %d entries, want %d", len(inner.Criteria), len(q.Options))
	}
	for _, opt := range q.Options {
		crit, ok := inner.Criteria[opt]
		if !ok {
			t.Errorf("criteria missing option %q", opt)
			continue
		}
		if crit != nil {
			t.Errorf("criteria[%q] = %q, want null (bare label)", opt, *crit)
		}
	}
}

// keysOf returns the sorted keys of a question map for test messages.
func keysOf(m map[string]systemOneQuestion) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestDecideBatchChunks verifies the 64-question cap is honoured by chunking:
// consecutive same-state runs are sent as separate requests, never over 64.
func TestDecideBatchChunks(t *testing.T) {
	sameState := make([]Question, 130)
	for i := range sameState {
		sameState[i] = testQuestion(i, "the vault")
	}
	interleaved := []Question{
		testQuestion(0, "A"), testQuestion(1, "A"),
		testQuestion(2, "B"),
		testQuestion(3, "A"),
	}
	tests := []struct {
		name       string
		qs         []Question
		wantCounts []int
		wantStates []string
	}{
		{
			name:       "130 same-state questions split into 64, 64, 2",
			qs:         sameState,
			wantCounts: []int{64, 64, 2},
			wantStates: []string{"the vault", "the vault", "the vault"},
		},
		{
			name:       "a state change starts a new request run",
			qs:         interleaved,
			wantCounts: []int{2, 1, 1},
			wantStates: []string{"A", "B", "A"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var counts []int
			var states []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				req, ok := decodeSystemOneRequest(t, w, r)
				if !ok {
					return
				}
				mu.Lock()
				counts = append(counts, len(req.Questions))
				states = append(states, req.State)
				mu.Unlock()
				writeSystemOneAnswer(t, w, req)
			}))
			t.Cleanup(srv.Close)

			verdicts, err := newTestClient(srv).DecideBatch(t.Context(), tt.qs)
			if err != nil {
				t.Fatalf("DecideBatch: %v", err)
			}
			if len(verdicts) != len(tt.qs) {
				t.Fatalf("got %d verdicts, want %d", len(verdicts), len(tt.qs))
			}
			for i, v := range verdicts {
				if want := fmt.Sprintf("unique-%d", i); v.Choice != want {
					t.Errorf("verdict %d choice = %q, want %q (input order must be preserved)", i, v.Choice, want)
				}
				if v.Cached {
					t.Errorf("verdict %d unexpectedly cached", i)
				}
				if !v.Advisory {
					t.Errorf("verdict %d Advisory = false, must be true", i)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if fmt.Sprint(counts) != fmt.Sprint(tt.wantCounts) {
				t.Errorf("request question counts = %v, want %v", counts, tt.wantCounts)
			}
			if fmt.Sprint(states) != fmt.Sprint(tt.wantStates) {
				t.Errorf("request states = %v, want %v", states, tt.wantStates)
			}
			for _, n := range counts {
				if n > maxQuestionsPerRequest {
					t.Errorf("request carries %d questions, over the %d cap", n, maxQuestionsPerRequest)
				}
			}
		})
	}
}

// TestDecideBatchEmpty documents that an empty batch is a no-op.
func TestDecideBatchEmpty(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writeJSON(w, map[string]any{})
	}))
	t.Cleanup(srv.Close)

	verdicts, err := newTestClient(srv).DecideBatch(t.Context(), nil)
	if err != nil {
		t.Fatalf("DecideBatch(nil): %v", err)
	}
	if len(verdicts) != 0 {
		t.Errorf("got %d verdicts, want 0", len(verdicts))
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("server called %d times, want 0", got)
	}
}

// TestDisabledClientNoNetwork checks a disabled client fails with
// ErrUnavailable without ever touching the network.
func TestDisabledClientNoNetwork(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		t.Errorf("disabled client touched the network: %s %s", r.Method, r.URL.Path)
		writeJSON(w, map[string]any{})
	}))
	t.Cleanup(srv.Close)

	c := New(Config{Enabled: false, URL: srv.URL})
	if _, err := c.Decide(t.Context(), testQuestion(0, "s")); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Decide error = %v, want ErrUnavailable", err)
	}
	if _, err := c.DecideBatch(t.Context(), []Question{testQuestion(0, "s")}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("DecideBatch error = %v, want ErrUnavailable", err)
	}
	if err := c.Healthy(t.Context()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Healthy error = %v, want ErrUnavailable", err)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("server called %d times, want 0", got)
	}
}

// TestDecideConnectionRefused maps an unreachable Ollama to ErrUnavailable.
func TestDecideConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not be called")
		writeJSON(w, map[string]any{})
	}))
	url := srv.URL
	srv.Close()

	_, err := newTestClientAt(url).Decide(t.Context(), testQuestion(0, "s"))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Decide error = %v, want ErrUnavailable", err)
	}
}

// newTestClientAt wires an enabled client at an explicit URL.
func newTestClientAt(url string) *Client {
	return New(Config{Enabled: true, URL: url})
}

// TestDecideCancelledContext checks caller cancellation propagates as a
// context error rather than being mislabelled as an absent model.
func TestDecideCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{})
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := newTestClient(srv).Decide(ctx, testQuestion(0, "s"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Decide error = %v, want context.Canceled", err)
	}
	if errors.Is(err, ErrUnavailable) {
		t.Errorf("cancellation must not be reported as ErrUnavailable: %v", err)
	}
}

// TestCacheServesRepeatedQuestion: an identical question reaches the server
// once; the repeat comes back cached.
func TestCacheServesRepeatedQuestion(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		req, ok := decodeSystemOneRequest(t, w, r)
		if !ok {
			return
		}
		writeSystemOneAnswer(t, w, req)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(srv)
	q := testQuestion(7, "the cellar")
	first, err := c.Decide(t.Context(), q)
	if err != nil {
		t.Fatalf("first Decide: %v", err)
	}
	second, err := c.Decide(t.Context(), q)
	if err != nil {
		t.Fatalf("second Decide: %v", err)
	}
	if first.Cached {
		t.Error("first verdict must come from the model")
	}
	if !second.Cached {
		t.Error("repeated question must be served from the cache")
	}
	if first.Choice != second.Choice || first.Advisory != second.Advisory {
		t.Errorf("cached verdict differs: %+v vs %+v", first, second)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("server called %d times, want 1", got)
	}
	// A different question still costs one fresh call.
	if _, err := c.Decide(t.Context(), testQuestion(8, "the cellar")); err != nil {
		t.Fatalf("third Decide: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("server called %d times after a new question, want 2", got)
	}
}

// TestCacheEvictsOldest pins the FIFO eviction bound and the client's cache size.
func TestCacheEvictsOldest(t *testing.T) {
	c := newCache(3)
	keys := make([]string, 4)
	for i := range keys {
		keys[i] = cacheKey(testQuestion(i, "s"))
		c.put(keys[i], Verdict{Choice: fmt.Sprintf("unique-%d", i), Advisory: true})
	}
	if _, ok := c.get(keys[0]); ok {
		t.Error("oldest entry should have been evicted")
	}
	for i := 1; i < len(keys); i++ {
		if _, ok := c.get(keys[i]); !ok {
			t.Errorf("entry %d evicted too early", i)
		}
	}
	if v, ok := c.get(keys[3]); !ok || !v.Cached {
		t.Errorf("cached read = (%+v, %v), want Cached verdict", v, ok)
	}
	c.mu.Lock()
	stored := c.entries[keys[3]]
	c.mu.Unlock()
	if stored.Cached {
		t.Error("stored verdict must stay fresh; only reads are marked Cached")
	}
	if got := New(Config{}).cache.max; got != 256 {
		t.Errorf("client cache bound = %d, want 256", got)
	}
}

// TestCacheKey checks the key is order-insensitive on options and safe
// against field-boundary collisions.
func TestCacheKey(t *testing.T) {
	base := Question{State: "ab", Question: "c", Options: []string{"x", "y"}}
	swapped := Question{State: "ab", Question: "c", Options: []string{"y", "x"}}
	boundary := Question{State: "a", Question: "bc", Options: []string{"x", "y"}}
	otherState := Question{State: "ba", Question: "c", Options: []string{"x", "y"}}
	if cacheKey(base) != cacheKey(swapped) {
		t.Error("option order must not change the cache key")
	}
	if cacheKey(base) == cacheKey(boundary) {
		t.Error("field boundaries must not be ambiguous")
	}
	if cacheKey(base) == cacheKey(otherState) {
		t.Error("distinct states must not collide")
	}
}

// TestValidateQuestion checks invalid questions are rejected before any
// network call.
func TestValidateQuestion(t *testing.T) {
	tests := []struct {
		name        string
		q           Question
		msgContains string
	}{
		{
			name:        "blank state",
			q:           Question{State: "  ", Question: "Which?", Options: []string{"a", "b"}},
			msgContains: "state must not be blank",
		},
		{
			name:        "blank question",
			q:           Question{State: "s", Question: " ", Options: []string{"a", "b"}},
			msgContains: "question must not be blank",
		},
		{
			name:        "single option",
			q:           Question{State: "s", Question: "Which?", Options: []string{"a"}},
			msgContains: "at least 2 options",
		},
		{
			name:        "blank option",
			q:           Question{State: "s", Question: "Which?", Options: []string{"a", " "}},
			msgContains: "option 1 is blank",
		},
		{
			name:        "duplicate option",
			q:           Question{State: "s", Question: "Which?", Options: []string{"a", "a"}},
			msgContains: "repeated",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				writeJSON(w, map[string]any{})
			}))
			t.Cleanup(srv.Close)

			_, err := newTestClient(srv).Decide(t.Context(), tt.q)
			if err == nil {
				t.Fatal("Decide accepted an invalid question")
			}
			if !strings.Contains(err.Error(), tt.msgContains) {
				t.Errorf("error = %q, want it to contain %q", err, tt.msgContains)
			}
			if got := calls.Load(); got != 0 {
				t.Errorf("server called %d times, want 0 (validation precedes network)", got)
			}
		})
	}
}

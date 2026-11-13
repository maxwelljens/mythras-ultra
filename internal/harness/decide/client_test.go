package decide

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// TestDecideHTTPStatuses drives Decide through the documented failure modes
// of /v1/systemone: 404 means the endpoint (or model) is missing and maps to
// ErrUnavailable; every other non-2xx is a plain wrapped error carrying the
// server's message; unparseable 2xx bodies are ErrBadResponse.
func TestDecideHTTPStatuses(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantIs      []error
		wantNotIs   []error
		msgContains string
	}{
		{
			name:        "malformed JSON",
			status:      http.StatusOK,
			body:        "not json{{",
			wantIs:      []error{ErrBadResponse},
			wantNotIs:   []error{ErrUnavailable},
			msgContains: "parse response",
		},
		{
			name:        "error body with 200",
			status:      http.StatusOK,
			body:        `{"error":"text and schema must not exceed 64 KiB"}`,
			wantIs:      []error{ErrBadResponse},
			wantNotIs:   []error{ErrUnavailable},
			msgContains: "64 KiB",
		},
		{
			name:        "413 over budget",
			status:      http.StatusRequestEntityTooLarge,
			body:        `{"error":"text and schema must not exceed 64 KiB"}`,
			wantNotIs:   []error{ErrUnavailable, ErrBadResponse},
			msgContains: "64 KiB",
		},
		{
			name:        "404 missing endpoint",
			status:      http.StatusNotFound,
			body:        `{"error":"model not found"}`,
			wantIs:      []error{ErrUnavailable},
			wantNotIs:   []error{ErrBadResponse},
			msgContains: "/v1/systemone",
		},
		{
			name:        "500 server error",
			status:      http.StatusInternalServerError,
			body:        `{"error":"model load failed"}`,
			wantNotIs:   []error{ErrUnavailable, ErrBadResponse},
			msgContains: "model load failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			t.Cleanup(srv.Close)

			_, err := newTestClient(srv).Decide(t.Context(), testQuestion(0, "s"))
			if err == nil {
				t.Fatal("Decide returned no error")
			}
			for _, want := range tt.wantIs {
				if !errors.Is(err, want) {
					t.Errorf("error = %v, want errors.Is(%v)", err, want)
				}
			}
			for _, unwanted := range tt.wantNotIs {
				if errors.Is(err, unwanted) {
					t.Errorf("error = %v, must not match %v", err, unwanted)
				}
			}
			if tt.msgContains != "" && !strings.Contains(err.Error(), tt.msgContains) {
				t.Errorf("error = %q, want it to contain %q", err, tt.msgContains)
			}
		})
	}
}

// TestDecideResponseTooLarge checks responses over the 64 KiB read cap are
// rejected with ErrBadResponse instead of being buffered.
func TestDecideResponseTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w,
			`{"model":"tev1:4b","answers":{"q0":{"type":"choice","choice":"fallback","probabilities":{},"confidence":1}},"pad":"`+
				strings.Repeat("x", maxResponseBytes)+`"}`)
	}))
	t.Cleanup(srv.Close)

	_, err := newTestClient(srv).Decide(t.Context(), testQuestion(0, "s"))
	if !errors.Is(err, ErrBadResponse) {
		t.Fatalf("Decide error = %v, want ErrBadResponse", err)
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error = %q, want it to mention the size cap", err)
	}
}

// TestOversizedStateTruncated: a state over the 64 KiB request budget is
// truncated safely so the question still gets answered.
func TestOversizedStateTruncated(t *testing.T) {
	var mu sync.Mutex
	var got systemOneRequest
	var bodyLen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		var req systemOneRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("decode /v1/systemone request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		mu.Lock()
		bodyLen = len(raw)
		got = req
		mu.Unlock()
		writeSystemOneAnswer(t, w, req)
	}))
	t.Cleanup(srv.Close)

	q := testQuestion(0, "x")
	q.State = strings.Repeat("x", 200_000)
	v, err := newTestClient(srv).Decide(t.Context(), q)
	if err != nil {
		t.Fatalf("Decide with oversized state: %v", err)
	}
	if v.Choice != "unique-0" {
		t.Errorf("Choice = %q, want unique-0", v.Choice)
	}
	mu.Lock()
	defer mu.Unlock()
	if bodyLen > maxRequestBytes {
		t.Errorf("request body is %d bytes, over the %d cap", bodyLen, maxRequestBytes)
	}
	if len(got.State) >= len(q.State) {
		t.Errorf("state not truncated: %d bytes, want < %d", len(got.State), len(q.State))
	}
	if strings.TrimSpace(got.State) == "" {
		t.Error("truncated state must stay non-blank (SystemOneContent requires it)")
	}
}

// TestOversizedQuestionRejected: when the question text alone cannot fit the
// 64 KiB budget, Decide errors instead of sending anything.
func TestOversizedQuestionRejected(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		t.Error("handler must not be called for an unsendable request")
		writeJSON(w, map[string]any{})
	}))
	t.Cleanup(srv.Close)

	q := testQuestion(0, "s")
	q.Question = strings.Repeat("q", maxRequestBytes)
	_, err := newTestClient(srv).Decide(t.Context(), q)
	if err == nil {
		t.Fatal("Decide accepted a question over the request budget")
	}
	if errors.Is(err, ErrUnavailable) || errors.Is(err, ErrBadResponse) {
		t.Errorf("error = %v, want a plain wrapped input error", err)
	}
	if !strings.Contains(err.Error(), "exceed") {
		t.Errorf("error = %q, want it to mention the size limit", err)
	}
	if calls != 0 {
		t.Errorf("server called %d times, want 0", calls)
	}
}

// TestHealthy covers the /api/tags probe: present model (both name and
// model fields, tagged and untagged), missing model, and server failures.
func TestHealthy(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		configured  string
		wantErr     bool
		msgContains string
	}{
		{
			name:       "model present by name",
			status:     http.StatusOK,
			body:       `{"models":[{"name":"tev1:4b"}]}`,
			configured: "tev1:4b",
		},
		{
			name:       "model present by legacy model field",
			status:     http.StatusOK,
			body:       `{"models":[{"model":"tev1:4b"}]}`,
			configured: "tev1:4b",
		},
		{
			name:       "untagged config matches latest",
			status:     http.StatusOK,
			body:       `{"models":[{"name":"tev1:latest"}]}`,
			configured: "tev1",
		},
		{
			name:        "model missing",
			status:      http.StatusOK,
			body:        `{"models":[{"name":"llama3:8b"}]}`,
			configured:  "tev1:4b",
			wantErr:     true,
			msgContains: "tev1:4b",
		},
		{
			name:        "empty model list",
			status:      http.StatusOK,
			body:        `{"models":[]}`,
			configured:  "tev1:4b",
			wantErr:     true,
			msgContains: "available: none",
		},
		{
			name:       "server error",
			status:     http.StatusInternalServerError,
			body:       `{"error":"boom"}`,
			configured: "tev1:4b",
			wantErr:    true,
		},
		{
			name:       "not an Ollama server",
			status:     http.StatusNotFound,
			body:       "not found",
			configured: "tev1:4b",
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/tags" {
					t.Errorf("probe = %s %s, want GET /api/tags", r.Method, r.URL.Path)
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			t.Cleanup(srv.Close)

			err := New(Config{Enabled: true, URL: srv.URL, Model: tt.configured}).Healthy(t.Context())
			if tt.wantErr {
				if !errors.Is(err, ErrUnavailable) {
					t.Fatalf("Healthy error = %v, want ErrUnavailable", err)
				}
				if tt.msgContains != "" && !strings.Contains(err.Error(), tt.msgContains) {
					t.Errorf("error = %q, want it to contain %q", err, tt.msgContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("Healthy = %v, want nil", err)
			}
		})
	}
}

// TestHealthyServerDown: an unreachable Ollama fails the probe with
// ErrUnavailable.
func TestHealthyServerDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not be called")
	}))
	url := srv.URL
	srv.Close()

	if err := newTestClientAt(url).Healthy(t.Context()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Healthy error = %v, want ErrUnavailable", err)
	}
}

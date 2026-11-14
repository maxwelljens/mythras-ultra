package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNormalizeBaseURL pins the base-URL normalisation the adapters rely
// on: full endpoint URLs are truncated back to the base the SDK will
// re-append their path to (the doubled /chat/completions 404 regression),
// trailing slashes are trimmed, and everything else is left alone.
func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "full openai endpoint",
			in:   "https://opencode.ai/inference/go/openai/v1/chat/completions",
			want: "https://opencode.ai/inference/go/openai/v1",
		},
		{
			name: "full endpoint at root",
			in:   "http://localhost:11434/chat/completions",
			want: "http://localhost:11434",
		},
		{
			name: "base unchanged",
			in:   "https://api.deepseek.com/v1",
			want: "https://api.deepseek.com/v1",
		},
		{
			name: "zen base unchanged",
			in:   "https://opencode.ai/zen/go/v1",
			want: "https://opencode.ai/zen/go/v1",
		},
		{
			name: "trailing slash trimmed",
			in:   "http://localhost:11434/v1/",
			want: "http://localhost:11434/v1",
		},
		{
			name: "anthropic full endpoint",
			in:   "https://api.anthropic.com/v1/messages",
			want: "https://api.anthropic.com",
		},
		{
			name: "anthropic messages suffix",
			in:   "https://host/proxy/v1/messages",
			want: "https://host/proxy",
		},
		{
			name: "bare messages suffix",
			in:   "https://host/proxy/messages",
			want: "https://host/proxy",
		},
		{
			name: "empty stays empty",
			in:   "",
			want: "",
		},
		{
			name: "env placeholder untouched",
			in:   "$AZURE_OPENAI_API_ENDPOINT",
			want: "$AZURE_OPENAI_API_ENDPOINT",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeBaseURL(tc.in); got != tc.want {
				t.Errorf("NormalizeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// chatCompletionBody is a minimal non-streaming chat.completion reply.
const chatCompletionBody = `{"id":"c1","created":1,"model":"test-model","object":"chat.completion",` +
	`"choices":[{"index":0,"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],` +
	`"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

// TestFullEndpointBaseSendsSinglePath is the regression test for the
// doubled-path 404: a full endpoint URL in base_url must reach the server
// as exactly one /chat/completions, not /chat/completions/chat/completions.
// It also pins the request headers: x-opencode-session from SessionID,
// static config headers, and the configured User-Agent.
func TestFullEndpointBaseSendsSinglePath(t *testing.T) {
	var got struct {
		path      string
		session   string
		custom    string
		userAgent string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.session = r.Header.Get("x-opencode-session")
		got.custom = r.Header.Get("x-custom-header")
		got.userAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(chatCompletionBody))
	}))
	defer srv.Close()

	client, err := NewFantasyClient(Config{
		Provider:  "openaicompat",
		Model:     "test-model",
		APIKey:    "test-key",
		BaseURL:   srv.URL + "/chat/completions", // full endpoint form, as documented
		SessionID: "s-abc123",
		Headers:   map[string]string{"x-custom-header": "custom-value"},
		UserAgent: "myul/test",
	})
	if err != nil {
		t.Fatalf("NewFantasyClient: %v", err)
	}
	if _, err := client.Chat(context.Background(), messageWith("hello")); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got.path != "/chat/completions" {
		t.Errorf("request path = %q, want exactly %q", got.path, "/chat/completions")
	}
	if got.session != "s-abc123" {
		t.Errorf("x-opencode-session = %q, want %q", got.session, "s-abc123")
	}
	if got.custom != "custom-value" {
		t.Errorf("x-custom-header = %q, want %q", got.custom, "custom-value")
	}
	if got.userAgent != "myul/test" {
		t.Errorf("User-Agent = %q, want %q", got.userAgent, "myul/test")
	}
}

// TestAnthropicFullEndpointBase is the same regression for the
// anthropic-style suffix: a full .../v1/messages base must not double.
func TestAnthropicFullEndpointBase(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"test-model",` +
			`"content":[{"type":"text","text":"Hi"}],"stop_reason":"end_turn",` +
			`"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer srv.Close()

	client, err := NewFantasyClient(Config{
		Provider:  "anthropic",
		Model:     "test-model",
		APIKey:    "test-key",
		BaseURL:   srv.URL + "/v1/messages", // full endpoint form
		SessionID: "s-abc123",
	})
	if err != nil {
		t.Fatalf("NewFantasyClient: %v", err)
	}
	if _, err := client.Chat(context.Background(), messageWith("hello")); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if gotPath != "/v1/messages" {
		t.Errorf("request path = %q, want exactly %q", gotPath, "/v1/messages")
	}
}

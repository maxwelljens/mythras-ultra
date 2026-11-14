package llm

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"charm.land/fantasy"
)

// fakeModel is the internal seam: it implements modelClient (the slice of
// fantasy.LanguageModel the adapter uses) so tests exercise the mapping code
// without a network.
type fakeModel struct {
	generateFn func(call fantasy.Call) (*fantasy.Response, error)
	streamFn   func(call fantasy.Call) (fantasy.StreamResponse, error)
	calls      []fantasy.Call
}

func (f *fakeModel) Generate(_ context.Context, call fantasy.Call) (*fantasy.Response, error) {
	f.calls = append(f.calls, call)
	if f.generateFn == nil {
		return nil, errors.New("fakeModel: no generateFn configured")
	}
	return f.generateFn(call)
}

func (f *fakeModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	f.calls = append(f.calls, call)
	if f.streamFn == nil {
		return nil, errors.New("fakeModel: no streamFn configured")
	}
	return f.streamFn(call)
}

// failingProvider fails language-model resolution, for exercising modelFor.
type failingProvider struct{ err error }

func (f failingProvider) Name() string { return "failing" }

func (f failingProvider) LanguageModel(context.Context, string) (fantasy.LanguageModel, error) {
	return nil, f.err
}

// messageWith is a valid single-user-message request for tests that only care
// about the response path.
func messageWith(text string) ChatRequest {
	return ChatRequest{Messages: []Message{{Role: RoleUser, Content: text}}}
}

func TestFantasyChatMapsConversationAndTools(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"skill": map[string]any{"type": "string"},
		},
		"required": []any{"skill"},
	}
	fake := &fakeModel{
		generateFn: func(fantasy.Call) (*fantasy.Response, error) {
			return &fantasy.Response{
				Content: fantasy.ResponseContent{
					fantasy.TextContent{Text: "The door holds — then gives."},
					fantasy.ToolCallContent{
						ToolCallID: "call_9",
						ToolName:   "rules_lookup",
						Input:      `{"query":"doors"}`,
					},
				},
				Usage: fantasy.Usage{InputTokens: 120, OutputTokens: 14},
			}, nil
		},
	}
	client := &FantasyClient{model: fake}
	req := ChatRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: "You are the GM."},
			{Role: RoleUser, Content: "I shoulder the door."},
			{
				Role:    RoleAssistant,
				Content: "The wood groans.",
				ToolCalls: []ToolCall{{
					ID:   "call_1",
					Name: "skill_test",
					Args: []byte(`{"skill":"Strength"}`),
				}},
			},
			{Role: RoleTool, ToolCallID: "call_1", Name: "skill_test", Content: `{"ok":true,"summary":"Success"}`},
			{Role: RoleUser, Content: "And then?"},
		},
		Tools: []ToolSpec{{
			Name:        "skill_test",
			Description: "Roll a skill test.",
			Schema:      schema,
		}},
	}

	resp, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("Generate called %d times, want 1", len(fake.calls))
	}
	call := fake.calls[0]

	wantRoles := []fantasy.MessageRole{
		fantasy.MessageRoleSystem,
		fantasy.MessageRoleUser,
		fantasy.MessageRoleAssistant,
		fantasy.MessageRoleTool,
		fantasy.MessageRoleUser,
	}
	if len(call.Prompt) != len(wantRoles) {
		t.Fatalf("prompt has %d messages, want %d", len(call.Prompt), len(wantRoles))
	}
	for i, want := range wantRoles {
		if got := call.Prompt[i].Role; got != want {
			t.Errorf("prompt[%d].Role = %q, want %q", i, got, want)
		}
	}

	// System message carries the prompt text.
	sysPart, ok := call.Prompt[0].Content[0].(fantasy.TextPart)
	if !ok || sysPart.Text != "You are the GM." {
		t.Errorf("system text part = %#v, want %q", call.Prompt[0].Content[0], "You are the GM.")
	}

	// Assistant replay: text part followed by the tool-call part.
	asst := call.Prompt[2].Content
	if len(asst) != 2 {
		t.Fatalf("assistant parts = %d, want 2", len(asst))
	}
	if p, ok := asst[0].(fantasy.TextPart); !ok || p.Text != "The wood groans." {
		t.Errorf("assistant text part = %#v", asst[0])
	}
	tc, ok := asst[1].(fantasy.ToolCallPart)
	if !ok {
		t.Fatalf("assistant part 1 = %T, want ToolCallPart", asst[1])
	}
	if tc.ToolCallID != "call_1" || tc.ToolName != "skill_test" || tc.Input != `{"skill":"Strength"}` {
		t.Errorf("assistant tool call = %+v", tc)
	}

	// Tool result: identified by call id, payload as text output.
	toolMsg := call.Prompt[3]
	toolPart, ok := toolMsg.Content[0].(fantasy.ToolResultPart)
	if !ok {
		t.Fatalf("tool part = %T, want ToolResultPart", toolMsg.Content[0])
	}
	if toolPart.ToolCallID != "call_1" {
		t.Errorf("tool result call id = %q, want call_1", toolPart.ToolCallID)
	}
	toolOut, ok := toolPart.Output.(fantasy.ToolResultOutputContentText)
	if !ok || toolOut.Text != `{"ok":true,"summary":"Success"}` {
		t.Errorf("tool result output = %#v", toolPart.Output)
	}

	// Tools pass through as FunctionTool with the harness's own schema.
	if len(call.Tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(call.Tools))
	}
	ft, ok := call.Tools[0].(fantasy.FunctionTool)
	if !ok {
		t.Fatalf("tool = %T, want FunctionTool", call.Tools[0])
	}
	if ft.Name != "skill_test" || ft.Description != "Roll a skill test." {
		t.Errorf("tool header = %q/%q", ft.Name, ft.Description)
	}
	if !reflect.DeepEqual(ft.InputSchema, schema) {
		t.Errorf("tool schema = %#v, want %#v", ft.InputSchema, schema)
	}

	// Response mapping: text, tool calls and usage.
	if resp.Message.Role != RoleAssistant {
		t.Errorf("role = %q, want assistant", resp.Message.Role)
	}
	if resp.Message.Content != "The door holds — then gives." {
		t.Errorf("content = %q", resp.Message.Content)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(resp.Message.ToolCalls))
	}
	got := resp.Message.ToolCalls[0]
	if got.ID != "call_9" || got.Name != "rules_lookup" || string(got.Args) != `{"query":"doors"}` {
		t.Errorf("tool call = %+v", got)
	}
	if resp.Usage != (Usage{InputTokens: 120, OutputTokens: 14}) {
		t.Errorf("usage = %+v", resp.Usage)
	}
}

func TestFantasyChatRejectsMalformedRequests(t *testing.T) {
	cases := []struct {
		name    string
		req     ChatRequest
		wantErr string
	}{
		{"no messages", ChatRequest{}, "no messages"},
		{
			"system after history",
			ChatRequest{Messages: []Message{
				{Role: RoleUser, Content: "hi"},
				{Role: RoleSystem, Content: "late"},
			}},
			"must precede",
		},
		{"unknown role", ChatRequest{Messages: []Message{{Role: "narrator"}}}, "unknown message role"},
		{
			"empty assistant",
			ChatRequest{Messages: []Message{{Role: RoleAssistant}}},
			"neither content nor tool calls",
		},
		{
			"tool result without id",
			ChatRequest{Messages: []Message{{Role: RoleTool, Content: "{}"}}},
			"no tool call id",
		},
		{
			"unnamed tool",
			ChatRequest{
				Messages: []Message{{Role: RoleUser, Content: "hi"}},
				Tools:    []ToolSpec{{Description: "anonymous"}},
			},
			"no name",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeModel{}
			_, err := (&FantasyClient{model: fake}).Chat(context.Background(), tc.req)
			if err == nil {
				t.Fatal("Chat succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err, tc.wantErr)
			}
			if len(fake.calls) != 0 {
				t.Errorf("Generate called %d times for an invalid request, want 0", len(fake.calls))
			}
		})
	}
}

func TestFantasyChatWrapsProviderErrors(t *testing.T) {
	boom := errors.New("upstream exploded")

	fake := &fakeModel{generateFn: func(fantasy.Call) (*fantasy.Response, error) { return nil, boom }}
	_, err := (&FantasyClient{model: fake}).Chat(context.Background(), messageWith("hi"))
	if !errors.Is(err, ErrProvider) {
		t.Errorf("error %v does not wrap ErrProvider", err)
	}
	if !errors.Is(err, boom) {
		t.Errorf("error %v does not wrap the provider error", err)
	}

	// A nil response with no error must not panic and must surface as a
	// provider failure.
	nilModel := &fakeModel{generateFn: func(fantasy.Call) (*fantasy.Response, error) { return nil, nil }}
	if _, err := (&FantasyClient{model: nilModel}).Chat(context.Background(), messageWith("hi")); !errors.Is(err, ErrProvider) {
		t.Errorf("nil response error = %v, want ErrProvider", err)
	}

	// Language-model resolution failures wrap both sentinels too.
	resolveErr := errors.New("no such model")
	client := &FantasyClient{provider: failingProvider{err: resolveErr}, modelID: "m"}
	if _, err := client.Chat(context.Background(), messageWith("hi")); !errors.Is(err, ErrProvider) || !errors.Is(err, resolveErr) {
		t.Errorf("resolution error = %v, want ErrProvider wrapping %v", err, resolveErr)
	}

	// A client with neither provider nor model fails cleanly instead of
	// panicking.
	if _, err := (&FantasyClient{}).Chat(context.Background(), messageWith("hi")); err == nil {
		t.Error("zero-value client Chat succeeded, want error")
	}
}

func TestFantasyStreamRelaysEvents(t *testing.T) {
	parts := []fantasy.StreamPart{
		{Type: fantasy.StreamPartTypeTextStart, ID: "t1"},
		{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: "The lamp "},
		{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: "gutters."},
		{Type: fantasy.StreamPartTypeToolCall, ID: "call_7", ToolCallName: "note", ToolCallInput: `{"text":"lamp"}`},
		{Type: fantasy.StreamPartTypeFinish, Usage: fantasy.Usage{InputTokens: 50, OutputTokens: 9}},
	}
	fake := &fakeModel{streamFn: func(fantasy.Call) (fantasy.StreamResponse, error) {
		return func(yield func(fantasy.StreamPart) bool) {
			for _, part := range parts {
				if !yield(part) {
					return
				}
			}
		}, nil
	}}

	var events []StreamEvent
	err := (&FantasyClient{model: fake}).Stream(context.Background(), messageWith("hi"), func(ev StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var kinds []StreamKind
	var text strings.Builder
	for _, ev := range events {
		kinds = append(kinds, ev.Kind)
		if ev.Kind == StreamTextDelta {
			text.WriteString(ev.Text)
		}
	}
	wantKinds := []StreamKind{StreamTextDelta, StreamTextDelta, StreamToolCall, StreamDone}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Fatalf("event kinds = %v, want %v", kinds, wantKinds)
	}
	if text.String() != "The lamp gutters." {
		t.Errorf("streamed text = %q", text.String())
	}
	toolEvents := events[2]
	if toolEvents.ToolCall.ID != "call_7" || toolEvents.ToolCall.Name != "note" ||
		string(toolEvents.ToolCall.Args) != `{"text":"lamp"}` {
		t.Errorf("tool call event = %+v", toolEvents.ToolCall)
	}
	if done := events[len(events)-1]; done.Usage != (Usage{InputTokens: 50, OutputTokens: 9}) {
		t.Errorf("done usage = %+v", done.Usage)
	}
}

func TestFantasyStreamCallbackErrorStopsIteration(t *testing.T) {
	listenerGone := errors.New("listener gone")
	parts := []fantasy.StreamPart{
		{Type: fantasy.StreamPartTypeTextDelta, Delta: "first"},
		{Type: fantasy.StreamPartTypeTextDelta, Delta: "second"},
	}
	yielded := 0
	fake := &fakeModel{streamFn: func(fantasy.Call) (fantasy.StreamResponse, error) {
		return func(yield func(fantasy.StreamPart) bool) {
			for _, part := range parts {
				yielded++
				if !yield(part) {
					return
				}
			}
		}, nil
	}}

	err := (&FantasyClient{model: fake}).Stream(context.Background(), messageWith("hi"), func(StreamEvent) error {
		return listenerGone
	})
	if !errors.Is(err, listenerGone) {
		t.Fatalf("error = %v, want wrapped listener error", err)
	}
	if !strings.Contains(err.Error(), "stream callback") {
		t.Errorf("error = %q, want stream callback context", err)
	}
	if yielded != 1 {
		t.Errorf("stream generator yielded %d times after callback failure, want 1", yielded)
	}
}

func TestFantasyStreamErrorPartWrapsProvider(t *testing.T) {
	upstream := errors.New("upstream 500")
	fake := &fakeModel{streamFn: func(fantasy.Call) (fantasy.StreamResponse, error) {
		return func(yield func(fantasy.StreamPart) bool) {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: upstream})
		}, nil
	}}

	var events []StreamEvent
	err := (&FantasyClient{model: fake}).Stream(context.Background(), messageWith("hi"), func(ev StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	if !errors.Is(err, ErrProvider) || !errors.Is(err, upstream) {
		t.Fatalf("error = %v, want ErrProvider wrapping %v", err, upstream)
	}
	if len(events) != 0 {
		t.Errorf("emitted %d events before the error, want 0", len(events))
	}
}

// TestResolveTargetProviderMapping pins the provider → adapter mapping:
// the fantasy-native ids keep their native adapters, every other id
// resolves through the embedded catalog and maps by the API type it
// declares (endpoint precedence: explicit config > catalog), and shapes no
// adapter can reach fail naming the provider instead of silently
// misconfiguring.
func TestResolveTargetProviderMapping(t *testing.T) {
	cases := []struct {
		name        string
		cfg         Config
		wantAdapter string
		wantBaseURL string // checked only when non-empty
		wantErr     string // substring, "" means the mapping must succeed
	}{
		{name: "openai native", cfg: Config{Provider: "openai"}, wantAdapter: "openai"},
		{name: "anthropic native", cfg: Config{Provider: "anthropic"}, wantAdapter: "anthropic"},
		{name: "openrouter native", cfg: Config{Provider: "openrouter"}, wantAdapter: "openrouter"},
		{
			name:        "openaicompat native",
			cfg:         Config{Provider: "openaicompat", BaseURL: "http://localhost:11434/v1"},
			wantAdapter: "openaicompat",
			wantBaseURL: "http://localhost:11434/v1",
		},
		{
			name:        "deepseek maps to openaicompat",
			cfg:         Config{Provider: "deepseek"},
			wantAdapter: "openaicompat",
			wantBaseURL: "https://api.deepseek.com/v1",
		},
		{
			name:        "deepseek id is case-insensitive",
			cfg:         Config{Provider: "DeepSeek"},
			wantAdapter: "openaicompat",
			wantBaseURL: "https://api.deepseek.com/v1",
		},
		{
			name:        "opencode-go maps to openaicompat",
			cfg:         Config{Provider: "opencode-go"},
			wantAdapter: "openaicompat",
			wantBaseURL: "https://opencode.ai/zen/go/v1",
		},
		{
			name:        "explicit base url beats the catalog endpoint",
			cfg:         Config{Provider: "deepseek", BaseURL: "https://proxy.example/v1"},
			wantAdapter: "openaicompat",
			wantBaseURL: "https://proxy.example/v1",
		},
		{
			// minimax speaks the Anthropic messages API, so it lands on
			// the anthropic adapter rather than being rejected.
			name:        "anthropic-type provider maps to anthropic",
			cfg:         Config{Provider: "minimax"},
			wantAdapter: "anthropic",
		},
		{
			name:    "responses type has no adapter",
			cfg:     Config{Provider: "cortecs"},
			wantErr: "API type",
		},
		{
			// azure's endpoint is an unresolved $ENV placeholder.
			name:    "placeholder endpoint without base_url",
			cfg:     Config{Provider: "azure"},
			wantErr: "no usable base URL",
		},
		{
			name:    "unknown provider",
			cfg:     Config{Provider: "not-a-provider"},
			wantErr: "unsupported provider",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTarget(tc.cfg)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("resolveTarget(%+v) = %+v, want error %q", tc.cfg, got, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want substring %q", err, tc.wantErr)
				}
				if tc.wantErr == "unsupported provider" && !strings.Contains(err.Error(), tc.cfg.Provider) {
					t.Errorf("error %q does not name the provider %q", err, tc.cfg.Provider)
				}
				if tc.wantErr == "API type" && !strings.Contains(err.Error(), tc.cfg.Provider) {
					t.Errorf("error %q does not name the provider %q", err, tc.cfg.Provider)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveTarget(%+v): %v", tc.cfg, err)
			}
			if got.adapter != tc.wantAdapter {
				t.Errorf("adapter = %q, want %q", got.adapter, tc.wantAdapter)
			}
			if tc.wantBaseURL != "" && got.baseURL != tc.wantBaseURL {
				t.Errorf("baseURL = %q, want %q", got.baseURL, tc.wantBaseURL)
			}
		})
	}
}

func TestNewFantasyClientConfig(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr string // substring, "" means construction must succeed
	}{
		{
			name:    "openai missing key",
			cfg:     Config{Provider: "openai", Model: "gpt-5"},
			wantErr: "missing API key",
		},
		{
			name:    "anthropic missing key",
			cfg:     Config{Provider: "anthropic", Model: "claude-sonnet-4-5"},
			wantErr: "missing API key",
		},
		{
			name:    "openrouter missing key",
			cfg:     Config{Provider: "openrouter", Model: "moonshotai/kimi-k2"},
			wantErr: "missing API key",
		},
		{
			name:    "missing model",
			cfg:     Config{Provider: "openaicompat", APIKey: "k"},
			wantErr: "model id is required",
		},
		{
			// groq IS in the embedded catalog now (it maps to
			// openaicompat); only an id nothing knows fails here.
			name:    "unknown provider",
			cfg:     Config{Provider: "not-a-provider", Model: "llama", APIKey: "k"},
			wantErr: "unsupported provider",
		},
		{
			name:    "openrouter rejects base url",
			cfg:     Config{Provider: "openrouter", Model: "m", APIKey: "k", BaseURL: "http://example.invalid"},
			wantErr: "does not support a custom base URL",
		},
		{
			name: "openai with key",
			cfg:  Config{Provider: "openai", Model: "gpt-5", APIKey: "k"},
		},
		{
			name: "anthropic with key and base url",
			cfg:  Config{Provider: "anthropic", Model: "claude", APIKey: "k", BaseURL: "http://example.invalid"},
		},
		{
			name: "openaicompat local server without key",
			cfg:  Config{Provider: "openaicompat", Model: "llama3.1", BaseURL: "http://localhost:11434/v1"},
		},
		{
			// deepseek is not a native adapter: it maps to openaicompat
			// through the catalog, but still demands its declared key.
			name:    "catalog provider missing key",
			cfg:     Config{Provider: "deepseek", Model: "deepseek-chat"},
			wantErr: "missing API key",
		},
		{
			name: "catalog provider with key",
			cfg:  Config{Provider: "deepseek", Model: "deepseek-chat", APIKey: "k"},
		},
		{
			// An API type no adapter speaks fails before any credential
			// check, naming the provider and its type.
			name:    "unsupported api type",
			cfg:     Config{Provider: "cortecs", Model: "cortecs-model", APIKey: "k"},
			wantErr: "API type",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewFantasyClient(tc.cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("NewFantasyClient: %v", err)
				}
				if client == nil || client.modelID != tc.cfg.Model {
					t.Fatalf("client = %+v, want non-nil with model %q", client, tc.cfg.Model)
				}
				return
			}
			if err == nil {
				t.Fatal("NewFantasyClient succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err, tc.wantErr)
			}
			if tc.wantErr == "missing API key" {
				if !errors.Is(err, ErrNoAPIKey) {
					t.Errorf("error %v does not wrap ErrNoAPIKey", err)
				}
			} else if errors.Is(err, ErrNoAPIKey) {
				t.Errorf("error wraps ErrNoAPIKey unexpectedly: %v", err)
			}
		})
	}
}

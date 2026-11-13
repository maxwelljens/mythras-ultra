package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openaicompat"
	"charm.land/fantasy/providers/openrouter"
)

// modelClient is the slice of [fantasy.LanguageModel] this adapter uses —
// Generate and Stream only. Unit tests substitute a fake here and never
// touch the network; everything else in the adapter is pure mapping code.
type modelClient interface {
	Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error)
	Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error)
}

// Config selects the provider, model and credentials for a [FantasyClient].
// Keys are passed explicitly: this package never reads the environment (the
// app layer resolves `api_key_env` and hands the value in).
type Config struct {
	// Provider names the fantasy provider package. Supported: "openai",
	// "anthropic", "openrouter" and "openaicompat" (the OpenAI-compatible
	// path for local or self-hosted servers such as Ollama's /v1 API).
	Provider string
	// Model is the provider-specific model id passed to
	// [fantasy.Provider.LanguageModel].
	Model string
	// APIKey is the provider credential. Required for "openai",
	// "anthropic" and "openrouter"; optional for "openaicompat", whose
	// local servers usually need none. Missing required keys fail with
	// [ErrNoAPIKey].
	APIKey string
	// BaseURL overrides the provider endpoint. Honoured by "openai",
	// "anthropic" and "openaicompat"; "openrouter" exposes no base-URL
	// option and rejects a non-empty value instead of ignoring it.
	BaseURL string
}

// FantasyClient is the production [Client], backed by charm.land/fantasy.
//
// Mapping decisions (verified against go doc for the pinned v0.48.1):
//
//   - The adapter drives [fantasy.LanguageModel] directly
//     (Provider → LanguageModel → Generate/Stream) instead of
//     [fantasy.Agent]. Fantasy's agent loop executes registered tools
//     itself; Mythras Ultra's harness must remain the only executor of
//     tools, dispatching them through its registry and session log
//     (ARCHITECTURE.md §3.2). Each Chat/Stream call therefore sends the
//     whole conversation and returns the model's next assistant message;
//     the harness appends the tool results and calls again.
//   - Tools are advertised as [fantasy.FunctionTool] values carrying the
//     harness's own JSON Schemas, which every provider maps to the
//     function-tool wire format — so arbitrary registry schemas pass
//     through without reflection or generated structs.
//
// The cached language model is created once under a mutex; concurrent use of
// the underlying fantasy model remains subject to the provider's own
// guarantees.
type FantasyClient struct {
	provider fantasy.Provider
	modelID  string

	mu    sync.Mutex
	model modelClient
}

var _ Client = (*FantasyClient)(nil)

// NewFantasyClient builds a [Client] for cfg. It performs no network I/O:
// the provider is constructed eagerly, the language model is resolved lazily
// on the first call with that call's context.
func NewFantasyClient(cfg Config) (*FantasyClient, error) {
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("llm: config: model id is required")
	}
	provider, err := newProvider(cfg)
	if err != nil {
		return nil, err
	}
	return &FantasyClient{provider: provider, modelID: cfg.Model}, nil
}

// newProvider constructs the fantasy provider package for cfg.
func newProvider(cfg Config) (fantasy.Provider, error) {
	providerName := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch providerName {
	case "openai":
		if err := requireAPIKey(providerName, cfg.APIKey); err != nil {
			return nil, err
		}
		opts := []openai.Option{openai.WithAPIKey(cfg.APIKey)}
		if cfg.BaseURL != "" {
			opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
		}
		provider, err := openai.New(opts...)
		return wrapProvider(providerName, provider, err)
	case "anthropic":
		if err := requireAPIKey(providerName, cfg.APIKey); err != nil {
			return nil, err
		}
		opts := []anthropic.Option{anthropic.WithAPIKey(cfg.APIKey)}
		if cfg.BaseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(cfg.BaseURL))
		}
		provider, err := anthropic.New(opts...)
		return wrapProvider(providerName, provider, err)
	case "openrouter":
		if err := requireAPIKey(providerName, cfg.APIKey); err != nil {
			return nil, err
		}
		if cfg.BaseURL != "" {
			return nil, fmt.Errorf("llm: provider %q does not support a custom base URL", providerName)
		}
		provider, err := openrouter.New(openrouter.WithAPIKey(cfg.APIKey))
		return wrapProvider(providerName, provider, err)
	case "openaicompat":
		// The OpenAI-compatible path targets arbitrary servers (including
		// local ones), where the API key is optional.
		var opts []openaicompat.Option
		if cfg.APIKey != "" {
			opts = append(opts, openaicompat.WithAPIKey(cfg.APIKey))
		}
		if cfg.BaseURL != "" {
			opts = append(opts, openaicompat.WithBaseURL(cfg.BaseURL))
		}
		provider, err := openaicompat.New(opts...)
		return wrapProvider(providerName, provider, err)
	default:
		return nil, fmt.Errorf(
			"llm: unsupported provider %q (supported: openai, anthropic, openrouter, openaicompat)",
			cfg.Provider)
	}
}

// requireAPIKey fails with [ErrNoAPIKey] when a provider that needs a
// credential was given none.
func requireAPIKey(provider, key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("llm: %w: provider %q requires an API key", ErrNoAPIKey, provider)
	}
	return nil
}

// wrapProvider tags provider-constructor failures with [ErrProvider].
func wrapProvider(name string, provider fantasy.Provider, err error) (fantasy.Provider, error) {
	if err != nil {
		return nil, fmt.Errorf("llm: initialise %s provider: %w: %w", name, ErrProvider, err)
	}
	return provider, nil
}

// modelFor returns the cached language model, resolving it on first use.
func (c *FantasyClient) modelFor(ctx context.Context) (modelClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.model != nil {
		return c.model, nil
	}
	if c.provider == nil {
		// Defensive: NewFantasyClient always sets a provider; tests build
		// clients around an injected model instead.
		return nil, errors.New("llm: fantasy client has no provider or model")
	}
	model, err := c.provider.LanguageModel(ctx, c.modelID)
	if err != nil {
		return nil, fmt.Errorf("llm: resolve language model %q: %w: %w", c.modelID, ErrProvider, err)
	}
	c.model = model
	return model, nil
}

// Chat sends the full conversation and returns the model's next assistant
// message. Tool calls in the request history are replayed to the provider;
// tool calls in the response are returned to the harness, never executed.
func (c *FantasyClient) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	call, err := buildCall(req)
	if err != nil {
		return ChatResponse{}, err
	}
	model, err := c.modelFor(ctx)
	if err != nil {
		return ChatResponse{}, err
	}
	resp, err := model.Generate(ctx, call)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("llm: generate: %w: %w", ErrProvider, err)
	}
	if resp == nil {
		return ChatResponse{}, fmt.Errorf("llm: generate: %w: provider returned no response", ErrProvider)
	}
	return fromResponse(resp), nil
}

// Stream sends the full conversation and relays the model's next assistant
// message to emit: text deltas, then tool calls, then one done event carrying
// the call's usage. An error from emit aborts the stream.
func (c *FantasyClient) Stream(ctx context.Context, req ChatRequest, emit func(StreamEvent) error) error {
	if emit == nil {
		return errors.New("llm: stream: nil event callback")
	}
	call, err := buildCall(req)
	if err != nil {
		return err
	}
	model, err := c.modelFor(ctx)
	if err != nil {
		return err
	}
	seq, err := model.Stream(ctx, call)
	if err != nil {
		return fmt.Errorf("llm: stream: %w: %w", ErrProvider, err)
	}
	var (
		usage   Usage
		loopErr error
	)
	// relay records the first callback failure; the loop then stops so a
	// failing consumer never keeps paying for stream chunks.
	relay := func(ev StreamEvent) {
		if loopErr != nil {
			return
		}
		if err := emit(ev); err != nil {
			loopErr = fmt.Errorf("llm: stream callback: %w", err)
		}
	}
	for part := range seq {
		switch part.Type {
		case fantasy.StreamPartTypeTextDelta:
			if part.Delta != "" {
				relay(StreamEvent{Kind: StreamTextDelta, Text: part.Delta})
			}
		case fantasy.StreamPartTypeToolCall:
			relay(StreamEvent{Kind: StreamToolCall, ToolCall: ToolCall{
				ID:   part.ID,
				Name: part.ToolCallName,
				Args: rawArgs(part.ToolCallInput),
			}})
		case fantasy.StreamPartTypeError:
			if loopErr == nil {
				err := part.Error
				if err == nil {
					err = errors.New("provider reported an unnamed stream error")
				}
				loopErr = fmt.Errorf("llm: stream: %w: %w", ErrProvider, err)
			}
		}
		if part.Usage.InputTokens != 0 || part.Usage.OutputTokens != 0 {
			usage = Usage{
				InputTokens:  int(part.Usage.InputTokens),
				OutputTokens: int(part.Usage.OutputTokens),
			}
		}
		if loopErr != nil {
			break
		}
	}
	if loopErr != nil {
		return loopErr
	}
	if err := emit(StreamEvent{Kind: StreamDone, Usage: usage}); err != nil {
		return fmt.Errorf("llm: stream callback: %w", err)
	}
	return nil
}

// buildCall maps a [ChatRequest] onto a [fantasy.Call], validating the
// conversation shape the providers expect.
func buildCall(req ChatRequest) (fantasy.Call, error) {
	if len(req.Messages) == 0 {
		return fantasy.Call{}, errors.New("llm: chat request has no messages")
	}
	prompt := make(fantasy.Prompt, 0, len(req.Messages))
	seenHistory := false
	for i, msg := range req.Messages {
		if msg.Role == RoleSystem && seenHistory {
			return fantasy.Call{}, fmt.Errorf(
				"llm: message %d: system message must precede the conversation", i)
		}
		if msg.Role != RoleSystem {
			seenHistory = true
		}
		mapped, err := toPromptMessage(msg)
		if err != nil {
			return fantasy.Call{}, fmt.Errorf("llm: message %d: %w", i, err)
		}
		prompt = append(prompt, mapped)
	}
	call := fantasy.Call{Prompt: prompt}
	for _, spec := range req.Tools {
		if strings.TrimSpace(spec.Name) == "" {
			return fantasy.Call{}, errors.New("llm: tool spec has no name")
		}
		schema := spec.Schema
		if len(schema) == 0 {
			// Providers expect an arguments schema; an absent one defaults
			// to an unconstrained object rather than being sent as null.
			schema = map[string]any{"type": "object"}
		}
		call.Tools = append(call.Tools, fantasy.FunctionTool{
			Name:        spec.Name,
			Description: spec.Description,
			InputSchema: schema,
		})
	}
	return call, nil
}

// toPromptMessage maps one harness message onto fantasy's prompt format.
// System and user messages become text messages; assistant messages carry
// text and/or tool-call parts; tool messages carry the textual result of one
// earlier call (providers address results by tool call id).
func toPromptMessage(msg Message) (fantasy.Message, error) {
	switch msg.Role {
	case RoleSystem:
		return fantasy.NewSystemMessage(msg.Content), nil
	case RoleUser:
		return fantasy.NewUserMessage(msg.Content), nil
	case RoleAssistant:
		mapped := fantasy.Message{Role: fantasy.MessageRoleAssistant}
		if msg.Content != "" {
			mapped.Content = append(mapped.Content, fantasy.TextPart{Text: msg.Content})
		}
		for _, call := range msg.ToolCalls {
			mapped.Content = append(mapped.Content, fantasy.ToolCallPart{
				ToolCallID: call.ID,
				ToolName:   call.Name,
				Input:      argsString(call.Args),
			})
		}
		if len(mapped.Content) == 0 {
			return fantasy.Message{}, errors.New("assistant message has neither content nor tool calls")
		}
		return mapped, nil
	case RoleTool:
		if msg.ToolCallID == "" {
			return fantasy.Message{}, errors.New("tool result message has no tool call id")
		}
		return fantasy.Message{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: msg.ToolCallID,
					Output:     fantasy.ToolResultOutputContentText{Text: msg.Content},
				},
			},
		}, nil
	default:
		return fantasy.Message{}, fmt.Errorf("unknown message role %q", msg.Role)
	}
}

// fromResponse maps a provider response onto the vendor-neutral reply: the
// response text plus every tool call the model completed, and token usage.
func fromResponse(resp *fantasy.Response) ChatResponse {
	msg := Message{Role: RoleAssistant, Content: resp.Content.Text()}
	for _, call := range resp.Content.ToolCalls() {
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{
			ID:   call.ToolCallID,
			Name: call.ToolName,
			Args: rawArgs(call.Input),
		})
	}
	return ChatResponse{
		Message: msg,
		Usage: Usage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
		},
	}
}

// argsString renders tool-call arguments as the JSON string fantasy's message
// parts require, defaulting absent arguments to an empty object.
func argsString(args json.RawMessage) string {
	if len(bytes.TrimSpace(args)) == 0 {
		return "{}"
	}
	return string(args)
}

// rawArgs converts a provider-supplied argument string back into raw JSON,
// defaulting absent arguments to an empty object.
func rawArgs(input string) json.RawMessage {
	if len(bytes.TrimSpace([]byte(input))) == 0 {
		return json.RawMessage("{}")
	}
	return json.RawMessage(input)
}

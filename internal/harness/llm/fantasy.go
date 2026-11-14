package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"charm.land/catwalk/v3/pkg/catwalk"
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
	// Provider names the provider to talk to. The fantasy-native adapters
	// are "openai", "anthropic", "openrouter" and "openaicompat" (the
	// OpenAI-compatible path for local or self-hosted servers such as
	// Ollama's /v1 API). Any other id in the embedded catwalk catalog —
	// "deepseek", "groq", "opencode-go", … — resolves through
	// [ResolveProvider] and maps onto an adapter by the API type the
	// catalog declares (see [resolveTarget]).
	Provider string
	// Model is the provider-specific model id passed to
	// [fantasy.Provider.LanguageModel].
	Model string
	// APIKey is the provider credential. Required by the native
	// "openai", "anthropic" and "openrouter" adapters and by every
	// catalog provider that declares an API-key env var; optional for
	// native "openaicompat", whose local servers usually need none.
	// Missing required keys fail with [ErrNoAPIKey].
	APIKey string
	// BaseURL is the provider base URL; full endpoint URLs like
	// `.../v1/chat/completions` are accepted and normalised
	// ([NormalizeBaseURL]). Explicit config always wins over the
	// catalog's endpoint. Honoured by "openai", "anthropic" and
	// "openaicompat"; "openrouter" exposes no base-URL option and
	// rejects a non-empty value instead of ignoring it.
	BaseURL string
	// SessionID is sent as the x-opencode-session header on every
	// request when non-empty: OpenCode's inference gateway needs a
	// stable per-conversation id for routing and prompt caching, and
	// other providers ignore the extra header. The command layer
	// derives it from the session's state id so it survives restarts.
	SessionID string
	// Headers are extra static request headers from the config's
	// [llm.headers] table, applied to every request. A configured
	// x-opencode-session value is overridden by [Config.SessionID].
	Headers map[string]string
	// UserAgent identifies myul to the provider (e.g. "myul/v1.2.3")
	// instead of the SDK's generic User-Agent; empty keeps the
	// adapter's default.
	UserAgent string
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

// providerTarget is the resolved construction target for a [Config]: which
// fantasy adapter to build, the endpoint to point it at ("" lets the
// adapter's own default stand), and whether the provider requires an API
// key before the adapter is built.
type providerTarget struct {
	adapter  string
	baseURL  string
	needsKey bool
}

// sessionHeader is the per-conversation routing header OpenCode's
// inference gateway requires; harmless on providers that ignore it.
const sessionHeader = "x-opencode-session"

// newProvider constructs the fantasy provider package for cfg.
func newProvider(cfg Config) (fantasy.Provider, error) {
	target, err := resolveTarget(cfg)
	if err != nil {
		return nil, err
	}
	providerName := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if target.needsKey {
		if err := requireAPIKey(providerName, cfg.APIKey); err != nil {
			return nil, err
		}
	}
	// Static config headers first, then the session id on top: the
	// routing header is plumbing, so it wins over a same-named entry
	// in [llm.headers].
	headers := make(map[string]string, len(cfg.Headers)+1)
	for k, v := range cfg.Headers {
		headers[k] = v
	}
	if cfg.SessionID != "" {
		headers[sessionHeader] = cfg.SessionID
	}
	switch target.adapter {
	case "openai":
		opts := []openai.Option{openai.WithAPIKey(cfg.APIKey)}
		if target.baseURL != "" {
			opts = append(opts, openai.WithBaseURL(target.baseURL))
		}
		if len(headers) > 0 {
			opts = append(opts, openai.WithHeaders(headers))
		}
		if cfg.UserAgent != "" {
			opts = append(opts, openai.WithUserAgent(cfg.UserAgent))
		}
		provider, err := openai.New(opts...)
		return wrapProvider(providerName, provider, err)
	case "anthropic":
		opts := []anthropic.Option{anthropic.WithAPIKey(cfg.APIKey)}
		if target.baseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(target.baseURL))
		}
		if len(headers) > 0 {
			opts = append(opts, anthropic.WithHeaders(headers))
		}
		if cfg.UserAgent != "" {
			opts = append(opts, anthropic.WithUserAgent(cfg.UserAgent))
		}
		provider, err := anthropic.New(opts...)
		return wrapProvider(providerName, provider, err)
	case "openrouter":
		if target.baseURL != "" {
			return nil, fmt.Errorf("llm: provider %q does not support a custom base URL", providerName)
		}
		opts := []openrouter.Option{openrouter.WithAPIKey(cfg.APIKey)}
		if len(headers) > 0 {
			opts = append(opts, openrouter.WithHeaders(headers))
		}
		if cfg.UserAgent != "" {
			opts = append(opts, openrouter.WithUserAgent(cfg.UserAgent))
		}
		provider, err := openrouter.New(opts...)
		return wrapProvider(providerName, provider, err)
	case "openaicompat":
		// The OpenAI-compatible path targets arbitrary servers (including
		// local ones), where the API key is optional.
		var opts []openaicompat.Option
		if cfg.APIKey != "" {
			opts = append(opts, openaicompat.WithAPIKey(cfg.APIKey))
		}
		if target.baseURL != "" {
			opts = append(opts, openaicompat.WithBaseURL(target.baseURL))
		}
		if len(headers) > 0 {
			opts = append(opts, openaicompat.WithHeaders(headers))
		}
		if cfg.UserAgent != "" {
			opts = append(opts, openaicompat.WithUserAgent(cfg.UserAgent))
		}
		provider, err := openaicompat.New(opts...)
		return wrapProvider(providerName, provider, err)
	default:
		// Unreachable: resolveTarget only ever names those four adapters.
		return nil, fmt.Errorf("llm: internal: unknown fantasy adapter %q for provider %q",
			target.adapter, cfg.Provider)
	}
}

// resolveTarget maps cfg.Provider onto a fantasy adapter and endpoint.
//
// The four fantasy-native ids ("openai", "anthropic", "openrouter",
// "openaicompat") use their own adapter with the explicitly configured
// base URL only. Every other id is looked up in the embedded catwalk
// catalog and mapped by the API type it speaks: "completions"
// (OpenAI-compatible) → "openaicompat", "messages" (Anthropic) →
// "anthropic", both pointed at cfg.BaseURL when set, else the catalog's
// endpoint. An API type no adapter in this build speaks (e.g. "responses")
// and an endpoint the catalog leaves empty or as an unresolved "$ENV"
// placeholder both fail with an error naming the provider — no silent
// misconfiguration.
func resolveTarget(cfg Config) (providerTarget, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Provider))
	baseURL := NormalizeBaseURL(cfg.BaseURL)
	switch name {
	case "openai", "anthropic", "openrouter":
		return providerTarget{adapter: name, baseURL: baseURL, needsKey: true}, nil
	case "openaicompat":
		return providerTarget{adapter: name, baseURL: baseURL}, nil
	}
	info, err := ResolveProvider(name)
	if err != nil {
		return providerTarget{}, fmt.Errorf(
			"llm: unsupported provider %q (supported: openai, anthropic, openrouter, openaicompat, or an embedded catalog provider such as deepseek, groq, opencode-go)",
			cfg.Provider)
	}
	var adapter string
	switch info.Type {
	case string(catwalk.TypeCompletions):
		adapter = "openaicompat"
	case string(catwalk.TypeMessages):
		adapter = "anthropic"
	default:
		return providerTarget{}, fmt.Errorf(
			"llm: provider %q (%s) speaks the %q API type, which no adapter in this build supports (supported: %q OpenAI-compatible, %q Anthropic messages)",
			info.Name, info.ID, info.Type, catwalk.TypeCompletions, catwalk.TypeMessages)
	}
	// Endpoint precedence: explicit config > the catalog's API endpoint.
	if baseURL == "" {
		baseURL = NormalizeBaseURL(info.BaseURL)
	}
	if !isHTTPURL(baseURL) {
		return providerTarget{}, fmt.Errorf(
			"llm: provider %q has no usable base URL (%q) — set base_url in the config to the provider's endpoint",
			info.Name, baseURL)
	}
	return providerTarget{
		adapter:  adapter,
		baseURL:  baseURL,
		needsKey: info.APIKeyEnv != "",
	}, nil
}

// isHTTPURL reports whether s is an absolute http(s) URL. Catalog endpoints
// can be empty or unresolved "$ENV" placeholders (azure, gemini); those
// must fail here instead of reaching a provider request.
func isHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// endpointSuffixes are the path endings the SDKs append to a base URL:
// openai-go appends "chat/completions", the Anthropic SDK "v1/messages"
// (with "/messages" as the shorter form some proxies document). Checked
// longest first so "/v1/messages" is not cut down to "/v1".
var endpointSuffixes = []string{"/chat/completions", "/v1/messages", "/messages"}

// NormalizeBaseURL accepts both documented forms of a provider endpoint:
// the base ("…/v1") and the full endpoint ("…/v1/chat/completions" or the
// anthropic-style "…/v1/messages"), which the SDKs would otherwise double
// by appending their own path to it. A recognised endpoint suffix is
// stripped and trailing slashes are trimmed; every other URL (including
// unresolved "$ENV" placeholders and non-URLs) is returned unchanged.
//
// It is exported because `myul doctor` probes "<base>/models" and must
// derive the same base the client will use.
func NormalizeBaseURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return s // not an absolute URL (e.g. an unresolved "$ENV" placeholder)
	}
	path := strings.TrimRight(u.Path, "/")
	for _, suffix := range endpointSuffixes {
		if strings.HasSuffix(path, suffix) {
			path = strings.TrimSuffix(path, suffix)
			break
		}
	}
	if path == u.Path {
		return s // nothing to change: keep the URL byte-for-byte
	}
	u.Path = path
	u.RawPath = ""
	return u.String()
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

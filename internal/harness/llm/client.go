// Package llm is the harness's vendor-neutral conversation layer: the chat
// request/response types, the [Client] interface the game-master loop talks
// to, the production adapter over charm.land/fantasy ([FantasyClient]), and
// the deterministic test double ([ScriptedClient]) that replays canned turns
// for the mock session.
//
// Nothing in this package executes tools. A [ChatResponse] may ask for tool
// calls; the harness dispatches them through its own registry so that every
// mechanical outcome stays auditable (ARCHITECTURE.md §3.2, golden rule 1 in
// AGENTS.md).
package llm

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrNoAPIKey reports that a provider was configured without the API key it
// requires. Errors returned by [NewFantasyClient] wrap it.
var ErrNoAPIKey = errors.New("llm: missing API key")

// ErrProvider reports a failure inside the LLM provider stack. Errors
// returned by [FantasyClient] operations wrap it together with the underlying
// provider error, so callers can test both with [errors.Is].
var ErrProvider = errors.New("llm: provider error")

// Role identifies the speaker of a [Message].
type Role string

const (
	// RoleSystem is the system prompt: persona, mechanics discipline, tool
	// policy and session context. System messages must precede the
	// conversation.
	RoleSystem Role = "system"
	// RoleUser is the player's input.
	RoleUser Role = "user"
	// RoleAssistant is the model's output: narration and/or tool calls.
	RoleAssistant Role = "assistant"
	// RoleTool is a tool result answering an earlier assistant tool call;
	// [Message.ToolCallID] and [Message.Name] identify which call it answers.
	RoleTool Role = "tool"
)

// ToolCall is a model request to invoke one tool by name.
type ToolCall struct {
	// ID is the provider-assigned call identifier, echoed back verbatim in
	// the matching [RoleTool] message.
	ID string `json:"id"`
	// Name is the registered tool to invoke.
	Name string `json:"name"`
	// Args are the call arguments as raw JSON (normally an object). It is
	// not parsed here: dispatching validates it against the tool schema.
	Args json.RawMessage `json:"args,omitempty"`
}

// Message is one entry of a conversation, in the vendor-neutral shape shared
// by every provider.
type Message struct {
	// Role says who speaks.
	Role Role `json:"role"`
	// Content is the text of the message: narration for [RoleUser] and
	// [RoleAssistant], the tool result payload for [RoleTool].
	Content string `json:"content,omitempty"`
	// ToolCalls carries the tools an [RoleAssistant] message invokes;
	// empty for every other role.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID identifies which tool call a [RoleTool] message answers.
	// Meaningful only when Role is [RoleTool].
	ToolCallID string `json:"tool_call_id,omitempty"`
	// Name is the name of the tool that produced a [RoleTool] result.
	// Meaningful only when Role is [RoleTool]; providers address results by
	// ToolCallID, so this is carried for logging and round-trips.
	Name string `json:"name,omitempty"`
}

// ToolSpec advertises one tool the model may call. Tools are advertised, never
// executed, by a [Client].
type ToolSpec struct {
	// Name is the registered tool name (snake_case, matching the harness
	// registry).
	Name string `json:"name"`
	// Description tells the model what the tool is for and when to use it.
	Description string `json:"description"`
	// Schema is the JSON Schema describing the tool's arguments object.
	Schema map[string]any `json:"schema,omitempty"`
}

// ChatRequest is a single turn of conversation sent to a [Client].
type ChatRequest struct {
	// Messages is the full conversation, oldest first: the [RoleSystem]
	// prompt (when present) first, then history, ending with the latest
	// player input or tool results.
	Messages []Message `json:"messages"`
	// Tools are the tools advertised for this request, in registry order.
	Tools []ToolSpec `json:"tools,omitempty"`
}

// Usage reports the token cost of one model call.
type Usage struct {
	// InputTokens counts prompt tokens (system, history, tools).
	InputTokens int `json:"input_tokens,omitempty"`
	// OutputTokens counts generated tokens.
	OutputTokens int `json:"output_tokens,omitempty"`
}

// ChatResponse is the model's reply to a [ChatRequest].
type ChatResponse struct {
	// Message is the model's next assistant message: narration, tool calls,
	// or both. A [Client] never executes the tool calls itself.
	Message Message `json:"message"`
	// Usage reports the token cost of the call when the provider supplies
	// it.
	Usage Usage `json:"usage,omitempty"`
}

// StreamKind discriminates the events a [Client.Stream] callback receives.
type StreamKind string

const (
	// StreamTextDelta carries a fragment of narration in [StreamEvent.Text].
	StreamTextDelta StreamKind = "text_delta"
	// StreamToolCall carries a completed tool call in
	// [StreamEvent.ToolCall].
	StreamToolCall StreamKind = "tool_call"
	// StreamDone terminates the stream and carries cumulative usage in
	// [StreamEvent.Usage].
	StreamDone StreamKind = "done"
)

// StreamEvent is one step of a streamed reply.
type StreamEvent struct {
	// Kind says which field of this event is meaningful.
	Kind StreamKind `json:"kind"`
	// Text holds the fragment when Kind is [StreamTextDelta].
	Text string `json:"text,omitempty"`
	// ToolCall holds the invocation when Kind is [StreamToolCall].
	ToolCall ToolCall `json:"tool_call,omitempty"`
	// Usage holds the call's token usage when Kind is [StreamDone].
	Usage Usage `json:"usage,omitempty"`
}

// Client is the interface the harness uses to converse with an LLM. The
// production implementation is [FantasyClient]; tests use [ScriptedClient].
//
// Contract: an implementation sends the request's full conversation and
// returns (or streams) the model's next assistant message. It never executes
// tools, never mutates game state, and wraps provider failures so
// [errors.Is] matches [ErrProvider].
type Client interface {
	// Chat sends req and returns the model's next assistant message.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)

	// Stream sends req and reports the model's reply incrementally to emit:
	// zero or more [StreamTextDelta] events, then zero or more
	// [StreamToolCall] events, then exactly one [StreamDone]. Returning an
	// error from emit aborts the stream and propagates wrapped.
	Stream(ctx context.Context, req ChatRequest, emit func(StreamEvent) error) error
}

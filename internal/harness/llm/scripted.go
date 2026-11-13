package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrScriptExhausted is returned by [ScriptedClient] once every canned turn
// has been replayed; a mock-session script that hits it is incomplete.
var ErrScriptExhausted = errors.New("llm: scripted client: script exhausted")

// ScriptedTurn is one canned reply. It round-trips through JSON, so mock
// session scripts can live in testdata as {"response": {…}} documents.
type ScriptedTurn struct {
	// Response is what [ScriptedClient.Chat] returns and what
	// [ScriptedClient.Stream] replays event by event.
	Response ChatResponse `json:"response"`
}

// ScriptedClient is a deterministic, mutex-protected [Client] test double:
// Chat and Stream consume [ScriptedTurn]s in order, record every request for
// assertions, and fail with [ErrScriptExhausted] when the script runs out.
type ScriptedClient struct {
	mu       sync.Mutex
	turns    []ScriptedTurn
	next     int
	requests []ChatRequest
}

var _ Client = (*ScriptedClient)(nil)

// NewScripted returns a [ScriptedClient] replaying turns in order. The turns
// are copied; the client is safe for concurrent use and never touches the
// network.
func NewScripted(turns ...ScriptedTurn) *ScriptedClient {
	return &ScriptedClient{turns: append([]ScriptedTurn(nil), turns...)}
}

// Chat records req and returns the next scripted response, or
// [ErrScriptExhausted] when the script is spent. The request is recorded
// even when it fails, so assertions can see the call that broke the script.
func (s *ScriptedClient) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return ChatResponse{}, fmt.Errorf("llm: scripted client: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req)
	if s.next >= len(s.turns) {
		return ChatResponse{}, fmt.Errorf(
			"%w: wanted turn %d of %d", ErrScriptExhausted, s.next+1, len(s.turns))
	}
	turn := s.turns[s.next]
	s.next++
	return turn.Response, nil
}

// Stream records req and replays the next scripted response as events: the
// message content split at line boundaries (concatenating the deltas
// reproduces it exactly), then each tool call, then a single done event with
// the turn's usage. It fails with [ErrScriptExhausted] when the script is
// spent. Events are emitted without holding the lock, so a callback may call
// back into the client.
func (s *ScriptedClient) Stream(ctx context.Context, req ChatRequest, emit func(StreamEvent) error) error {
	if emit == nil {
		return errors.New("llm: scripted client: nil event callback")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("llm: scripted client: %w", err)
	}
	s.mu.Lock()
	s.requests = append(s.requests, req)
	if s.next >= len(s.turns) {
		next, total := s.next, len(s.turns)
		s.mu.Unlock()
		return fmt.Errorf("%w: wanted turn %d of %d", ErrScriptExhausted, next+1, total)
	}
	resp := s.turns[s.next].Response
	s.next++
	s.mu.Unlock()

	for _, chunk := range textChunks(resp.Message.Content) {
		if err := emit(StreamEvent{Kind: StreamTextDelta, Text: chunk}); err != nil {
			return fmt.Errorf("llm: scripted client: stream callback: %w", err)
		}
	}
	for _, call := range resp.Message.ToolCalls {
		if err := emit(StreamEvent{Kind: StreamToolCall, ToolCall: call}); err != nil {
			return fmt.Errorf("llm: scripted client: stream callback: %w", err)
		}
	}
	if err := emit(StreamEvent{Kind: StreamDone, Usage: resp.Usage}); err != nil {
		return fmt.Errorf("llm: scripted client: stream callback: %w", err)
	}
	return nil
}

// Requests returns every request passed to Chat and Stream, in call order.
// The outer slice is copied; the requests themselves are not deep-copied.
func (s *ScriptedClient) Requests() []ChatRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ChatRequest(nil), s.requests...)
}

// LastRequest returns the most recently recorded request, or ok=false when
// nothing has been called yet.
func (s *ScriptedClient) LastRequest() (ChatRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		return ChatRequest{}, false
	}
	return s.requests[len(s.requests)-1], true
}

// textChunks splits content after each newline so a scripted stream arrives
// as several deltas; concatenating the chunks reproduces content exactly.
// Empty content yields no chunks.
func textChunks(content string) []string {
	if content == "" {
		return nil
	}
	var chunks []string
	for content != "" {
		if i := strings.IndexByte(content, '\n'); i >= 0 {
			chunks = append(chunks, content[:i+1])
			content = content[i+1:]
			continue
		}
		chunks = append(chunks, content)
		content = ""
	}
	return chunks
}

package llm

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestScriptedChatReplaysTurnsInOrder(t *testing.T) {
	client := NewScripted(
		ScriptedTurn{Response: ChatResponse{Message: Message{Role: RoleAssistant, Content: "first"}}},
		ScriptedTurn{Response: ChatResponse{Message: Message{Role: RoleAssistant, Content: "second"}}},
	)
	ctx := context.Background()

	for i, want := range []string{"first", "second"} {
		resp, err := client.Chat(ctx, messageWith("go"))
		if err != nil {
			t.Fatalf("Chat %d: %v", i, err)
		}
		if resp.Message.Content != want {
			t.Errorf("Chat %d content = %q, want %q", i, resp.Message.Content, want)
		}
	}

	if _, err := client.Chat(ctx, messageWith("go")); !errors.Is(err, ErrScriptExhausted) {
		t.Fatalf("exhausted Chat error = %v, want ErrScriptExhausted", err)
	}
	// The failing call is still recorded so assertions can see it.
	if got := len(client.Requests()); got != 3 {
		t.Errorf("recorded %d requests, want 3", got)
	}
}

func TestScriptedChatRecordsRequests(t *testing.T) {
	client := NewScripted() // empty script: requests must still be recorded
	if _, ok := client.LastRequest(); ok {
		t.Error("LastRequest ok on a fresh client, want false")
	}

	reqA := ChatRequest{Messages: []Message{{Role: RoleSystem, Content: "sys"}, {Role: RoleUser, Content: "a"}}}
	reqB := ChatRequest{Messages: []Message{{Role: RoleUser, Content: "b"}}}
	if _, err := client.Chat(context.Background(), reqA); !errors.Is(err, ErrScriptExhausted) {
		t.Fatalf("Chat on empty script: %v", err)
	}
	if _, err := client.Chat(context.Background(), reqB); !errors.Is(err, ErrScriptExhausted) {
		t.Fatalf("Chat on empty script: %v", err)
	}

	requests := client.Requests()
	if len(requests) != 2 {
		t.Fatalf("Requests() = %d entries, want 2", len(requests))
	}
	if !reflect.DeepEqual(requests[0], reqA) || !reflect.DeepEqual(requests[1], reqB) {
		t.Errorf("recorded requests = %#v, want %#v", requests, []ChatRequest{reqA, reqB})
	}
	last, ok := client.LastRequest()
	if !ok || !reflect.DeepEqual(last, reqB) {
		t.Errorf("LastRequest = (%#v, %v), want reqB", last, ok)
	}
}

func TestScriptedChatHonoursCancelledContext(t *testing.T) {
	client := NewScripted(ScriptedTurn{Response: ChatResponse{Message: Message{Role: RoleAssistant, Content: "hi"}}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Chat(ctx, messageWith("go")); !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want wrapped context.Canceled", err)
	}
	// A cancelled call never reached the script: the turn is unconsumed and
	// nothing is recorded.
	if got := len(client.Requests()); got != 0 {
		t.Errorf("recorded %d requests for a cancelled call, want 0", got)
	}
	resp, err := client.Chat(context.Background(), messageWith("go"))
	if err != nil {
		t.Fatalf("Chat after cancel: %v", err)
	}
	if resp.Message.Content != "hi" {
		t.Errorf("content = %q, want the scripted turn still available", resp.Message.Content)
	}
}

func TestScriptedStreamReplaysTurnAsEvents(t *testing.T) {
	content := "Line one.\nLine two.\n"
	usage := Usage{InputTokens: 11, OutputTokens: 22}
	toolCall := ToolCall{ID: "c1", Name: "note", Args: json.RawMessage(`{"text":"remember"}`)}
	client := NewScripted(ScriptedTurn{Response: ChatResponse{
		Message: Message{Role: RoleAssistant, Content: content, ToolCalls: []ToolCall{toolCall}},
		Usage:   usage,
	}})

	var events []StreamEvent
	err := client.Stream(context.Background(), messageWith("go"), func(ev StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("events = %d, want 4 (2 deltas, tool call, done): %#v", len(events), events)
	}
	var text strings.Builder
	for i, ev := range events[:2] {
		if ev.Kind != StreamTextDelta {
			t.Errorf("event[%d].Kind = %q, want text delta", i, ev.Kind)
		}
		text.WriteString(ev.Text)
	}
	if text.String() != content {
		t.Errorf("concatenated deltas = %q, want %q", text.String(), content)
	}
	if events[2].Kind != StreamToolCall || events[2].ToolCall.ID != toolCall.ID {
		t.Errorf("event[2] = %+v, want the scripted tool call", events[2])
	}
	done := events[3]
	if done.Kind != StreamDone || done.Usage != usage {
		t.Errorf("event[3] = %+v, want done with usage %+v", done, usage)
	}
	if _, ok := client.LastRequest(); !ok {
		t.Error("Stream did not record its request")
	}
}

func TestScriptedStreamErrors(t *testing.T) {
	ctx := context.Background()

	// Exhausted script.
	err := NewScripted().Stream(ctx, messageWith("go"), func(StreamEvent) error { return nil })
	if !errors.Is(err, ErrScriptExhausted) {
		t.Errorf("exhausted stream error = %v, want ErrScriptExhausted", err)
	}

	// Nil callback.
	err = NewScripted(ScriptedTurn{}).Stream(ctx, messageWith("go"), nil)
	if err == nil || !strings.Contains(err.Error(), "nil event callback") {
		t.Errorf("nil callback error = %v", err)
	}

	// Callback failure propagates wrapped and stops the replay.
	listenerGone := errors.New("listener gone")
	client := NewScripted(ScriptedTurn{Response: ChatResponse{
		Message: Message{Role: RoleAssistant, Content: "a\nb\n"},
	}})
	seen := 0
	err = client.Stream(ctx, messageWith("go"), func(StreamEvent) error {
		seen++
		return listenerGone
	})
	if !errors.Is(err, listenerGone) {
		t.Errorf("callback error = %v, want wrapped %v", err, listenerGone)
	}
	if seen != 1 {
		t.Errorf("callback invoked %d times, want 1", seen)
	}
}

func TestScriptedTurnJSONRoundTrip(t *testing.T) {
	turn := ScriptedTurn{Response: ChatResponse{
		Message: Message{
			Role:       RoleAssistant,
			Content:    "The GM speaks.",
			ToolCalls:  []ToolCall{{ID: "call_1", Name: "skill_test", Args: json.RawMessage(`{"skill":"Stealth"}`)}},
			ToolCallID: "",
			Name:       "",
		},
		Usage: Usage{InputTokens: 7, OutputTokens: 3},
	}}

	data, err := json.Marshal(turn)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back ScriptedTurn
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(turn, back) {
		t.Errorf("round trip lost data:\n in: %#v\nout: %#v", turn, back)
	}
	if !strings.Contains(string(data), `"skill_test"`) {
		t.Errorf("marshalled JSON %s does not look like a script document", data)
	}
}

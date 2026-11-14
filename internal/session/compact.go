// SPDX-License-Identifier: EUPL-1.2

package session

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/tui"
)

const (
	// contextPrefix tags the harness-written summary that heads a windowed
	// history. It is a user message, not a system message, because
	// providers require system messages to precede the conversation —
	// llm.FantasyClient rejects one that arrives mid-history.
	contextPrefix = "[context] earlier turns, condensed for continuity: "

	// summarisationPrompt is the one-shot instruction compaction sends
	// through the same llm.Client before the turns it is about to drop.
	summarisationPrompt = "You are condensing the earlier part of a TTRPG session's " +
		"conversation history into a single continuity note for the game master. " +
		"Output only that note, in plain prose under 200 words. Preserve every name, " +
		"place, promise, decision, number and open thread the GM must not contradict; " +
		"drop dialogue and scene-setting that changed nothing."
)

// compact enforces Config.HistoryWindow (session-loop.md "Context
// compaction"): when the history outgrows the window, the older turns are
// summarised in one extra model call and replaced by that summary at the
// front of the history, while the last window stays verbatim. If the
// summarisation fails, everything is kept verbatim and a note lands in the
// transcript instead — compaction is an optimisation, never a data loss.
// It runs after each completed turn, on the runner goroutine, the only
// writer of s.history.
func (s *Session) compact() {
	if len(s.history) <= s.window {
		return
	}
	start := len(s.history) - s.window
	// Never window into the middle of a tool exchange: a history opening
	// on a RoleTool message has no assistant call to answer it, and
	// providers reject that shape.
	for start > 0 && s.history[start].Role == llm.RoleTool {
		start--
	}
	older, kept := s.history[:start], s.history[start:]
	summary, err := s.summarise(older)
	if err != nil {
		s.emit(tui.Event{Kind: tui.EventSystem, Markdown: fmt.Sprintf(
			"context compaction skipped (%v) — keeping all %d messages verbatim", err, len(s.history))})
		return
	}
	windowed := make([]llm.Message, 0, len(kept)+1)
	windowed = append(windowed, llm.Message{Role: llm.RoleUser, Content: contextPrefix + summary})
	windowed = append(windowed, kept...)
	s.history = windowed
}

// summarise condenses older turns in one Chat call through the same client.
// An empty or failed reply is an error, so the caller keeps the turns
// verbatim rather than replacing them with nothing.
func (s *Session) summarise(older []llm.Message) (string, error) {
	if s.client == nil {
		return "", ErrNoClient
	}
	msgs := make([]llm.Message, 0, len(older)+1)
	msgs = append(msgs, llm.Message{Role: llm.RoleSystem, Content: summarisationPrompt})
	msgs = append(msgs, older...)
	resp, err := s.client.Chat(s.ctx, llm.ChatRequest{Messages: msgs})
	if err != nil {
		return "", fmt.Errorf("summarise earlier turns: %w", err)
	}
	summary := strings.TrimSpace(resp.Message.Content)
	if summary == "" {
		return "", errors.New("summarise earlier turns: the model returned an empty summary")
	}
	return summary, nil
}

// SPDX-License-Identifier: EUPL-1.2

package session

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/state"
	"github.com/mythras-ultra/myul/internal/tui"
)

// chatLogKind is the LogEntry.Kind play text is recorded under.
const chatLogKind = "chat"

// Recorded chat kinds — the neutral strings log.jsonl carries; the session
// maps them to and from tui.EventKind (state stays neutral about the TUI).
const (
	chatKindNarrative = "narrative"
	chatKindError     = "error"
)

// saveChat marshals the conversation history to chat.json (atomic write).
// It runs at every autosaved turn end, on /save and on Close — those are
// the points where the in-memory history is known-good (session-loop.md
// "Chat persistence").
func (s *Session) saveChat() error {
	data, err := json.Marshal(s.history)
	if err != nil {
		return fmt.Errorf("session: encode conversation history: %w", err)
	}
	if err := s.store.SaveChat(s.state.ID, data); err != nil {
		return fmt.Errorf("session: save chat: %w", err)
	}
	return nil
}

// loadChat seeds the conversation history from chat.json on Resume, before
// the runner starts. A missing or corrupt file (or bytes that do not decode
// as a conversation) continues with an empty history — the system prompt
// still carries the recaps and the mechanical state rebuilt every turn, so
// the GM keeps its context — and the reason is queued as one system event,
// never swallowed.
func (s *Session) loadChat() {
	var history []llm.Message
	var err error
	if data, derr := s.store.LoadChat(s.state.ID); derr != nil {
		err = derr
	} else if uerr := json.Unmarshal(data, &history); uerr != nil {
		err = fmt.Errorf("decode saved conversation: %w", uerr)
	} else {
		s.history = history
		return
	}
	s.replay = append(s.replay, tui.Event{Kind: tui.EventSystem, Markdown: fmt.Sprintf(
		"saved conversation unavailable (%v) — resuming with an empty history; recaps and game state are intact",
		err)})
}

// loadReplay rebuilds the prior play from log.jsonl at construction
// (Resume): every chat entry becomes the event it was recorded from, in
// recorded order, carrying its original timestamp, queued after any
// resume notice loadChat added. The runner delivers the queue before it
// processes any input (run), so the TUI shows the play history
// immediately — and because they come from the log they bypass emitChat
// entirely: replay never re-records. A log that cannot be read costs the
// replay only (the reason joins the queue as a system event); the
// transcript on disk is unaffected.
func (s *Session) loadReplay() {
	entries, err := s.store.ReadLog(s.state.ID)
	if err != nil {
		s.replay = append(s.replay, tui.Event{Kind: tui.EventSystem, Markdown: fmt.Sprintf(
			"could not replay the earlier play (%v) — the recorded transcript on disk is unaffected", err)})
		return
	}
	for _, e := range entries {
		if e.Chat == nil {
			continue
		}
		s.replay = append(s.replay, tui.Event{
			Kind:     replayKind(e.Chat.Kind),
			Channel:  replayChannel(e.Chat.Channel),
			Markdown: e.Chat.Text,
			At:       e.At,
		})
	}
}

// emitChat delivers one play-text event — the player's echo, the GM's
// narration or an error line — to the TUI and records it in log.jsonl as
// a chat line. Recording happens inline on the runner goroutine, the same
// one that records moves, which is what keeps the log in true play order.
// Play text is the only thing chat-recorded: system confirmations,
// thinking, turn-end, annotations and dice are transient (the moves they
// summarise are already in the log). A record failure surfaces as an
// error line that is deliberately not recorded itself — the append that
// just failed would fail again, so recursing there only spins; later move
// records keep failing loudly on their own.
func (s *Session) emitChat(ev tui.Event, player bool) {
	s.emit(ev)
	kind := chatKindNarrative
	if ev.Kind == tui.EventError {
		kind = chatKindError
	}
	entry := state.LogEntry{Kind: chatLogKind, Chat: &state.ChatLine{
		Channel: ev.Channel.String(),
		Player:  player,
		Kind:    kind,
		Text:    ev.Markdown,
	}}
	if err := s.store.Record(s.state.ID, entry); err != nil {
		s.emit(tui.Event{Kind: tui.EventError, Channel: ev.Channel,
			Markdown: fmt.Sprintf("record chat line: %v", err)})
	}
}

// replayKind maps a recorded chat kind back to its event kind; an empty
// kind (defensive — every recorded line carries one) reads as narrative.
func replayKind(kind string) tui.EventKind {
	if kind == chatKindError {
		return tui.EventError
	}
	return tui.EventNarrative
}

// replayChannel maps a recorded channel label back to its channel;
// anything that is not "OOC" plays back on the in-character channel.
func replayChannel(label string) tui.Channel {
	if strings.EqualFold(label, "OOC") {
		return tui.ChannelOOC
	}
	return tui.ChannelIC
}

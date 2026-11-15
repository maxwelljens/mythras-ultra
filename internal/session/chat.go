// SPDX-License-Identifier: EUPL-1.2

package session

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mythras-ultra/myul/internal/engine/resolve"
	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/state"
	"github.com/mythras-ultra/myul/internal/tui"
)

// chatLogKind is the LogEntry.Kind play text is recorded under.
const chatLogKind = "chat"

// Recorded chat kinds — the neutral strings log.jsonl carries; the session
// maps them to and from tui.EventKind (state stays neutral about the TUI).
// The golden-rule rewrite adds two of its own — retract (a tombstone
// hiding one MsgSeq's lines) and note (the muted remark left behind by an
// accepted rewrite) — whose shapes state itself understands, so the
// session uses state.ChatKindRetract / state.ChatKindNote directly.
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
// (Resume): every chat line becomes the event it was recorded from, every
// recorded dice roll comes back as a finished banner, and — with
// Config.ReplayTools — the tool annotation that preceded those dice, all
// in recorded order, carrying their original timestamps and the Replay
// flag the TUI renders history with. The runner delivers the queue before
// it processes any input (run), so the TUI shows the play history
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
	s.replay = append(s.replay, replayEvents(entries, s.replayTools)...)
}

// replayResult is what a log entry's Result carries: a tool result states
// OK, while the session-start entry records a plain resolve.Result, which
// has no OK field — its absence marks the entry as something other than a
// tool dispatch, so it earns no annotation on replay.
type replayResult struct {
	OK      *bool          `json:"ok"`
	Summary string         `json:"summary"`
	Facts   []string       `json:"facts"`
	Audit   *resolve.Audit `json:"audit,omitempty"`
}

// replayEvents turns recorded log entries into the events Resume queues —
// the pure half of loadReplay, shared with the tests so the queue they
// drain is the queue the runner delivers:
//
//   - every chat line, as the event it was recorded from — collected
//     in two phases: the "retract" tombstones name the MsgSeqs whose
//     lines a golden-rule rewrite superseded, and those lines (and the
//     tombstones themselves) are skipped, so the replay shows only the
//     final version of a rewritten message while every other line plays
//     back as recorded;
//   - every result's audit rolls, always — resume shows the rolls that
//     happened as finished banners (Replay stops the TUI performing them
//     again), with the tool annotation in front of its dice when
//     replayTools is set (config ui.replay_tools);
//   - nothing else: annotations and dice are transient, so only what a
//     Result carries can be replayed. Golden-rule audit entries carry no
//     Result, so they never replay either — the note does.
//
// Entries keep log order, and annotation + dice stay together per entry.
// Moves carry no channel, so dice and annotations follow the channel of
// the chat line last recorded before them. An undecodable Result costs
// that entry's replay only and says so as a system event.
func replayEvents(entries []state.LogEntry, replayTools bool) []tui.Event {
	var (
		out       []tui.Event
		ch        tui.Channel // moves record no channel: follow the play around them
		retracted = retractedMsgSeqs(entries)
	)
	for _, e := range entries {
		if e.Chat != nil {
			c := e.Chat
			if c.Kind == state.ChatKindRetract || (c.MsgSeq != 0 && retracted[c.MsgSeq]) {
				continue // hidden by (or being) a rewrite tombstone
			}
			ch = replayChannel(c.Channel)
			out = append(out, tui.Event{
				Kind:     replayKind(c.Kind),
				Channel:  ch,
				Markdown: c.Text,
				At:       e.At,
				Player:   c.Player,
				MsgSeq:   c.MsgSeq,
				Replay:   true,
			})
			continue
		}
		if len(e.Result) == 0 {
			continue
		}
		var res replayResult
		if err := json.Unmarshal(e.Result, &res); err != nil {
			out = append(out, tui.Event{Kind: tui.EventSystem, At: e.At, Replay: true,
				Markdown: fmt.Sprintf("could not replay the dice of the recorded %q entry (%v) — the recorded transcript on disk is unaffected",
					e.Kind, err)})
			continue
		}
		if replayTools && res.OK != nil {
			out = append(out, tui.Event{
				Kind:    tui.EventToolAnnotation,
				Channel: ch,
				At:      e.At,
				Replay:  true,
				Annotation: &tui.Annotation{
					ToolName: e.Kind,
					Summary:  res.Summary,
					OK:       *res.OK,
				},
			})
		}
		for _, de := range diceEvents(res.Audit, res.Facts) {
			ev := de
			out = append(out, tui.Event{Kind: tui.EventDice, Channel: ch, At: e.At,
				Replay: true, Dice: &ev})
		}
	}
	return out
}

// emitChat delivers one play-text event — the player's echo, the GM's
// narration or an error line — to the TUI and records it in log.jsonl as
// a chat line. Recording happens inline on the runner goroutine, the same
// one that records moves, which is what keeps the log in true play order.
// Play text is the only thing chat-recorded: system confirmations,
// thinking, turn-end, annotations and dice are transient (the moves they
// summarise are already in the log).
func (s *Session) emitChat(ev tui.Event, player bool) {
	s.emit(ev)
	s.recordChat(ev, player)
}

// recordChat appends one play-text event to log.jsonl as a chat line — the
// split-out half of emitChat, for streamed narrative whose display batches
// were emitted before its entry closed. A record failure surfaces as an
// error line that is deliberately not recorded itself — the append that
// just failed would fail again, so recursing there only spins; later move
// records keep failing loudly on their own.
func (s *Session) recordChat(ev tui.Event, player bool) {
	kind := chatKindNarrative
	if ev.Kind == tui.EventError {
		kind = chatKindError
	}
	entry := state.LogEntry{Kind: chatLogKind, Chat: &state.ChatLine{
		Channel: ev.Channel.String(),
		Player:  player,
		Kind:    kind,
		MsgSeq:  ev.MsgSeq,
		Text:    ev.Markdown,
	}}
	if err := s.store.Record(s.state.ID, entry); err != nil {
		s.emit(tui.Event{Kind: tui.EventError, Channel: ev.Channel,
			Markdown: fmt.Sprintf("record chat line: %v", err)})
	}
}

// recordRetract appends the golden-rule rewrite's tombstone: an empty chat
// line naming the MsgSeq whose recorded lines a rewritten reply supersedes.
// The log stays append-only — the transcript and the replay collect these
// first and hide exactly the lines they name. A record failure surfaces as
// an error line that is deliberately not recorded itself (see recordChat).
func (s *Session) recordRetract(ch tui.Channel, msgSeq int) {
	entry := state.LogEntry{Kind: chatLogKind, Chat: &state.ChatLine{
		Channel: ch.String(),
		Kind:    state.ChatKindRetract,
		MsgSeq:  msgSeq,
		Text:    "",
	}}
	if err := s.store.Record(s.state.ID, entry); err != nil {
		s.emit(tui.Event{Kind: tui.EventError, Channel: ch,
			Markdown: fmt.Sprintf("record rewrite tombstone: %v", err)})
	}
}

// emitGoldenNote shows the golden-rule check's one visible mark: the muted
// system line left behind when a rewritten reply is finally standing. It
// is recorded as its own note chat line (with the rewritten message's
// MsgSeq), so replay shows it as a system event. A record failure surfaces
// as an error line (see recordChat).
func (s *Session) emitGoldenNote(ch tui.Channel, msgSeq int) {
	s.emit(tui.Event{Kind: tui.EventSystem, Channel: ch,
		Markdown: goldenRuleNote, MsgSeq: msgSeq})
	entry := state.LogEntry{Kind: chatLogKind, Chat: &state.ChatLine{
		Channel: ch.String(),
		Kind:    state.ChatKindNote,
		MsgSeq:  msgSeq,
		Text:    goldenRuleNote,
	}}
	if err := s.store.Record(s.state.ID, entry); err != nil {
		s.emit(tui.Event{Kind: tui.EventError, Channel: ch,
			Markdown: fmt.Sprintf("record rewrite note: %v", err)})
	}
}

// replayKind maps a recorded chat kind back to its event kind; an empty
// kind (defensive — every recorded line carries one) reads as narrative.
// A golden-rule note comes back as a system event; a retract tombstone
// never reaches replayEvents' output at all (it hides lines instead).
func replayKind(kind string) tui.EventKind {
	switch kind {
	case chatKindError:
		return tui.EventError
	case state.ChatKindNote:
		return tui.EventSystem
	default:
		return tui.EventNarrative
	}
}

// retractedMsgSeqs collects, in one pass over the log, the MsgSeqs the
// "retract" tombstones hide — the two-phase groundwork replay and
// transcript share: collect first, then render chronologically, so later
// lines with a different MsgSeq always show. A tombstone with MsgSeq 0
// (legacy — never written) retracts nothing.
func retractedMsgSeqs(entries []state.LogEntry) map[int]bool {
	var out map[int]bool
	for _, e := range entries {
		if e.Chat == nil || e.Chat.Kind != state.ChatKindRetract || e.Chat.MsgSeq == 0 {
			continue
		}
		if out == nil {
			out = make(map[int]bool)
		}
		out[e.Chat.MsgSeq] = true
	}
	return out
}

// replayChannel maps a recorded channel label back to its channel;
// anything that is not "OOC" plays back on the in-character channel.
func replayChannel(label string) tui.Channel {
	if strings.EqualFold(label, "OOC") {
		return tui.ChannelOOC
	}
	return tui.ChannelIC
}

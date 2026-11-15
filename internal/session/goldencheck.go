// SPDX-License-Identifier: EUPL-1.2

package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mythras-ultra/myul/internal/harness/decide"
	"github.com/mythras-ultra/myul/internal/harness/llm"
	"github.com/mythras-ultra/myul/internal/state"
	"github.com/mythras-ultra/myul/internal/tui"
)

// The golden-rule check (docs/design/session-loop.md "Golden rule check"):
// after an in-character reply's tool calls have run and before the reply
// enters history, the decision model is asked whether it decided the
// player's character's actions. The gate is the channel check's pattern —
// a "violates" verdict at channelCheckMinConfidence (0.70) and above
// prompts, everything else passes silently.
const (
	// goldenCheckQuestion is the decision-model question about one Game
	// Master reply. It names the violation (deciding the character the
	// player plays) and the passing condition in one sentence, so the
	// two offered verdicts are unambiguous.
	goldenCheckQuestion = "Does this Game Master reply decide what the player's character does — choosing or narrating the character's actions the player never chose? It complies if the player keeps all choices."
	// goldenOptionComplies / goldenOptionViolates are the two verdicts;
	// the check acts only on the latter.
	goldenOptionComplies = "complies"
	goldenOptionViolates = "violates"
	// goldenRegenPrompt is the exact question [EventConfirmRegen] files:
	// the violation is stated, then the answer defaults to y ("Y/n"), the
	// inverse of the channel check's held-message "N/y" — Enter
	// regenerates here, Enter discards there.
	goldenRegenPrompt = "The Game Master appears to have decided your character's actions for you. This is a violation of a golden rule in roleplaying. Regenerate Game Master's response? Y/n"
	// goldenRewriteNudge is the corrective message appended after the
	// reply under judgement when the player asks for a regeneration. It
	// orders a narration-only rewrite, leaves every choice with the
	// player, and demands prose alone: the original reply's tool calls
	// already ran, and any tool calls the rewrite returns anyway are
	// stripped (never dispatched, never recorded).
	goldenRewriteNudge = "Rewrite the Game Master reply shown above. Rewrite ONLY the narration: never decide the player character's actions for the player — choosing or narrating what they do — because the player keeps every choice. Emit no tool calls."
	// goldenRuleNote is the muted line left behind when a rewritten reply
	// is finally standing: one note per rewritten message, replayed as a
	// system event (state.ChatKindNote).
	goldenRuleNote = "↩ reply rewritten (golden rule)"
)

// Decisions the golden-rule check records in its audit entries — what the
// check did with the reply it judged, decided once the loop resolves it.
const (
	goldenDecisionPass  = "pass"  // no prompt: complies, or under the confidence floor
	goldenDecisionRegen = "regen" // the player answered y: the reply was regenerated
	goldenDecisionKeep  = "keep"  // the player answered n (or a rewrite failed): the reply on screen stands
)

// goldenCheckMove is one golden_rule_check audit entry's Move: which
// assistant message was judged, what the decision model answered, and what
// the check did about it. Attempt counts the replies one gate judged:
// 1 = the reply as streamed, +1 per regeneration of it.
type goldenCheckMove struct {
	MsgSeq     int     `json:"msg_seq"`
	Verdict    string  `json:"verdict"`
	Confidence float64 `json:"confidence"`
	Decision   string  `json:"decision"`
	Attempt    int     `json:"attempt"`
}

// latestPlayerMessage returns the most recent player message from the
// conversation history — the context the golden-rule check judges (the
// turn under way appended its own message before the model was called,
// so it is always the latest on an in-character turn). Empty when history
// carries none (defensive: the check then passes silently).
func (s *Session) latestPlayerMessage() string {
	for i := len(s.history) - 1; i >= 0; i-- {
		if s.history[i].Role == llm.RoleUser {
			return s.history[i].Content
		}
	}
	return ""
}

// goldenCheckState renders one golden-rule check's decision-model context:
// the player's latest message, then the sequence of the reply under
// judgement. The sequence keeps every check a fresh question for decide's
// per-question verdict cache (keyed by State+Question+Options): each
// rewrite streams a reply with its own MsgSeq, so the re-check can reach
// the model — without it, the cache would repeat the first "violates"
// verdict forever and a clean rewrite could never pass.
func goldenCheckState(player string, msgSeq int) string {
	return player + fmt.Sprintf("\n\nReply under judgement: %d", msgSeq)
}

// goldenCheck gates one assistant reply (session-loop.md "Golden rule
// check"). It is a no-op unless the turn is in character, the check is on
// (Config GMCheck), a decision client exists and the reply carries text —
// each of those passes silently without a decide call, like the channel
// check, so the gate can never fail a turn.
//
// A "violates" verdict at channelCheckMinConfidence confidence files the
// exact "Y/n" regeneration prompt and waits for Command("confirm", …):
// y re-asks the model OFF-SCREEN (requestMessages + the original reply —
// text and the tool calls it already ran — + goldenRewriteNudge; the
// rewrite's own tool calls are stripped), swaps the rewrite into the
// transcript as one Replace event, tombstones the replaced message's chat
// lines and re-checks the rewrite — still violating means the same prompt
// again, up to the player (n keeps whatever is on screen, without a mark).
// A clean rewrite (or the player's n) ends the loop; a rewritten reply
// that ends up standing leaves the one goldenRuleNote behind. Discarded
// drafts never enter history: the caller appends only the reply returned
// here, carrying the ORIGINAL reply's tool calls.
//
// Every completed check is recorded as a golden_rule_check LogEntry
// (raw audit; the transcript renderer skips the kind). A decide call that
// errors — disabled, unreachable — passes silently and is not recorded:
// there is no verdict to audit.
func (s *Session) goldenCheck(ch tui.Channel, prompt string, res reply) reply {
	if ch != tui.ChannelIC || !s.gmCheck || s.decision == nil ||
		strings.TrimSpace(res.text) == "" {
		return res
	}
	player := s.latestPlayerMessage()
	if strings.TrimSpace(player) == "" {
		return res
	}
	original := res // the reply the rewrite request always rewrites
	rewritten := false
	for attempt := 1; ; attempt++ {
		msgSeq := s.msgSeq // the sequence of the reply under judgement
		verdict, err := s.decision.Decide(s.ctx, decide.Question{
			State:    goldenCheckState(player, msgSeq),
			Question: goldenCheckQuestion,
			Options:  []string{goldenOptionComplies, goldenOptionViolates},
		})
		if err != nil {
			return s.finishGolden(ch, res, rewritten) // silent pass, no verdict to audit
		}
		if verdict.Choice != goldenOptionViolates ||
			verdict.Confidence < channelCheckMinConfidence {
			s.recordGoldenCheck(ch, msgSeq, verdict, goldenDecisionPass, attempt)
			return s.finishGolden(ch, res, rewritten)
		}
		if answer := s.waitRegen(ch); answer != "y" {
			// n keeps the reply on screen; a Close during the prompt
			// keeps it too — the shutdown has no player left to ask.
			s.recordGoldenCheck(ch, msgSeq, verdict, goldenDecisionKeep, attempt)
			return s.finishGolden(ch, res, rewritten)
		}
		next, rerr := s.rewriteOffscreen(ch, prompt, original)
		if rerr != nil {
			s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch,
				Markdown: fmt.Sprintf("regenerate the reply: %v — keeping the reply on screen", rerr)}, false)
			s.recordGoldenCheck(ch, msgSeq, verdict, goldenDecisionKeep, attempt)
			return s.finishGolden(ch, res, rewritten)
		}
		s.recordGoldenCheck(ch, msgSeq, verdict, goldenDecisionRegen, attempt)
		// Swap the rewrite in: tombstone the lines of the reply it
		// replaces, then file the full text once — one event, one chat
		// line, the new MsgSeq.
		s.recordRetract(ch, msgSeq)
		s.emitChat(tui.Event{Kind: tui.EventNarrative, Channel: ch,
			Markdown: stripChannelTag(next.text), Replace: true,
			ReplaceSeq: msgSeq, MsgSeq: s.msgSeq}, false)
		res = reply{text: next.text, calls: original.calls}
		rewritten = true
	}
}

// finishGolden closes one gate pass: an accepted rewrite leaves its one
// muted note behind (the reply standing is not the one that streamed), a
// reply that never changed leaves nothing.
func (s *Session) finishGolden(ch tui.Channel, res reply, rewritten bool) reply {
	if rewritten {
		s.emitGoldenNote(ch, s.msgSeq)
	}
	return res
}

// rewriteRequest builds the off-screen regeneration request: the system
// prompt, the conversation history (the reply under judgement has not
// entered it yet), the original reply verbatim — its text and the tool
// calls it already ran — and the corrective nudge. No tools are advertised
// (the rewrite is prose only; any calls it returns are stripped anyway).
func (s *Session) rewriteRequest(prompt string, original reply) llm.ChatRequest {
	msgs := s.requestMessages(prompt)
	msgs = append(msgs,
		llm.Message{Role: llm.RoleAssistant, Content: original.text, ToolCalls: original.calls},
		llm.Message{Role: llm.RoleUser, Content: goldenRewriteNudge},
	)
	return llm.ChatRequest{Messages: msgs}
}

// rewriteOffscreen re-asks the model for a clean rewrite of the original
// reply, showing and recording nothing (callModelMode's offscreen path).
// Its own tool calls are stripped — the original's already ran and are the
// only ones history may carry — and an empty rewrite is a failure the
// caller reports, never a reply that could replace good text with nothing.
func (s *Session) rewriteOffscreen(ch tui.Channel, prompt string, original reply) (reply, error) {
	res, err := s.callModelMode(ch, s.rewriteRequest(prompt, original), true)
	if err != nil {
		return reply{}, err
	}
	if strings.TrimSpace(res.text) == "" {
		return reply{}, errors.New("the model returned an empty rewrite")
	}
	res.calls = nil
	return res, nil
}

// waitRegen holds the runner until the player answers the regeneration
// prompt: it emits the [EventConfirmRegen] carrying the exact "Y/n"
// question (no draft text — the reply on screen is what n keeps), then
// blocks on Command("confirm", "y"|"n") or Close, exactly like the
// channel check's hold. Inputs queue meanwhile; Close discards the
// prompt without hanging.
func (s *Session) waitRegen(ch tui.Channel) string {
	s.held.Store(true)
	defer s.held.Store(false)
	s.emit(tui.Event{Kind: tui.EventConfirmRegen, Channel: ch,
		Markdown: goldenRegenPrompt})
	select {
	case answer := <-s.confirmCh:
		return answer
	case <-s.quit:
		return ""
	}
}

// recordGoldenCheck appends one golden_rule_check entry to log.jsonl: the
// verdict the decision model gave and what the check did with it, for the
// message sequence it judged. The transcript renderer skips this kind —
// the play shows only the rewrite's note — but the raw log keeps every
// check, including the ones that passed. A record failure surfaces as an
// error line (see recordChat).
func (s *Session) recordGoldenCheck(ch tui.Channel, msgSeq int, verdict decide.Verdict, decision string, attempt int) {
	s.mu.Lock()
	seq := s.state.EventSeq
	s.mu.Unlock()
	move, err := json.Marshal(goldenCheckMove{
		MsgSeq:     msgSeq,
		Verdict:    verdict.Choice,
		Confidence: verdict.Confidence,
		Decision:   decision,
		Attempt:    attempt,
	})
	if err != nil {
		// Unreachable for these scalar fields, but a swallowed error
		// would silently drop the audit entry (AGENTS.md golden rules).
		s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch,
			Markdown: fmt.Sprintf("encode golden-rule check: %v", err)}, false)
		return
	}
	if err := s.store.Record(s.state.ID, state.LogEntry{
		Seq: seq, Kind: state.GoldenRuleCheckKind, Move: move,
	}); err != nil {
		s.emitChat(tui.Event{Kind: tui.EventError, Channel: ch,
			Markdown: fmt.Sprintf("record golden-rule check: %v", err)}, false)
	}
}

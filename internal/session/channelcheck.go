// SPDX-License-Identifier: EUPL-1.2

package session

import (
	"github.com/mythras-ultra/myul/internal/harness/decide"
	"github.com/mythras-ultra/myul/internal/tui"
)

const (
	// channelCheckMinConfidence is the lowest decision-model confidence
	// that makes the channel check hold a message: a verdict disagreeing
	// with the current IC/OOC mode prompts "N/y" at 0.70 and above, and
	// sends the message silently below it.
	channelCheckMinConfidence = 0.7
	// channelCheckMinLen is the shortest player message, in bytes, the
	// channel check examines: a message of 31 characters or fewer is
	// echoed without ever consulting the decision model.
	channelCheckMinLen = 32
)

// The channel check's decision-model question (docs/design/session-loop.md
// "Channel check"): the mode sentence plus the message itself, so the model
// judges the text — and the per-state verdict cache cannot answer a later
// send with an earlier message's verdict.
const (
	channelCheckMode      = "The player is currently in the in-character / out-of-character chat mode."
	channelCheckQuestion  = "Which channel does this player message belong to?"
	channelCheckPromptIC  = "You are in IC mode, but your response looks like a message for OOC. Is this correct? N/y"
	channelCheckPromptOOC = "You are in OOC mode, but your response looks like a message for IC. Is this correct? N/y"
)

// channelOption maps a channel onto the option label the decision model
// answers with; the current channel's label is the "correct" option the
// verdict is compared against.
func channelOption(ch tui.Channel) string {
	if ch == tui.ChannelOOC {
		return "out-of-character"
	}
	return "in-character"
}

// channelCheckState renders one channel check's decision-model context: the
// pinned mode sentence followed by the message under judgement.
func channelCheckState(text string) string {
	return channelCheckMode + "\n\nPlayer message:\n" + text
}

// channelGate consults the decision model about one player message and
// reports the confirmation question to ask when the verdict disagrees with
// the current channel. It asks only when the check is on (Config
// ChannelCheck), a decision client exists and the message reaches
// channelCheckMinLen; a decision model that cannot be consulted — disabled,
// unreachable, or answering badly — returns no question, so the gate never
// blocks a message and never fails a turn. Confidence below
// channelCheckMinConfidence and a verdict matching the channel pass
// silently too.
func (s *Session) channelGate(ch tui.Channel, text string) (question string, ok bool) {
	if !s.channelCheck || s.decision == nil || len(text) < channelCheckMinLen {
		return "", false
	}
	verdict, err := s.decision.Decide(s.ctx, decide.Question{
		State:    channelCheckState(text),
		Question: channelCheckQuestion,
		Options:  []string{"in-character", "out-of-character"},
	})
	if err != nil || verdict.Choice == channelOption(ch) ||
		verdict.Confidence < channelCheckMinConfidence {
		return "", false
	}
	if ch == tui.ChannelIC {
		return channelCheckPromptIC, true
	}
	return channelCheckPromptOOC, true
}

// waitConfirm holds one gated message: it emits the EventConfirm carrying
// the question and the held text, then blocks the runner until the player
// answers with Command("confirm", "y"|"n") or the session closes. The
// inputs queue is untouched meanwhile — new messages keep queueing on the
// buffered channel — and Close discards the hold without hanging: nothing
// is echoed, recorded or sent to the model until the answer says y.
func (s *Session) waitConfirm(ch tui.Channel, question, text string) bool {
	s.held.Store(true)
	defer s.held.Store(false)
	s.emit(tui.Event{Kind: tui.EventConfirm, Channel: ch,
		Markdown: question, ConfirmText: text})
	select {
	case answer := <-s.confirmCh:
		return answer == "y"
	case <-s.quit:
		return false
	}
}

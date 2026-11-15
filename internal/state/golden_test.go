// SPDX-License-Identifier: EUPL-1.2

package state

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mythras-ultra/myul/internal/engine/resolve"
)

// TestRenderTranscriptGoldenRewrite pins the two-phase play rendering: a
// "retract" tombstone hides exactly the chat lines of its MsgSeq (the
// superseded original), the rewritten line and its muted note show — the
// note as its own italic line — golden_rule_check audit entries never
// print (the display shows only the note), while player echoes, moves and
// legacy MsgSeq-0 lines render unchanged. Two renders stay byte-identical.
func TestRenderTranscriptGoldenRewrite(t *testing.T) {
	st := resolve.NewState("The Ash Road", 42)
	st.ID = "s-golden"
	move, err := json.Marshal(resolve.RollDice{Expr: "1d6", Reason: "omen"})
	if err != nil {
		t.Fatalf("marshal move: %v", err)
	}
	res, err := json.Marshal(resolve.Result{Summary: "1d6 → 4"})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	entries := []LogEntry{
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Player: true, Kind: "narrative", Text: "I push the door."}},
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Kind: "narrative", MsgSeq: 1, Text: "You force the door."}},
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Kind: "narrative", MsgSeq: 1, Text: "Dust hangs."}},
		{Kind: "roll_dice", Move: move, Result: res},
		{Kind: GoldenRuleCheckKind, Move: json.RawMessage(
			`{"msg_seq":1,"verdict":"violates","confidence":0.9,"decision":"regen","attempt":1}`)},
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Kind: ChatKindRetract, MsgSeq: 1}},
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Kind: "narrative", MsgSeq: 2, Text: "The door yields."}},
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Kind: ChatKindNote, MsgSeq: 2, Text: "↩ reply rewritten (golden rule)"}},
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Kind: "narrative", Text: "A legacy line."}},
	}

	out, err := RenderTranscript(st, entries)
	if err != nil {
		t.Fatalf("RenderTranscript: %v", err)
	}
	for _, want := range []string{
		"**You:** I push the door.\n",
		"- **roll_dice** — 1d6 → 4\n",
		"The door yields.\n",
		"*↩ reply rewritten (golden rule)*\n",
		"A legacy line.\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("transcript missing %q:\n%s", want, out)
		}
	}
	for _, gone := range []string{"You force the door.", "Dust hangs."} {
		if strings.Contains(out, gone) {
			t.Errorf("transcript still shows the retracted original %q:\n%s", gone, out)
		}
	}
	if strings.Contains(out, "golden_rule_check") {
		t.Errorf("the golden-rule audit must not render:\n%s", out)
	}
	// The retract tombstone itself is machinery, not play.
	if strings.Contains(out, ChatKindRetract) {
		t.Errorf("the tombstone line must not render:\n%s", out)
	}
	// Later lines keep their order: roll before the rewrite, note after.
	if strings.Index(out, "- **roll_dice**") > strings.Index(out, "The door yields.") {
		t.Errorf("the roll should render before the rewrite:\n%s", out)
	}

	again, err := RenderTranscript(st, entries)
	if err != nil {
		t.Fatalf("second RenderTranscript: %v", err)
	}
	if out != again {
		t.Errorf("renders differ:\nfirst:\n%s\nsecond:\n%s", out, again)
	}
}

// TestRenderTranscriptLegacyLogUnaffected: chat lines without the message
// sequence (every log written before the golden-rule check) render exactly
// as before — a stray MsgSeq-0 tombstone retracts nothing and shows nothing.
func TestRenderTranscriptLegacyLogUnaffected(t *testing.T) {
	st := resolve.NewState("Legacy", 7)
	entries := []LogEntry{
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Player: true, Kind: "narrative", Text: "I knock twice."}},
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Kind: ChatKindRetract, MsgSeq: 0}},
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Kind: "narrative", Text: "The door swings open."}},
	}
	out, err := RenderTranscript(st, entries)
	if err != nil {
		t.Fatalf("RenderTranscript: %v", err)
	}
	for _, want := range []string{"**You:** I knock twice.\n", "The door swings open.\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("transcript missing %q (a MsgSeq-0 tombstone must hide nothing):\n%s", want, out)
		}
	}
}

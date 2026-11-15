// SPDX-License-Identifier: EUPL-1.2

package tui

// Offline tests for the golden-rule surface: the "Y/n" regeneration
// confirm (the inverse default of the channel check's "N/y" hold — enter
// regenerates there, enter discards here — both coexisting in one app)
// and the MsgSeq replacement that swaps a rewritten reply into the
// transcript. Everything runs against the fake driver: no terminal, no
// harness, no network.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// regenQuestion is the exact prompt the harness files on EventConfirmRegen.
const regenQuestion = "The Game Master appears to have decided your character's actions for you. This is a violation of a golden rule in roleplaying. Regenerate Game Master's response? Y/n"

// askRegen feeds one EventConfirmRegen to the app, as the harness emits it
// (no draft text — the reply on screen is what n keeps).
func askRegen(t *testing.T, a *App) {
	t.Helper()
	_, _ = a.Update(eventMsg{ev: Event{
		Kind: EventConfirmRegen, Channel: ChannelIC, Markdown: regenQuestion,
	}})
}

// pressAnswer feeds a key to the app and runs the command it produced off
// the update loop, exactly as the app would (a confirm key must always
// produce one).
func pressAnswer(t *testing.T, a *App, k tea.KeyPressMsg) {
	t.Helper()
	cmd := press(t, a, k)
	if cmd == nil {
		t.Fatalf("%s: the answer key should produce a command", k.String())
	}
	_ = cmd()
}

// hasConfirmCommand reports whether the driver received exactly one
// Command("confirm", want).
func hasConfirmCommand(d *fakeDriver, want string) bool {
	return len(d.commands) == 1 && d.commands[0].name == "confirm" &&
		len(d.commands[0].args) == 1 && d.commands[0].args[0] == want
}

// TestRegenConfirmMode pins the regeneration confirm: the exact question
// files as one styled, width-clipped line, the prompt reads "Y/n ▸ ",
// enter/y/Y regenerate (Command confirm y), n/N/esc keep (confirm n, with
// nothing to restore), every other key is ignored and ctrl+c still quits.
// The channel check's inverse "N/y" default is asserted in the same test —
// both variants coexist in one app.
func TestRegenConfirmMode(t *testing.T) {
	t.Run("question renders and the prompt switches", func(t *testing.T) {
		// The exact question is longer than a narrow chat column: give
		// it room, so a clipped question cannot pass the exactness check.
		a, _ := newTestApp(t, Options{}, 200, 24)
		askRegen(t, a)
		if !a.confirm || !a.confirmRegen {
			t.Fatalf("confirm=%v regen=%v, want both on", a.confirm, a.confirmRegen)
		}
		if a.input.Prompt != "Y/n ▸ " {
			t.Errorf("prompt = %q, want %q", a.input.Prompt, "Y/n ▸ ")
		}
		if got := plain(view(a)); !strings.Contains(got, regenQuestion) {
			t.Errorf("the question should render, view:\n%s", got)
		}
		last := a.entries[len(a.entries)-1]
		if last.ev.Kind != EventConfirmRegen {
			t.Fatalf("last entry = %+v, want the regeneration question", last.ev)
		}
		if last.ev.ConfirmText != "" {
			t.Errorf("ConfirmText = %q, want empty — the regen prompt carries no draft", last.ev.ConfirmText)
		}
		if strings.Contains(last.text, "\n") {
			t.Errorf("the question must render as one line, got %q", last.text)
		}
		if last.text == plain(last.text) {
			t.Error("the question should be styled, not plain text")
		}
		if w := lipgloss.Width(last.text); w > a.chatW {
			t.Errorf("question rendered %d columns, want at most %d", w, a.chatW)
		}
	})

	t.Run("enter, y and Y regenerate", func(t *testing.T) {
		for _, k := range []tea.KeyPressMsg{
			{Code: tea.KeyEnter},
			key('y'),
			key('Y'),
		} {
			a, d := newTestApp(t, Options{}, 80, 24)
			askRegen(t, a)
			pressAnswer(t, a, k)
			if !hasConfirmCommand(d, "y") {
				t.Fatalf("%s: commands = %+v, want confirm y", k.String(), d.commands)
			}
			if a.confirm || a.input.Prompt != "> " {
				t.Errorf("%s: confirm=%v prompt=%q, want the normal line", k.String(), a.confirm, a.input.Prompt)
			}
			if a.input.Value() != "" {
				t.Errorf("%s: input = %q, want it untouched", k.String(), a.input.Value())
			}
		}
	})

	t.Run("n, N and esc keep the reply without restoring", func(t *testing.T) {
		for _, k := range []tea.KeyPressMsg{
			key('n'),
			key('N'),
			{Code: tea.KeyEsc},
		} {
			a, d := newTestApp(t, Options{}, 80, 24)
			askRegen(t, a)
			pressAnswer(t, a, k)
			if !hasConfirmCommand(d, "n") {
				t.Fatalf("%s: commands = %+v, want confirm n", k.String(), d.commands)
			}
			if a.confirm || a.input.Prompt != "> " {
				t.Errorf("%s: confirm=%v prompt=%q, want the normal line", k.String(), a.confirm, a.input.Prompt)
			}
			if a.input.Value() != "" {
				t.Errorf("%s: input = %q, want nothing restored — the regen holds no draft", k.String(), a.input.Value())
			}
		}
	})

	t.Run("other keys are ignored while the answer is pending", func(t *testing.T) {
		a, d := newTestApp(t, Options{}, 80, 24)
		askRegen(t, a)
		for _, k := range []tea.KeyPressMsg{
			key('x'), key('q'),
			tea.KeyPressMsg{Code: tea.KeyUp}, tea.KeyPressMsg{Code: tea.KeyDown},
			ctrl('t'), ctrl('p'), ctrl('m'),
		} {
			press(t, a, k)
		}
		if len(d.commands) != 0 {
			t.Errorf("ignored keys reached the harness: commands=%+v", d.commands)
		}
		if !a.confirm {
			t.Fatal("the regen confirm should still be pending")
		}
		if a.channel != ChannelIC {
			t.Errorf("channel = %v, want ctrl+t ignored during a confirm", a.channel)
		}
		if a.pal.open || a.helpOpen {
			t.Error("overlays must stay closed during a confirm")
		}
		if a.input.Value() != "" {
			t.Errorf("input = %q, want ignored keys kept out of the line", a.input.Value())
		}
	})

	t.Run("ctrl+c still quits", func(t *testing.T) {
		a, _ := newTestApp(t, Options{}, 80, 24)
		askRegen(t, a)
		cmd := press(t, a, ctrl('c'))
		if cmd == nil {
			t.Fatal("ctrl+c should produce a quit command")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Error("ctrl+c from confirm mode should quit")
		}
	})

	t.Run("both confirm variants coexist", func(t *testing.T) {
		const held = "Could we pause here — I need to check my sheet before deciding."
		a, d := newTestApp(t, Options{}, 80, 24)

		// Golden-rule regeneration: Enter = y.
		askRegen(t, a)
		pressAnswer(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
		if !hasConfirmCommand(d, "y") {
			t.Fatalf("regen enter: commands = %+v, want a lone confirm y", d.commands)
		}
		// Channel check afterwards: Enter = n, held text restored.
		_, _ = a.Update(eventMsg{ev: Event{
			Kind: EventConfirm, Channel: ChannelIC,
			Markdown:    "You are in IC mode, but your response looks like a message for OOC. Is this correct? N/y",
			ConfirmText: held,
		}})
		if a.confirmRegen || a.input.Prompt != "N/y ▸ " {
			t.Fatalf("regen=%v prompt=%q, want the channel check's N/y line", a.confirmRegen, a.input.Prompt)
		}
		pressAnswer(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
		if len(d.commands) != 2 || d.commands[1].name != "confirm" ||
			len(d.commands[1].args) != 1 || d.commands[1].args[0] != "n" {
			t.Fatalf("channel enter: commands = %+v, want the second to be confirm n", d.commands)
		}
		if a.input.Value() != held {
			t.Errorf("input = %q, want the held message returned unsent", a.input.Value())
		}
	})
}

// TestReplaceNarrativeSwapsMessage pins the MsgSeq replacement: a
// Replace event removes every narrative entry of the replaced sequence,
// files the full rewritten text at the first one's position (the
// interleaved tool strip keeps its place after it), the run tags are
// recomputed so the swap opens its own tagged run, and the note follows —
// while a later system line and the player's echo stay put.
func TestReplaceNarrativeSwapsMessage(t *testing.T) {
	const (
		original1 = "You force the door and step inside."
		original2 = "Dust hangs from the rafters."
		rewrite   = "The door yields to your push."
		note      = "↩ reply rewritten (golden rule)"
	)
	a, _ := newTestApp(t, Options{}, 80, 24)
	ingest := func(ev Event) { _, _ = a.Update(eventMsg{ev: ev}) }

	ingest(Event{Kind: EventNarrative, Channel: ChannelIC,
		Markdown: "I push the door.", Player: true})
	// The original reply, two paragraph entries under one MsgSeq.
	ingest(Event{Kind: EventNarrative, Channel: ChannelIC, Markdown: original1, MsgSeq: 1})
	ingest(Event{Kind: EventNarrative, Channel: ChannelIC, Markdown: original2, MsgSeq: 1})
	// A tool strip interleaved with the reply: metadata, never replaced.
	ingest(Event{Kind: EventToolAnnotation, Channel: ChannelIC,
		Annotation: &Annotation{ToolName: "roll_dice", Summary: "1d6 → 4", OK: true}})

	if len(a.entries) != 4 {
		t.Fatalf("entries before the swap = %d, want 4", len(a.entries))
	}
	if !a.follow {
		t.Fatal("follow mode should be on before the swap")
	}

	// The accepted rewrite swaps in, then the note lands behind it.
	ingest(Event{Kind: EventNarrative, Channel: ChannelIC,
		Markdown: rewrite, Replace: true, ReplaceSeq: 1, MsgSeq: 2})
	ingest(Event{Kind: EventSystem, Channel: ChannelIC, Markdown: note, MsgSeq: 2})

	if len(a.entries) != 4 {
		t.Fatalf("entries after the swap = %d, want 4 (echo, rewrite, strip, note): %+v",
			len(a.entries), a.entries)
	}
	if ev := a.entries[1].ev; ev.Markdown != rewrite || ev.MsgSeq != 2 || !ev.Replace {
		t.Errorf("slot 1 = %+v, want the full rewrite filed where the original opened", ev)
	}
	if a.entries[2].ev.Kind != EventToolAnnotation {
		t.Errorf("slot 2 = %+v, want the interleaved tool strip kept", a.entries[2].ev)
	}
	if a.entries[3].ev.Markdown != note || a.entries[3].ev.Kind != EventSystem {
		t.Errorf("slot 3 = %+v, want the note as a system line", a.entries[3].ev)
	}
	// Tags recomputed: the rewrite opens its own GM run after the player
	// line, and the strip stays untagged metadata.
	if !a.entries[0].tagged || !a.entries[1].tagged || a.entries[2].tagged {
		t.Errorf("tags = [%v %v %v], want the echo and rewrite tagged, the strip not",
			a.entries[0].tagged, a.entries[1].tagged, a.entries[2].tagged)
	}
	got := plain(view(a))
	if !strings.Contains(got, rewrite) {
		t.Errorf("view should show the rewrite:\n%s", got)
	}
	for _, gone := range []string{original1, original2} {
		if strings.Contains(got, gone) {
			t.Errorf("view still shows the replaced text %q:\n%s", gone, got)
		}
	}
	if !strings.Contains(got, note) {
		t.Errorf("view should show the note:\n%s", got)
	}
	if !a.follow {
		t.Error("follow mode must survive the swap")
	}
}

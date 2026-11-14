package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mythras-ultra/myul/internal/engine/char"
	"github.com/mythras-ultra/myul/internal/engine/resolve"
)

// transcriptSession builds a seeded mini-session the way the session loop
// does (session-loop.md): a refreshed PC, a few moves applied through
// resolve.Apply with each accepted move recorded as a LogEntry built from
// its Result, plus one refused move recorded for audit.
func transcriptSession(t *testing.T) (*resolve.State, []LogEntry) {
	t.Helper()
	st := resolve.NewState("The Ash Road", 42)
	st.ID = "s-transcript"
	st.CreatedAt = time.Date(2026, time.November, 14, 12, 0, 0, 0, time.UTC)
	st.UpdatedAt = time.Date(2026, time.November, 14, 13, 30, 0, 0, time.UTC)
	c := &char.Character{
		ID: "pc-1", Name: "Ynara", PC: true,
		Chars: char.Characteristics{STR: 14, CON: 13, SIZ: 12, DEX: 15, INT: 12, POW: 14, CHA: 10},
	}
	if err := c.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if err := c.SetCombatStyle("Broadsword Art", 70); err != nil {
		t.Fatalf("SetCombatStyle: %v", err)
	}
	st.Characters[c.ID] = c

	moves := []resolve.Move{
		resolve.RollDice{Expr: "1d6", Reason: "omen"},
		resolve.SkillTest{Actor: "pc-1", Skill: "Stealth"},
		resolve.SetScene{Location: "The Broken Causeway", Description: "Rain.", Present: []string{"pc-1"}},
		resolve.EndRecap{Summary: "Session one: the ferryman lied."},
	}
	var entries []LogEntry
	for _, m := range moves {
		res, err := resolve.Apply(st, m)
		if err != nil {
			t.Fatalf("%s: %v", m.Kind(), err)
		}
		moveRaw, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("marshal move %s: %v", m.Kind(), err)
		}
		resRaw, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal result %s: %v", m.Kind(), err)
		}
		entries = append(entries, LogEntry{
			Seq: st.EventSeq, Kind: m.Kind(), Move: moveRaw, Result: resRaw,
		})
	}
	// A refused move: applied nothing, recorded for audit only.
	entries = append(entries, LogEntry{
		Seq:   st.EventSeq,
		Kind:  "skill_test",
		Move:  json.RawMessage(`{"actor":"pc-9","skill":"Stealth"}`),
		Error: `resolve: unknown character "pc-9" (known: pc-1)`,
	})
	return st, entries
}

func TestRenderTranscript(t *testing.T) {
	st, entries := transcriptSession(t)

	out, err := RenderTranscript(st, entries)
	if err != nil {
		t.Fatalf("RenderTranscript: %v", err)
	}
	for _, want := range []string{
		"# The Ash Road — Play Log\n",
		"- Session: s-transcript\n",
		"- Seed: 42\n",
		"- Created: 2026-11-14T12:00:00Z\n",
		"- Updated: 2026-11-14T13:30:00Z\n",
		"## Play Log\n",
		"- **roll_dice** — ",
		"- **skill_test** — Stealth test: ",
		"  - outcome: ",
		"- **set_scene** — ",
		"- **end_session_recap** — recap 1 stored",
		"- **skill_test** — refused: resolve: unknown character \"pc-9\" (known: pc-1)\n",
		"## Session Recap\n",
		"1. Session one: the ferryman lied.\n",
		"## Final Characters\n",
		"- **Ynara** (pc-1, PC) — HP ",
		"— conditions: none\n",
		"  - combat styles: Broadsword Art 70%\n",
		"  - key skills: ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("transcript missing %q:\n%s", want, out)
		}
	}

	// The dice line carries the exact numbers from the Result recorded in
	// the log: the skill_test entry's 1d100 appears with its audit values.
	var skillRes resolve.Result
	if err := json.Unmarshal(entries[1].Result, &skillRes); err != nil {
		t.Fatalf("unmarshal recorded skill_test result: %v", err)
	}
	if skillRes.Audit == nil || len(skillRes.Audit.Rolls) != 1 {
		t.Fatalf("recorded skill_test audit = %+v, want one roll", skillRes.Audit)
	}
	roll := skillRes.Audit.Rolls[0]
	want := "  - roll: 1d100 → [" + strconv.Itoa(roll.Dice[0].Value) + "] = " + strconv.Itoa(roll.Total) + "\n"
	if !strings.Contains(out, want) {
		t.Errorf("transcript missing roll line %q:\n%s", want, out)
	}
	if n := strings.Count(out, "roll: 1d6 → "); n != 1 {
		t.Errorf("roll_dice roll rendered %d times, want 1 (fact and audit must dedup):\n%s", n, out)
	}

	// Determinism: two renders are byte-identical (an unsorted map range
	// would surface here as a shuffled character/skill block).
	again, err := RenderTranscript(st, entries)
	if err != nil {
		t.Fatalf("second RenderTranscript: %v", err)
	}
	if out != again {
		t.Errorf("renders differ:\nfirst:\n%s\nsecond:\n%s", out, again)
	}
}

// TestRenderTranscriptChatInterleaved pins the play rendering: chat lines
// and move annotations form one ordered stream — the player's line
// prefixed "**You:** ", the GM's narration and recorded error text
// verbatim — and two renders of the same log stay byte-identical.
func TestRenderTranscriptChatInterleaved(t *testing.T) {
	st := resolve.NewState("The Ash Road", 42)
	st.ID = "s-play"
	result, err := json.Marshal(resolve.Result{Summary: "1d6 → 4"})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	entries := []LogEntry{
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Player: true, Kind: "narrative", Text: "I knock twice."}},
		{Kind: "chat", Chat: &ChatLine{Channel: "IC", Kind: "narrative", Text: "The door swings open.\n"}},
		{Kind: "roll_dice", Move: json.RawMessage(`{"expr":"1d6"}`), Result: result},
		{Kind: "chat", Chat: &ChatLine{Channel: "OOC", Kind: "error", Text: "turn failed: the model is unavailable"}},
	}

	out, err := RenderTranscript(st, entries)
	if err != nil {
		t.Fatalf("RenderTranscript: %v", err)
	}
	for _, want := range []string{
		"**You:** I knock twice.\n",
		"The door swings open.\n",
		"- **roll_dice** — 1d6 → 4\n",
		"turn failed: the model is unavailable\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("transcript missing %q:\n%s", want, out)
		}
	}
	// Interleaving: chat and move lines share the recorded order.
	wantOrder := []string{"**You:** I knock twice.", "The door swings open.",
		"- **roll_dice**", "turn failed: the model is unavailable"}
	prev := -1
	for _, needle := range wantOrder {
		i := strings.Index(out, needle)
		if i < 0 {
			t.Fatalf("transcript missing %q:\n%s", needle, out)
		}
		if i <= prev {
			t.Errorf("%q appears out of recorded order:\n%s", needle, out)
		}
		prev = i
	}
	// The stream-flush newline is trimmed, so no blank line opens before
	// the move annotation, and chat never renders as a move line.
	if strings.Contains(out, "open.\n\n- **roll_dice**") {
		t.Errorf("chat line left a stray blank line:\n%s", out)
	}
	if strings.Contains(out, "- **chat**") {
		t.Errorf("chat entry rendered as a move:\n%s", out)
	}

	again, err := RenderTranscript(st, entries)
	if err != nil {
		t.Fatalf("second RenderTranscript: %v", err)
	}
	if out != again {
		t.Errorf("renders differ:\nfirst:\n%s\nsecond:\n%s", out, again)
	}
}

func TestRenderTranscriptEmptyLog(t *testing.T) {
	st := resolve.NewState("Fresh Start", 7)
	out, err := RenderTranscript(st, nil)
	if err != nil {
		t.Fatalf("RenderTranscript(nil entries): %v", err)
	}
	for _, want := range []string{"(no moves recorded)\n", "(no recaps recorded)\n", "(no characters in play)\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("transcript missing %q:\n%s", want, out)
		}
	}
}

func TestRenderTranscriptRejectsBadInput(t *testing.T) {
	if _, err := RenderTranscript(nil, nil); !errors.Is(err, ErrInvalidState) {
		t.Errorf("nil state error = %v, want ErrInvalidState", err)
	}
	corrupt := LogEntry{Seq: 7, Kind: "skill_test", Result: json.RawMessage(`42`)}
	_, err := RenderTranscript(resolve.NewState("x", 1), []LogEntry{corrupt})
	if err == nil {
		t.Fatal("corrupt result payload: want error, got nil")
	}
	if !strings.Contains(err.Error(), "seq 7") || !strings.Contains(err.Error(), "skill_test") {
		t.Errorf("corrupt result error = %v, want seq and kind context", err)
	}
}

// TestTranscriptSkipsTruncatedLogTail pins the export pipeline contract: a
// log.jsonl whose last line was cut off mid-append (crash signature) reads
// through ReadLog into entries that RenderTranscript renders fully.
func TestTranscriptSkipsTruncatedLogTail(t *testing.T) {
	store, _ := testStore(t)
	st := makeState(t, "s-trunc")
	if err := store.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}
	move, err := json.Marshal(resolve.RollDice{Expr: "1d6"})
	if err != nil {
		t.Fatalf("marshal move: %v", err)
	}
	result, err := json.Marshal(resolve.Result{Summary: "1d6 → 4", Facts: []string{"roll: 1d6 → [4] = 4"}})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := store.Record("s-trunc", LogEntry{Seq: 1, Kind: "roll_dice", Move: move, Result: result}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	dir, err := store.sessionDir("s-trunc")
	if err != nil {
		t.Fatalf("sessionDir: %v", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "log.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	if _, err := f.WriteString(`{"seq":2,"kind":"ski`); err != nil {
		_ = f.Close()
		t.Fatalf("append truncated line: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close log: %v", err)
	}

	entries, err := store.ReadLog("s-trunc")
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ReadLog = %d entries, want 1 (truncated tail skipped)", len(entries))
	}
	out, err := RenderTranscript(st, entries)
	if err != nil {
		t.Fatalf("RenderTranscript: %v", err)
	}
	if !strings.Contains(out, "- **roll_dice** — 1d6 → 4") {
		t.Errorf("transcript missing recorded roll:\n%s", out)
	}
}

func TestWriteTranscript(t *testing.T) {
	st, entries := transcriptSession(t)
	want, err := RenderTranscript(st, entries)
	if err != nil {
		t.Fatalf("RenderTranscript: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteTranscript(&buf, st, entries); err != nil {
		t.Fatalf("WriteTranscript: %v", err)
	}
	if buf.String() != want {
		t.Errorf("WriteTranscript output differs from RenderTranscript")
	}
	if err := WriteTranscript(failingWriter{}, st, entries); err == nil {
		t.Error("WriteTranscript to failing writer: want error, got nil")
	}
}

// failingWriter always fails, so the write-error path is exercised without
// a filesystem.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("no space") }

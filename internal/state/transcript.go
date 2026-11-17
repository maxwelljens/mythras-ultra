package state

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/maxwelljens/mythras-ultra/internal/engine/char"
	"github.com/maxwelljens/mythras-ultra/internal/engine/resolve"
)

// transcriptKeySkills is how many of a character's highest-rated skills the
// final summary lists ("key skills", not the whole sheet).
const transcriptKeySkills = 5

// RenderTranscript renders a session's play log as deterministic Markdown:
// a header of session identity, the play itself in recorded order — chat
// lines as the play text (the player's lines prefixed "**You:** ", the
// GM's narration and error lines verbatim) interleaved with one readable
// line per move entry (an accepted move as "**kind** — summary" with its
// Facts, dice rolls and rule citations underneath; a refused move as
// "**kind** — refused: ...") — then the stored recaps and a compact final
// character summary (ARCHITECTURE.md §5, transcript.md).
//
// The output is a pure function of s and entries: entries keep the order
// they were given (the order [Store.ReadLog] replayed them, which already
// skips a crash-truncated final line), maps are only iterated in sorted
// order, and no wall time is read — the only timestamps are the session's
// own CreatedAt and UpdatedAt in the header. Two renders of the same state
// and log are byte-identical, so transcript.md regenerates from log.jsonl.
// Rewrites are two-phase: lines hidden by a "retract" tombstone and the
// golden-rule audit entries (Kind [GoldenRuleCheckKind]) never print — the
// play shows the rewritten reply and its note.
//
// A nil state is rejected wrapping [ErrInvalidState]; an entry whose Result
// payload is not a resolve.Result is rejected with its sequence number — a
// corrupt log must surface, not render as something false.
func RenderTranscript(s *resolve.State, entries []LogEntry) (string, error) {
	if s == nil {
		return "", fmt.Errorf("%w: nil state for transcript", ErrInvalidState)
	}
	var b strings.Builder
	title := strings.TrimSpace(s.Name)
	if title == "" {
		title = "Untitled Session"
	}
	fmt.Fprintf(&b, "# %s — Play Log\n\n", oneLine(title))
	fmt.Fprintf(&b, "- Session: %s\n", s.ID)
	fmt.Fprintf(&b, "- Seed: %d\n", s.Seed)
	fmt.Fprintf(&b, "- Created: %s\n", formatStamp(s.CreatedAt))
	fmt.Fprintf(&b, "- Updated: %s\n", formatStamp(s.UpdatedAt))

	b.WriteString("\n## Play Log\n\n")
	if len(entries) == 0 {
		b.WriteString("(no moves recorded)\n")
	}
	// Two-phase render: the retracted message sequence numbers are known
	// before any line prints, so a "retract" tombstone hides exactly the
	// lines of the message it superseded — later lines (a rewrite, a
	// note) always show, and logs without tombstones render as before.
	retracted := retractedMsgSeqs(entries)
	for i, e := range entries {
		lines, err := renderEntry(e, retracted)
		if err != nil {
			return "", fmt.Errorf("state: transcript entry %d: %w", i+1, err)
		}
		for _, ln := range lines {
			b.WriteString(ln)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n## Session Recap\n\n")
	if len(s.Recaps) == 0 {
		b.WriteString("(no recaps recorded)\n")
	}
	for i, r := range s.Recaps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, indentContinuation(r))
	}

	b.WriteString("\n## Final Characters\n\n")
	renderCharacters(&b, s)
	return b.String(), nil
}

// WriteTranscript renders the play log with [RenderTranscript] and writes it
// to w — the form the export command uses so stdout and --out files share
// one code path. Write failures wrap with %w.
func WriteTranscript(w io.Writer, s *resolve.State, entries []LogEntry) error {
	doc, err := RenderTranscript(s, entries)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, doc); err != nil {
		return fmt.Errorf("state: write transcript: %w", err)
	}
	return nil
}

// retractedMsgSeqs collects the MsgSeqs a "retract" tombstone hides: the
// set of assistant messages whose recorded chat lines a rewrite superseded.
// A tombstone with MsgSeq 0 (legacy — never written) retracts nothing, so
// untracked logs stay fully visible.
func retractedMsgSeqs(entries []LogEntry) map[int]bool {
	var out map[int]bool
	for _, e := range entries {
		if e.Chat == nil || e.Chat.Kind != ChatKindRetract || e.Chat.MsgSeq == 0 {
			continue
		}
		if out == nil {
			out = make(map[int]bool)
		}
		out[e.Chat.MsgSeq] = true
	}
	return out
}

// renderEntry renders one log entry as its markdown lines: a chat line as
// its play text (retracted lines and tombstones render nothing), the
// golden-rule audit as nothing (raw log only — the play shows the rewrite's
// note), otherwise the "kind — summary" head, then indented Facts and
// Audit rolls/citations. Lines that would repeat verbatim within one
// entry are emitted once (several moves record the same roll as both a
// Fact and an Audit roll).
func renderEntry(e LogEntry, retracted map[int]bool) ([]string, error) {
	if e.Chat != nil {
		return renderChat(e.Chat, retracted), nil
	}
	if e.Kind == GoldenRuleCheckKind {
		return nil, nil
	}
	kind := e.Kind
	if kind == "" {
		kind = "move"
	}
	if e.Error != "" {
		return []string{fmt.Sprintf("- **%s** — refused: %s", kind, oneLine(e.Error))}, nil
	}
	if len(e.Result) == 0 {
		return []string{fmt.Sprintf("- **%s**", kind)}, nil
	}
	var res resolve.Result
	if err := json.Unmarshal(e.Result, &res); err != nil {
		return nil, fmt.Errorf("decode result of %q (seq %d): %w", kind, e.Seq, err)
	}
	head := "- **" + kind + "**"
	if summary := oneLine(res.Summary); summary != "" {
		head += " — " + summary
	}
	lines := []string{head}
	seen := map[string]bool{head: true}
	add := func(line string) {
		if !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
	}
	for _, f := range res.Facts {
		add("  - " + oneLine(f))
	}
	if res.Audit != nil {
		for _, r := range res.Audit.Rolls {
			add("  - roll: " + r.String())
		}
		for _, c := range res.Audit.Citations {
			add("  - rule: " + oneLine(c))
		}
	}
	return lines, nil
}

// renderChat renders one chat entry as its play line: the player's line
// prefixed "**You:** ", the GM's narration (and recorded error text)
// verbatim, a note as its muted italic line, in log order — so the play
// reads between the move annotations. Lines a "retract" tombstone hides
// (and the tombstones themselves) render nothing, so a rewritten reply
// shows only its final version while legacy lines (MsgSeq 0) are
// unaffected. Trailing line breaks from stream flushes are trimmed (the
// entry loop adds exactly one), keeping renders byte-identical line for line.
func renderChat(c *ChatLine, retracted map[int]bool) []string {
	if c.Kind == ChatKindRetract {
		return nil
	}
	if c.MsgSeq != 0 && retracted[c.MsgSeq] {
		return nil // superseded by a rewrite of that message
	}
	text := strings.TrimRight(c.Text, "\r\n")
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if c.Player {
		return []string{"**You:** " + text}
	}
	if c.Kind == ChatKindNote {
		return []string{"*" + oneLine(text) + "*"}
	}
	return []string{text}
}

// renderCharacters writes the compact final character summary to b: one
// bullet per character (ListCharacters is sorted by ID), with live HP,
// conditions, combat styles and the top-rated skills as sub-bullets.
func renderCharacters(b *strings.Builder, s *resolve.State) {
	chars := resolve.ListCharacters(s)
	if len(chars) == 0 {
		b.WriteString("(no characters in play)\n")
		return
	}
	for _, c := range chars {
		name := oneLine(c.Name)
		if name == "" {
			name = c.ID
		}
		line := "- **" + name + "** (" + c.ID
		if c.PC {
			line += ", PC"
		}
		if c.Dead {
			line += ", dead"
		}
		line += ")"
		if cur, max := hpTotals(c); max > 0 {
			line += fmt.Sprintf(" — HP %d/%d", cur, max)
		}
		line += " — conditions: " + conditionsLabel(c)
		b.WriteString(line + "\n")
		if len(c.CombatStyles) > 0 {
			b.WriteString("  - combat styles: " + styleLabels(c) + "\n")
		}
		if skills := keySkills(c, transcriptKeySkills); len(skills) > 0 {
			b.WriteString("  - key skills: " + strings.Join(skills, ", ") + "\n")
		}
	}
}

// hpTotals sums the character's live hit points across hit locations and
// returns the starting total they are compared against (0 when the
// character carries no HP information, so the caller omits the segment).
func hpTotals(c *char.Character) (cur, max int) {
	for _, v := range c.CurrentHP {
		cur += v
	}
	return cur, c.Derived.HitPoints
}

// conditionsLabel renders the active condition names, or "none".
func conditionsLabel(c *char.Character) string {
	if len(c.Conditions) == 0 {
		return "none"
	}
	names := make([]string, 0, len(c.Conditions))
	for _, cond := range c.Conditions {
		names = append(names, oneLine(cond.Name))
	}
	return strings.Join(names, ", ")
}

// styleLabels renders every combat style as "Name 70%", sorted by name so
// the line never depends on map iteration.
func styleLabels(c *char.Character) string {
	names := make([]string, 0, len(c.CombatStyles))
	for name := range c.CombatStyles {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		style := c.CombatStyles[name]
		if style == nil {
			continue
		}
		parts = append(parts, oneLine(name)+" "+strconv.Itoa(style.Value)+"%")
	}
	return strings.Join(parts, ", ")
}

// keySkills renders the n highest-rated skills as "Name 45%", best first
// with ties broken by name — a stable order regardless of map iteration.
func keySkills(c *char.Character, n int) []string {
	type rated struct {
		name  string
		value int
	}
	list := make([]rated, 0, len(c.Skills))
	for name, sk := range c.Skills {
		if sk == nil {
			continue
		}
		list = append(list, rated{name: name, value: sk.Value})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].value != list[j].value {
			return list[i].value > list[j].value
		}
		return list[i].name < list[j].name
	})
	if len(list) > n {
		list = list[:n]
	}
	out := make([]string, 0, len(list))
	for _, r := range list {
		out = append(out, oneLine(r.name)+" "+strconv.Itoa(r.value)+"%")
	}
	return out
}

// formatStamp renders a session timestamp for the header in UTC RFC3339;
// the persistence layer owns it, so a state that has never been saved shows
// "unknown" rather than year one.
func formatStamp(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.UTC().Format(time.RFC3339)
}

// oneLine collapses a value's whitespace into single spaces so summaries,
// facts and free text always render as one markdown line, whatever the GM
// or a tool put in them.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// indentContinuation keeps a recap's own line breaks inside its list item
// by indenting continuation lines to the "N. " marker width.
func indentContinuation(s string) string {
	return strings.ReplaceAll(s, "\n", "\n   ")
}

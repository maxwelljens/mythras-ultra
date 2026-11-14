package tui

import (
	"strings"
	"testing"
	"time"
)

// TestPlayerTextIsDimmed pins the dimming rule: player-submitted narrative
// carries the SGR faint attribute (re-asserted after interior resets, so a
// styled fragment inside the block cannot switch it off), while GM narration
// stays at full strength. Both blocks end with a clean reset so the next
// transcript entry is unaffected.
func TestPlayerTextIsDimmed(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 70, 30)
	ev := Event{
		Kind:     EventNarrative,
		Channel:  ChannelIC,
		Markdown: "The **door** opens.",
		At:       time.Now(),
	}
	player := a.renderEvent(ev, 70, true, true)
	gm := a.renderEvent(ev, 70, false, true)

	if !strings.Contains(player, "\x1b[2m") {
		t.Errorf("player text should start faint: %q", player)
	}
	// "door" is bold in markdown: glamour resets the SGR state after it —
	// faint must be re-asserted right behind the reset.
	if !strings.Contains(player, "\x1b[0m\x1b[2m") && !strings.Contains(player, "\x1b[m\x1b[2m") {
		t.Errorf("faint must survive interior resets: %q", player)
	}
	if !strings.HasSuffix(player, "\x1b[0m") {
		t.Errorf("dim block must close with a clean reset: %q", player)
	}
	if strings.Contains(gm, "\x1b[2m") {
		t.Errorf("GM narration must not be dimmed: %q", gm)
	}
}

// TestReplayedPlayerTaggedWithoutSendHistory is the regression test for
// losing authorship on resume: an explicit Player flag must yield the "You"
// tag even though nothing was sent in this process, and consecutive player
// lines in the same run are tagged once at the run start.
func TestReplayedPlayerTaggedWithoutSendHistory(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 70, 30)
	for _, text := range []string{"I light the candle.", "And wait."} {
		_, _ = a.Update(eventMsg{ev: Event{
			Kind:     EventNarrative,
			Channel:  ChannelIC,
			Markdown: text,
			Player:   true,
			At:       time.Now(),
		}})
	}
	got := plain(view(a))
	if strings.Count(got, "You") != 1 {
		t.Errorf("player run must be tagged exactly once, got %d tags:\n%s", strings.Count(got, "You"), got)
	}
	if !strings.Contains(got, "I light the candle.") || !strings.Contains(got, "And wait.") {
		t.Errorf("both player lines should render:\n%s", got)
	}
}

// TestPlayerDimKeepsSpacing guards the interaction between faintBlock's edge
// trimming and the viewport join: a dim player entry followed by a GM entry
// still separates by exactly one blank line (the wrapper's trailing reset
// must not hide the trimmed newlines).
func TestPlayerDimKeepsSpacing(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 70, 30)
	_, _ = a.Update(eventMsg{ev: Event{
		Kind: EventNarrative, Channel: ChannelIC,
		Markdown: "I knock twice.", Player: true, At: time.Now(),
	}})
	_, _ = a.Update(eventMsg{ev: Event{
		Kind: EventNarrative, Channel: ChannelIC,
		Markdown: "The door opens.", At: time.Now(),
	}})
	lines := strings.Split(plain(view(a)), "\n")
	knock, door := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "I knock twice.") {
			knock = i
		}
		if strings.Contains(l, "The door opens.") {
			door = i
		}
	}
	if knock < 0 || door < 0 {
		t.Fatalf("both lines should render (knock=%d door=%d):\n%s", knock, door, plain(view(a)))
	}
	// The player entry wears the "You" tag and the GM entry the "GM" tag
	// (each is a run start), so count the blank separator lines between
	// them rather than the raw line distance.
	blank := 0
	for _, l := range lines[knock+1 : door] {
		if strings.TrimSpace(l) == "" {
			blank++
		}
	}
	if blank != 1 {
		t.Errorf("exactly one blank line between player and GM entry, got %d", blank)
	}
}

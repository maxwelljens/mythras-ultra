package tui

import (
	"strings"
	"testing"
	"time"
)

// TestBlankBatchKeepsSingleSpacing reproduces the double-blank-line bug: the
// stream used to flush a whitespace-only batch for each paragraph break, and
// the viewport's "\n\n" join rendered that as TWO blank lines between
// paragraphs instead of one. Whitespace-only entries must not contribute a
// part at all.
func TestBlankBatchKeepsSingleSpacing(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 70, 30)
	for _, md := range []string{"One.", "\n", "Two."} {
		_, _ = a.Update(eventMsg{ev: Event{
			Kind:     EventNarrative,
			Channel:  ChannelIC,
			Markdown: md,
			At:       time.Now(),
		}})
	}
	lines := strings.Split(plain(view(a)), "\n")
	find := func(want string) int {
		for i, l := range lines {
			if strings.Contains(l, want) {
				return i
			}
		}
		return -1
	}
	one, two := find("One."), find("Two.")
	if one < 0 || two < 0 {
		t.Fatalf("both paragraphs should render (one=%d two=%d), view:\n%s", one, two, plain(view(a)))
	}
	if two-one != 2 {
		t.Errorf("exactly one blank line expected between paragraphs, got %d lines between them", two-one-1)
	}
}

// TestParagraphEntriesRenderSpacing asserts the common case: two real
// narrative entries are separated by exactly one blank line (indent- and
// ANSI-agnostic — glamour renders blocks with a 2-space indent).
func TestParagraphEntriesRenderSpacing(t *testing.T) {
	a, _ := newTestApp(t, Options{}, 70, 30)
	for _, md := range []string{"First paragraph.", "Second paragraph."} {
		_, _ = a.Update(eventMsg{ev: Event{
			Kind:     EventNarrative,
			Channel:  ChannelIC,
			Markdown: md,
			At:       time.Now(),
		}})
	}
	lines := strings.Split(plain(view(a)), "\n")
	first, second := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "First paragraph.") {
			first = i
		}
		if strings.Contains(l, "Second paragraph.") {
			second = i
		}
	}
	if first < 0 || second < 0 {
		t.Fatalf("both entries should render (first=%d second=%d), view:\n%s", first, second, plain(view(a)))
	}
	if second-first != 2 {
		t.Errorf("entries must be separated by exactly one blank line, got %d", second-first-1)
	}
}

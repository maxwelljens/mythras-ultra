package codex

import (
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// testHeadingRE independently extracts ATX headings from the source document,
// deliberately separate from the implementation's parser.
var testHeadingRE = regexp.MustCompile(`^#{1,6} (.*)$`)

func mustCorpus(t *testing.T) *Corpus {
	t.Helper()
	c, err := NewCorpus()
	if err != nil {
		t.Fatalf("NewCorpus() error: %v", err)
	}
	return c
}

func TestNewCorpusDocuments(t *testing.T) {
	c := mustCorpus(t)
	docs := c.Documents()
	if len(docs) == 0 {
		t.Fatal("Documents() is empty")
	}
	found := false
	for _, d := range docs {
		if d == "Mythras-Imperative.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("Documents() = %v, missing Mythras-Imperative.md", docs)
	}
	if !slices.Equal(docs, sortedCopy(docs)) {
		t.Errorf("Documents() = %v, not sorted", docs)
	}

	// Mutating the returned slice must not affect the corpus.
	docs[0] = "mutated.md"
	if again := c.Documents(); again[0] == "mutated.md" {
		t.Errorf("Documents() exposes internal state: %v", again)
	}
}

func TestSectionsMatchDocumentHeadings(t *testing.T) {
	c := mustCorpus(t)
	secs := c.Sections()
	if len(secs) == 0 {
		t.Fatal("Sections() is empty")
	}

	raw, err := fs.ReadFile(dataFS, "data/Mythras-Imperative.md")
	if err != nil {
		t.Fatalf("read embedded rulebook: %v", err)
	}
	var want []string
	for _, line := range strings.Split(string(raw), "\n") {
		if m := testHeadingRE.FindStringSubmatch(line); m != nil {
			want = append(want, strings.TrimSpace(m[1]))
		}
	}
	if len(want) == 0 {
		t.Fatal("test found no headings in the embedded rulebook")
	}

	// The rulebook starts with a heading, so there is no preamble section
	// and every section heading must appear, in document order.
	var got []string
	for _, s := range secs {
		if s.Heading != "" {
			got = append(got, s.Heading)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("section headings: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("heading[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSectionDetails(t *testing.T) {
	c := mustCorpus(t)
	tests := []struct {
		heading string
		level   int
		path    string
		phrase  string
	}{
		{"Rounding of Numbers and Results", 2,
			"Mythras-Imperative.md#rounding-of-numbers-and-results",
			"always round up to the whole number"},
		{"Dice Used", 2, "Mythras-Imperative.md#dice-used", "d100"},
		{"Action Points", 3, "Mythras-Imperative.md#action-points",
			"characters have 2 Action Points"},
		{"Hit Points", 3, "Mythras-Imperative.md#hit-points",
			"before it becomes useless"},
		{"Hit Locations", 2, "Mythras-Imperative.md#hit-locations",
			"specific Hit Location"},
		{"Combat", 1, "Mythras-Imperative.md#combat",
			"Violent confrontation"},
	}
	for _, tt := range tests {
		var found []Section
		for _, s := range c.Sections() {
			if s.Heading == tt.heading {
				found = append(found, s)
			}
		}
		if len(found) == 0 {
			t.Errorf("heading %q not found", tt.heading)
			continue
		}
		s := found[0]
		if s.Level != tt.level {
			t.Errorf("heading %q Level = %d, want %d", tt.heading, s.Level, tt.level)
		}
		if s.Path != tt.path {
			t.Errorf("heading %q Path = %q, want %q", tt.heading, s.Path, tt.path)
		}
		if s.Doc != "Mythras-Imperative.md" {
			t.Errorf("heading %q Doc = %q, want Mythras-Imperative.md", tt.heading, s.Doc)
		}
		if !strings.Contains(s.Text, tt.phrase) {
			t.Errorf("heading %q Text does not contain %q", tt.heading, tt.phrase)
		}
	}
}

func TestParseSections(t *testing.T) {
	content := "preamble intro\r\n\r\n" +
		"# One\r\nbody one\r\n\r\n" +
		"####### not a heading\r\n" +
		"## Two Deep\r\ntwo body\r\n"
	secs := parseSections("doc.md", content)
	if len(secs) != 3 {
		t.Fatalf("parseSections returned %d sections, want 3", len(secs))
	}

	pre := secs[0]
	if pre.Heading != "" || pre.Level != 0 || pre.Path != "doc.md" {
		t.Errorf("preamble = %+v, want empty heading, level 0, path doc.md", pre)
	}
	if pre.Text != "preamble intro" {
		t.Errorf("preamble Text = %q, want %q", pre.Text, "preamble intro")
	}

	if secs[1].Heading != "One" || secs[1].Level != 1 || secs[1].Path != "doc.md#one" {
		t.Errorf("section 1 = %+v, want heading One, level 1, path doc.md#one", secs[1])
	}
	if secs[1].Text != "body one\r\n\r\n####### not a heading" {
		t.Errorf("section 1 Text = %q, want body through next heading", secs[1].Text)
	}
	if secs[2].Heading != "Two Deep" || secs[2].Level != 2 || secs[2].Path != "doc.md#two-deep" {
		t.Errorf("section 2 = %+v, want heading Two Deep, level 2, path doc.md#two-deep", secs[2])
	}
	if secs[2].Text != "two body" {
		t.Errorf("section 2 Text = %q, want %q", secs[2].Text, "two body")
	}

	// A document that starts with a heading has no preamble section.
	noPre := parseSections("doc.md", "# Only\r\ntext\r\n")
	if len(noPre) != 1 || noPre[0].Heading != "Only" {
		t.Errorf("parseSections = %+v, want a single Only section", noPre)
	}
}

func TestSectionsReturnsCopy(t *testing.T) {
	c := mustCorpus(t)
	secs := c.Sections()
	if len(secs) == 0 {
		t.Fatal("Sections() is empty")
	}
	secs[0] = Section{}
	again := c.Sections()
	if again[0].Heading == "" && again[0].Text == "" {
		t.Error("Sections() exposes internal state")
	}
}

func TestSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Step 1: Character Concept", "step-1-character-concept"},
		{"Hit Locations", "hit-locations"},
		{"So, Just How Good Are You?", "so-just-how-good-are-you"},
		{"Culture (INT x2)", "culture-int-x2"},
	}
	for _, tt := range tests {
		if got := slug(tt.in); got != tt.want {
			t.Errorf("slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	slices.Sort(out)
	return out
}

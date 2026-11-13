package codex

import (
	"reflect"
	"testing"
)

// TestSearchGolden pins rankings against the real embedded rulebook: each
// query must return hits, surface its canonical heading(s) in the top three,
// and produce well-formed snippets. Expectations were derived from the
// actual content of data/Mythras-Imperative.md.
func TestSearchGolden(t *testing.T) {
	c := mustCorpus(t)
	tests := []struct {
		query  string
		top3   []string // headings expected among the first min(3, len) hits
		minHit int
	}{
		{"action points", []string{"Action Points"}, 1},
		{"parry", []string{"Parry", "Unable or Unwilling to Parry"}, 1},
		{"dodge", []string{"Combat Styles", "Evade (DEX x2)"}, 1},
		{"hit points", []string{"Hit Points"}, 1},
		{"damage", []string{"Damage Modifier"}, 1},
		{"skill", []string{"The Skill Pyramid", "Modifying Skills"}, 1},
		{"experience", []string{"Experience Rolls", "Experience Modifier"}, 1},
		{"hit location", []string{"Hit Locations"}, 1},
		{"combat", []string{"Combat Styles", "Combat Actions"}, 1},
		{"culture", []string{"Step 5: Culture"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			hits := c.Search(tt.query, 0)
			if len(hits) < tt.minHit {
				t.Fatalf("Search(%q) returned %d hits, want >= %d", tt.query, len(hits), tt.minHit)
			}

			limit := 3
			if len(hits) < limit {
				limit = len(hits)
			}
			inTop := make(map[string]bool, limit)
			for _, h := range hits[:limit] {
				inTop[h.Section.Heading] = true
			}
			for _, want := range tt.top3 {
				if !inTop[want] {
					t.Errorf("Search(%q): heading %q not in top-%d (got %v)",
						tt.query, want, limit, headings(hits[:limit]))
				}
			}

			// Scores rank descending; every hit is from the rulebook
			// and carries a usable snippet.
			for i, h := range hits {
				if i > 0 && h.Score > hits[i-1].Score {
					t.Errorf("hits not sorted by score: %v then %v", hits[i-1].Score, h.Score)
				}
				if h.Section.Doc != "Mythras-Imperative.md" {
					t.Errorf("hit %d doc = %q", i, h.Section.Doc)
				}
				if h.Snippet == "" {
					t.Errorf("hit %d (%s) has empty snippet", i, h.Section.Heading)
				}
				if len(h.Snippet) > 400 {
					t.Errorf("hit %d (%s) snippet is %d bytes, want <= 400",
						i, h.Section.Heading, len(h.Snippet))
				}
			}
		})
	}
}

func TestSearchDeterministic(t *testing.T) {
	c1, err := NewCorpus()
	if err != nil {
		t.Fatalf("NewCorpus() error: %v", err)
	}
	c2 := mustCorpus(t)

	a := c1.Search("hit location", 0)
	b := c1.Search("hit location", 0)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("same corpus, same query differs:\n%v\n%v", a, b)
	}
	d := c2.Search("hit location", 0)
	if !reflect.DeepEqual(a, d) {
		t.Errorf("fresh corpus differs from first corpus:\n%v\n%v", a, d)
	}
}

func TestSearchLimit(t *testing.T) {
	c := mustCorpus(t)
	full := c.Search("damage", 0)
	if len(full) != 8 {
		t.Fatalf("default limit: got %d hits, want 8", len(full))
	}
	if got := c.Search("damage", -3); len(got) != 8 {
		t.Errorf("negative limit: got %d hits, want 8", len(got))
	}
	if got := c.Search("damage", 3); !reflect.DeepEqual(got, full[:3]) {
		t.Errorf("limit 3 should be the first 3 default hits, got %v", headings(got))
	}
	if got := c.Search("damage", 1); len(got) != 1 {
		t.Errorf("limit 1: got %d hits, want 1", len(got))
	}
	if got := c.Search("damage", 100); len(got) != 50 {
		t.Errorf("limit 100 should cap at 50, got %d", len(got))
	}
	if got := c.Search("damage", 50); len(got) != 50 {
		t.Errorf("limit 50: got %d hits, want 50", len(got))
	}
}

func TestSearchEmptyAndUselessQueries(t *testing.T) {
	c := mustCorpus(t)
	for _, q := range []string{"", "   ", "\t\n", "a", "a i o", "-- !!"} {
		if hits := c.Search(q, 0); len(hits) != 0 {
			t.Errorf("Search(%q) = %d hits, want 0", q, len(hits))
		}
	}
}

func TestQueryTerms(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"Action Points", []string{"action", "points"}},
		{"Hit  location,", []string{"hit", "location"}},
		{"parry parry", []string{"parry"}},
		{"a i", nil},
		{"hit-location", []string{"hit", "location"}},
	}
	for _, tt := range tests {
		got := queryTerms(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("queryTerms(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func headings(hits []Hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Section.Heading
	}
	return out
}

package codex

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Search limits: a non-positive limit means defaultLimit; results are never
// capped above maxLimit, and snippets never exceed maxSnippet bytes.
const (
	defaultLimit = 8
	maxLimit     = 50
	maxSnippet   = 400
	// headingBoostFactor multiplies a term's idf when the term appears in
	// a section's heading: a heading hit is a strong topical signal, and
	// must outweigh raw term frequency in long grab-bag sections.
	headingBoostFactor = 8.0
)

// Hit is one ranked search result.
type Hit struct {
	// Section is the matching rule section.
	Section Section
	// Score is the ranking score; higher is better, ties keep document
	// and section order.
	Score float64
	// Snippet is a short excerpt around the best match: at most 400
	// bytes, cut at word boundaries and ellipsed when truncated.
	Snippet string
}

// Search ranks sections against a whitespace-separated query using
// deterministic term-frequency/idf-lite scoring: term frequency in the
// section weighted by idf, a large bonus for terms appearing in the
// heading, a coverage factor favouring sections that match more distinct
// query terms, and a small bonus for shallower, earlier sections. Terms
// shorter than two characters are ignored. Results are ordered by score
// descending, then document order, then section order. An empty query (or
// one with no usable terms) returns no hits and no error. limit <= 0 means
// 8 results; limits above 50 are capped at 50.
func (c *Corpus) Search(query string, limit int) []Hit {
	terms := queryTerms(query)
	if len(terms) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	n := float64(len(c.entries))
	// Document frequency of each query term across sections (heading or
	// body counts), then its idf-lite weight.
	df := make(map[string]int, len(terms))
	idf := make(map[string]float64, len(terms))
	for _, t := range terms {
		for _, e := range c.entries {
			if e.body[t] > 0 || e.head[t] > 0 {
				df[t]++
			}
		}
		if df[t] > 0 {
			idf[t] = math.Log(1 + n/float64(df[t]))
		}
	}

	var hits []Hit
	for _, e := range c.entries {
		var sum float64
		matched := 0
		for _, t := range terms {
			bh, hh := e.body[t], e.head[t]
			if bh == 0 && hh == 0 {
				continue
			}
			matched++
			sum += float64(bh+hh) * idf[t]
			if hh > 0 {
				sum += headingBoostFactor * idf[t]
			}
		}
		if matched == 0 {
			continue
		}
		// Coverage: sections matching more distinct query terms score
		// proportionally higher; a partial match still scores.
		coverage := float64(matched) / float64(len(terms))
		score := sum * (0.5 + 0.5*coverage)
		// Small positional bonus: shallower and earlier sections win
		// otherwise-equal ties (a preamble counts as level 1).
		level := e.sec.Level
		if level < 1 {
			level = 1
		}
		score += 0.5/float64(level) + 0.1/float64(1+e.ord)
		hits = append(hits, Hit{Section: e.sec, Score: score})
	}

	// Stable sort keeps ties in document order, then section order.
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	for i := range hits {
		src := hits[i].Section.Text
		if src == "" {
			// Heading-only section: excerpt the heading instead of
			// returning an empty snippet.
			src = hits[i].Section.Heading
		}
		hits[i].Snippet = makeSnippet(src, terms)
	}
	return hits
}

// queryTerms splits a query into lowercased terms of at least two runes,
// dropping non-alphanumeric punctuation and duplicates while keeping first
// occurrence order (so scoring sums stay deterministic).
func queryTerms(query string) []string {
	seen := make(map[string]bool)
	var terms []string
	for _, tok := range splitAlnum(query) {
		if utf8.RuneCountInString(tok) < 2 || seen[tok] {
			continue
		}
		seen[tok] = true
		terms = append(terms, tok)
	}
	return terms
}

// splitAlnum lowercases s and splits it on runs of non-alphanumeric runes,
// the tokenisation shared by queries, section bodies and headings.
func splitAlnum(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// tokenCounts returns the frequency of every token in s.
func tokenCounts(s string) map[string]int {
	counts := make(map[string]int)
	for _, tok := range splitAlnum(s) {
		counts[tok]++
	}
	return counts
}

// wordSpan is a byte range [start, end) of one whitespace-delimited word.
type wordSpan struct{ start, end int }

// makeSnippet returns the first window of the text that contains the most
// distinct query terms, within maxSnippet bytes, cut at word boundaries and
// prefixed/suffixed with ellipses where truncated. Text at or below the
// budget is returned unchanged (only trimmed).
func makeSnippet(text string, terms []string) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxSnippet {
		return text
	}
	words := wordSpans(text)
	if len(words) == 0 {
		return ""
	}

	// Slide a window over the words; ellipses cost 4 bytes each side
	// ("... " / " ...") while that side truncates the text.
	bestStart, bestEnd := 0, 0
	bestLeft, bestRight := false, false
	bestDistinct := -1
	for s := 0; s < len(words); s++ {
		e := s
		for e < len(words) {
			overhead := 0
			if s > 0 {
				overhead += 4
			}
			if e+1 < len(words) {
				overhead += 4
			}
			if words[e].end-words[s].start+overhead > maxSnippet {
				break
			}
			e++
		}
		if e == s {
			// Even one word exceeds the budget (pathologically long
			// token); skip this start and fall back if needed.
			continue
		}
		distinct := countDistinct(text[words[s].start:words[e-1].end], terms)
		if distinct > bestDistinct {
			bestDistinct = distinct
			bestStart, bestEnd = words[s].start, words[e-1].end
			bestLeft, bestRight = s > 0, e < len(words)
		}
	}
	if bestDistinct < 0 {
		// No word ever fit; hard-cut at a rune boundary instead of
		// exceeding the budget or panicking.
		return truncateRunes(text, maxSnippet-4) + "..."
	}

	var b strings.Builder
	b.Grow(bestEnd - bestStart + 8)
	if bestLeft {
		b.WriteString("... ")
	}
	b.WriteString(text[bestStart:bestEnd])
	if bestRight {
		b.WriteString(" ...")
	}
	return b.String()
}

// wordSpans returns the byte range of each whitespace-delimited word.
func wordSpans(s string) []wordSpan {
	var out []wordSpan
	start := -1
	for i, r := range s {
		if unicode.IsSpace(r) {
			if start >= 0 {
				out = append(out, wordSpan{start, i})
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, wordSpan{start, len(s)})
	}
	return out
}

// countDistinct reports how many of the query terms occur as tokens in win.
func countDistinct(win string, terms []string) int {
	present := make(map[string]bool)
	for _, tok := range splitAlnum(win) {
		present[tok] = true
	}
	n := 0
	for _, t := range terms {
		if present[t] {
			n++
		}
	}
	return n
}

// truncateRunes cuts s to at most n bytes without splitting a UTF-8 rune.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

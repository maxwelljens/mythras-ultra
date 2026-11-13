// Package codex embeds the Mythras Imperative rulebook in the myul binary
// and offers deterministic, dependency-free section parsing and search, so
// the LLM game master can ground its rulings in the actual rules (tool
// rules_lookup).
package codex

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

// dataFS carries every rule document shipped with the binary. Documents are
// discovered from the glob, so additional Markdown files dropped under
// data/ become searchable documents without code changes.
//
//go:embed data/*.md
var dataFS embed.FS

// headingRE matches an ATX markdown heading of level 1..6. A line with seven
// or more hashes is not a heading and must not match.
var headingRE = regexp.MustCompile(`^(#{1,6})[ \t]+(.*)$`)

// slugSepRE matches the runs of characters that separate words in a heading
// when building a URL-style anchor slug.
var slugSepRE = regexp.MustCompile(`[^a-z0-9]+`)

// Section is one heading-delimited block of a rule document.
type Section struct {
	// Doc is the document's file name, e.g. "Mythras-Imperative.md".
	Doc string
	// Heading is the nearest markdown heading; empty for the preamble
	// before a document's first heading.
	Heading string
	// Level is the heading depth, 1..6, or 0 for the preamble.
	Level int
	// Path is a stable anchor of the form "Doc.md#slug-of-heading", or
	// just "Doc.md" for the preamble. Duplicate headings (the rulebook
	// has several sections named "Skills") share the same slug.
	Path string
	// Text is the raw markdown body between this heading and the next
	// heading (or end of document).
	Text string
}

// Corpus is an immutable parsed view of every embedded rule document, built
// once by NewCorpus. All methods are safe for concurrent reads and return
// copies, so callers cannot mutate the corpus.
type Corpus struct {
	docs    []string
	entries []entry
}

// entry couples a section with token frequencies precomputed at load time
// and its 0-based position within its document, used for tie-breaking.
type entry struct {
	sec  Section
	body map[string]int // body token -> occurrences
	head map[string]int // heading token -> occurrences
	ord  int            // position within the document, 0-based
}

// NewCorpus loads and parses every embedded document matching data/*.md.
// Document names are the file names, sorted lexicographically. It returns an
// error if the embed glob fails, no documents are found, or a document
// cannot be read.
func NewCorpus() (*Corpus, error) {
	paths, err := fs.Glob(dataFS, "data/*.md")
	if err != nil {
		return nil, fmt.Errorf("codex: glob embedded rule documents: %w", err)
	}
	if len(paths) == 0 {
		return nil, errors.New("codex: no embedded rule documents matched data/*.md")
	}
	sort.Strings(paths)

	c := &Corpus{docs: make([]string, 0, len(paths))}
	for _, p := range paths {
		raw, err := fs.ReadFile(dataFS, p)
		if err != nil {
			return nil, fmt.Errorf("codex: read embedded rule document %s: %w", p, err)
		}
		name := path.Base(p)
		c.docs = append(c.docs, name)
		for i, sec := range parseSections(name, string(raw)) {
			c.entries = append(c.entries, entry{
				sec:  sec,
				body: tokenCounts(sec.Text),
				head: tokenCounts(sec.Heading),
				ord:  i,
			})
		}
	}
	return c, nil
}

// Documents returns the sorted names of all embedded rule documents.
func (c *Corpus) Documents() []string {
	return append([]string(nil), c.docs...)
}

// Sections returns every parsed section across all documents, in document
// order and then heading order within each document.
func (c *Corpus) Sections() []Section {
	secs := make([]Section, len(c.entries))
	for i, e := range c.entries {
		secs[i] = e.sec
	}
	return secs
}

// parseSections splits a markdown document into sections: an optional
// preamble (content before the first heading, with an empty Heading), then
// one section per ATX heading running until the next heading.
func parseSections(doc, content string) []Section {
	lines := strings.Split(content, "\n")

	type heading struct {
		line  int
		level int
		text  string
	}
	var heads []heading
	for i, ln := range lines {
		if m := headingRE.FindStringSubmatch(ln); m != nil {
			heads = append(heads, heading{
				line:  i,
				level: len(m[1]),
				text:  strings.TrimSpace(m[2]),
			})
		}
	}

	var secs []Section
	// Preamble: everything before the first heading; omitted when blank
	// so documents starting with a heading yield no empty section.
	preambleEnd := len(lines)
	if len(heads) > 0 {
		preambleEnd = heads[0].line
	}
	if text := strings.TrimSpace(strings.Join(lines[:preambleEnd], "\n")); text != "" {
		secs = append(secs, Section{Doc: doc, Path: doc, Text: text})
	}

	for k, h := range heads {
		end := len(lines)
		if k+1 < len(heads) {
			end = heads[k+1].line
		}
		secs = append(secs, Section{
			Doc:     doc,
			Heading: h.text,
			Level:   h.level,
			Path:    anchor(doc, h.text),
			Text:    strings.TrimSpace(strings.Join(lines[h.line+1:end], "\n")),
		})
	}
	return secs
}

// anchor renders a section's stable path, "Doc.md#slug-of-heading", or just
// "Doc.md" for a heading-less preamble.
func anchor(doc, heading string) string {
	if heading == "" {
		return doc
	}
	return doc + "#" + slug(heading)
}

// slug lowercases a heading and collapses every run of non-alphanumeric
// characters into a single hyphen, e.g. "Step 1: Character Concept" becomes
// "step-1-character-concept".
func slug(heading string) string {
	s := slugSepRE.ReplaceAllString(strings.ToLower(heading), "-")
	return strings.Trim(s, "-")
}

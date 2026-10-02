// Package headings reads the headings of an already-rendered HTML fragment:
// the docs site's on-this-page contents and the workbench Document page's
// contents rail both list the same headings, with the same ids, by calling
// this one extraction (spec/document-page-v2, ledger SI-340 (1)).
//
// It is a second pass over the rendered bytes rather than a second walk of
// the Markdown AST: simpler, and just as deterministic, because it is a pure
// function of the bytes goldmark emitted. The ids are the ones goldmark's
// WithAutoHeadingID gave each heading (internal/render), so a link to an
// entry's id lands on that heading in the same fragment.
package headings

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

// headingRe matches every h2–h4 heading goldmark emitted with an
// auto-generated id: <h2 id="foo">Text</h2>.
var headingRe = regexp.MustCompile(`(?s)<h([2-4]) id="([^"]*)">(.*?)</h[2-4]>`)

// innerTagRe strips any nested tags (e.g. <code>, <em>) a heading's inline
// Markdown produced, so an entry's text is plain.
var innerTagRe = regexp.MustCompile(`<[^>]+>`)

// Entry is one heading: its level (2–4), its id, and its plain text.
type Entry struct {
	Level int
	ID    string
	Text  string
}

// Extract returns renderedHTML's h2–h4 headings that carry an id, in
// document order; none is an empty slice.
func Extract(renderedHTML string) []Entry {
	matches := headingRe.FindAllStringSubmatch(renderedHTML, -1)
	entries := make([]Entry, 0, len(matches))
	for _, m := range matches {
		level, err := strconv.Atoi(m[1])
		if err != nil {
			level = 2
		}
		text := strings.TrimSpace(innerTagRe.ReplaceAllString(m[3], ""))
		entries = append(entries, Entry{Level: level, ID: m[2], Text: html.UnescapeString(text)})
	}
	return entries
}

// Sections keeps only the level-2 entries, in order: a document's section
// headings. Its h3s are whole sentences (a decision's text, a criterion's
// text), which would turn a contents list into a wall of prose. None is
// nil.
func Sections(entries []Entry) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.Level == 2 {
			out = append(out, e)
		}
	}
	return out
}

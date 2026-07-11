package dex

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/OWNER/verdi/internal/render"
)

// renderMarkdown and highlightCode delegate to internal/render, the shared
// goldmark+chroma machinery (05 §Verdi-dex mechanics: "markdown via
// goldmark and syntax highlighting via chroma at build time"). Package
// dex used to own this code directly; it moved to internal/render once
// internal/workbench needed the identical rendering (CLAUDE.md: "anything
// used by two or more packages lives in a shared internal/ package") —
// kept as package-level vars here (not a straight rename at every call
// site) so this file's own tests, and every other dex file's call sites,
// are untouched.
var (
	renderMarkdown = render.RenderMarkdown
	renderBody     = render.RenderBody
	highlightCode  = render.HighlightCode
)

// tocHeadingRe extracts every heading goldmark emitted with an
// auto-generated id — <h2 id="foo">Text</h2> — to build the on-this-page
// TOC (05 §Verdi-dex page anatomy) as a second pass over already-rendered
// HTML rather than a second AST walk; simpler, and just as deterministic
// since it's a pure function of the same rendered bytes.
var tocHeadingRe = regexp.MustCompile(`(?s)<h([2-4]) id="([^"]*)">(.*?)</h[2-4]>`)

// innerTagRe strips any nested tags (e.g. <code>, <em>) a heading's inline
// markdown produced, so the TOC shows plain text labels.
var innerTagRe = regexp.MustCompile(`<[^>]+>`)

// TOCEntry is one on-this-page table-of-contents entry.
type TOCEntry struct {
	Level int
	ID    string
	Text  string
}

// extractTOC walks renderedHTML's h2-h4 headings (goldmark's
// WithAutoHeadingID gave each one a stable id) in document order.
func extractTOC(renderedHTML string) []TOCEntry {
	matches := tocHeadingRe.FindAllStringSubmatch(renderedHTML, -1)
	entries := make([]TOCEntry, 0, len(matches))
	for _, m := range matches {
		level, err := strconv.Atoi(m[1])
		if err != nil {
			level = 2
		}
		text := strings.TrimSpace(innerTagRe.ReplaceAllString(m[3], ""))
		entries = append(entries, TOCEntry{Level: level, ID: m[2], Text: html.UnescapeString(text)})
	}
	return entries
}

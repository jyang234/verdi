package dex

import (
	"github.com/jyang234/verdi/internal/headings"
	"github.com/jyang234/verdi/internal/render"
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

// TOCEntry is one on-this-page table-of-contents entry: the shared heading
// extraction's entry (internal/headings), which the workbench's Document
// page reads too (spec/document-page-v2, ledger SI-340 (1)).
type TOCEntry = headings.Entry

// extractTOC walks renderedHTML's h2-h4 headings (goldmark's
// WithAutoHeadingID gave each one a stable id) in document order, through
// the shared extraction (05 §Verdi-dex page anatomy: the on-this-page TOC).
func extractTOC(renderedHTML string) []TOCEntry {
	return headings.Extract(renderedHTML)
}

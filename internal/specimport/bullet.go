package specimport

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmtext "github.com/yuin/goldmark/text"
)

// bulletItem is one direct child of a flat Markdown bullet list, located in
// one source's SELECTED bytes.
type bulletItem struct {
	// ordinal is the item's 1-based position among its list's direct
	// items, counted even when the item itself is not representable, so
	// generated ids keep their source-position numbering
	// (spec-import-contract.md residual 4).
	ordinal int
	// supported is false for an item this package cannot represent as a
	// span at all: an empty item, or one containing a nested list.
	supported bool
	// markerStart is the offset of the item's own bullet marker; start/end
	// are its CONTENT span, excluding the marker and the final line's
	// trailing terminator. start/end is the span automatic extraction
	// records.
	markerStart, start, end int
	// text is the list-item transform's output for this item.
	text string
}

// bulletItemsOf returns one bulletItem per direct child of list, in source
// order, with base added to every offset so the result is expressed in the
// source's selected-byte coordinates rather than the post-frontmatter
// body's.
//
// start/end is the item's COMPLETE body: from the first byte past its own
// bullet marker to the last non-blank byte before the next item (or before
// whatever closes the list), so an opening and closing fence delimiter, a
// blockquote marker and the blank line between two blocks of one item are
// all inside the recorded span.
//
// Neither boundary may be derived from goldmark's per-line leaf segments.
// Those segments are the INTERIOR of whatever block they belong to:
// goldmark emits none at all for a fence delimiter or a blockquote marker
// and trims a paragraph's trailing newline when it closes the block. A
// segment-derived extent therefore opened past the item's own first-block
// syntax and closed before its last, and a segment-derived text both
// dropped that syntax and fused the item's words across a block boundary —
// all of it published as copied-source. The item's real extent comes from
// its own recorded line start plus goldmark's recorded content column, and
// from the real block boundary that follows it.
//
// text is the declared list-item transform over exactly those bytes — a
// bullet-marker strip (start already begins past the marker) plus the
// deterministic continuation deindent of deindentItemBody, and nothing
// else. Automatic extraction and explicit list-item mappings share this one
// implementation rather than each deriving a text of their own.
func bulletItemsOf(body []byte, base int, list ast.Node) []bulletItem {
	listLimit := nextBlockLineStart(body, list, len(body))

	var items []bulletItem
	ordinal := 0
	for node := list.FirstChild(); node != nil; node = node.NextSibling() {
		ordinal++
		markerStart, contentCol, located := bulletItemMarker(body, node)
		if !located || containsNestedList(node) {
			items = append(items, bulletItem{ordinal: ordinal})
			continue
		}
		rawEnd := nextBlockLineStart(body, node, listLimit)
		rawStart := markerStart + contentCol
		if rawStart > rawEnd {
			rawStart = rawEnd
		}
		start, end := trimBodyRange(body, rawStart, rawEnd)
		if start >= end {
			// No body bytes at all: an empty item contributes nothing, but
			// its ordinal is still consumed. This is a real emptiness test
			// over the item's own bytes — an empty fenced block or an empty
			// blockquote carries no leaf segment yet is ordinary flat-list
			// content, and must not be dropped as if the item were blank.
			items = append(items, bulletItem{ordinal: ordinal})
			continue
		}
		if lineStartBefore(body, start) != markerStart {
			// The marker's own line carried no content, so the body begins
			// on a later line whose continuation indentation is alignment,
			// not content.
			start += leadingSpaceCount(body[start:end], contentCol)
		}
		items = append(items, bulletItem{
			ordinal:     ordinal,
			supported:   true,
			markerStart: base + markerStart,
			start:       base + start,
			end:         base + end,
			text:        deindentItemBody(body[start:end], contentCol),
		})
	}
	return items
}

// bulletItemMarker returns the line start of node's own bullet marker and
// the content column that marker sets, both in body's coordinates.
//
// contentCol is goldmark's own ast.ListItem.Offset — the column, relative
// to the item's line start, at which the item's content begins. Reading it
// rather than rescanning the line keeps one CommonMark rule in one place:
// the marker's width plus the 1-4 spaces after it, or the marker's width
// plus one when the item opens with a blank line or with more than four
// spaces (which start an indented code block instead of widening the item).
func bulletItemMarker(body []byte, node ast.Node) (markerStart, contentCol int, ok bool) {
	item, isItem := node.(*ast.ListItem)
	if !isItem || item.Offset <= 0 {
		return 0, 0, false
	}
	lineStart, located := blockLineStart(body, node)
	if !located || lineStart+item.Offset > len(body) {
		return 0, 0, false
	}
	return lineStart, item.Offset, true
}

// leadingSpaceCount returns how many leading ASCII spaces data begins with,
// counting at most limit of them.
func leadingSpaceCount(data []byte, limit int) int {
	n := 0
	for n < limit && n < len(data) && data[n] == ' ' {
		n++
	}
	return n
}

// nextBlockLineStart returns the line start of the block following n among
// its siblings, or fallback when n is the last sibling or that block has no
// locatable source position.
func nextBlockLineStart(body []byte, n ast.Node, fallback int) int {
	next := n.NextSibling()
	if next == nil {
		return fallback
	}
	at, ok := blockLineStart(body, next)
	if !ok {
		return fallback
	}
	return at
}

// deindentItemBody is the declared deterministic deindent transform
// (spec-import-contract.md: "Strip only the bullet marker and its following
// space; multiline continuation indentation is a declared deterministic
// deindent transform"). raw is one item's complete body, already starting
// past its bullet marker; indent is the column that marker and its
// following space occupied, so every continuation line of that item carries
// at most that much alignment indentation.
//
// Exactly up to indent leading ASCII SPACES are removed from each line
// after the first. Nothing else is touched: interior blank lines, line
// terminators (LF or CRLF), fence delimiters, blockquote markers, trailing
// spaces and every word and punctuation mark are reproduced byte for byte.
// A line indented with a tab keeps it, because one tab is not a known
// number of columns and silently consuming it would not be a reversible
// deindent.
func deindentItemBody(raw []byte, indent int) string {
	if indent <= 0 {
		return string(raw)
	}
	var buf bytes.Buffer
	buf.Grow(len(raw))
	for i, first := 0, true; i < len(raw); first = false {
		lineEnd := i
		for lineEnd < len(raw) && raw[lineEnd] != '\n' {
			lineEnd++
		}
		if lineEnd < len(raw) {
			lineEnd++ // keep this line's own terminator
		}
		line := raw[i:lineEnd]
		if !first {
			line = line[leadingSpaceCount(line, indent):]
		}
		buf.Write(line)
		i = lineEnd
	}
	return buf.String()
}

// supportedBulletItems returns every direct item of every top-level flat
// bullet list in one source's selected bytes — exactly the shape automatic
// extraction supports. It is the structural validator an explicit
// list-item Mapping is checked against, so "list-item is valid only for an
// actual supported direct list item span" (spec-import-contract.md) is
// enforced against the source's real Markdown structure instead of assumed
// from the caller's offsets. A list nested inside another list or any other
// container is not a direct item of a supported flat list and is not
// returned.
func supportedBulletItems(selected []byte) ([]bulletItem, error) {
	bodyStart, err := stripFrontmatter(selected)
	if err != nil {
		return nil, err
	}
	body := selected[bodyStart:]
	doc := goldmark.New().Parser().Parse(gmtext.NewReader(body))

	var items []bulletItem
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		list, ok := bulletList(node)
		if !ok {
			continue
		}
		for _, item := range bulletItemsOf(body, bodyStart, list) {
			if item.supported {
				items = append(items, item)
			}
		}
	}
	return items, nil
}

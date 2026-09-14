package splice

// The creation-only body-section primitives internal/specimport's Compose
// needs (spec-import-contract.md, "Candidate and validation"): a freshly
// rendered scaffold's own known placeholder body sections ("## Problem",
// "## Outcome", the placeholder acceptance criterion's own heading) must
// be mirrored with real mapped text, or removed outright once orphaned by
// a removed placeholder frontmatter entry. Nothing in ops.go reaches body
// prose at all: SetObjectText/AppendObject/RemoveObjectEntry only ever
// touch the FRONTMATTER half of an object (RemoveObjectEntry's own doc
// comment is explicit: "its body prose and anchor heading are NEVER
// touched: prose is not silently destroyed"). These two operations are
// the narrow, tested exception the plan anticipates for exactly this
// creation-time gap — never a general prose-editing surface for an
// already-accepted spec.

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// headingLine is one ATX heading found while scanning a document's body:
// its own whole-line span [start, end) (end just past the line's trailing
// newline, or end of document for the last line), and the anchor slug its
// heading text resolves to (artifact.SlugifyHeading — the identical
// transform ResolveObjectAnchors compares anchors against, so a section
// this package locates is always exactly the section anchor resolution
// would find, never a second, drifting heading resolver).
type headingLine struct {
	start, end int
	slug       string
}

// scanHeadings walks d's body — everything after the frontmatter's own
// closing delimiter line — and returns every ATX ("#".."######") heading
// line it finds, in document order.
func (d *Doc) scanHeadings() ([]headingLine, error) {
	i := d.fmCloseOffset
	for i < len(d.src) && d.src[i] != '\n' {
		i++
	}
	if i >= len(d.src) {
		return nil, fmt.Errorf("splice: frontmatter closing delimiter has no line ending")
	}
	i++ // first byte of the body

	var out []headingLine
	for i < len(d.src) {
		lineStart := i
		lineEnd := i
		for lineEnd < len(d.src) && d.src[lineEnd] != '\n' {
			lineEnd++
		}
		next := lineEnd
		if next < len(d.src) {
			next++ // consume the line's own trailing newline
		}

		line := strings.TrimRight(string(d.src[lineStart:lineEnd]), "\r")
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "#") {
			j := 0
			for j < len(trimmed) && trimmed[j] == '#' {
				j++
			}
			rest := trimmed[j:]
			if j <= 6 && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
				text := strings.TrimSpace(rest)
				if text != "" {
					out = append(out, headingLine{start: lineStart, end: next, slug: artifact.SlugifyHeading(text)})
				}
			}
		}
		i = next
	}
	return out, nil
}

// findSection locates the ONE heading whose text resolves anchor
// (ResolveAnchor's identical slug-symmetric exact-match target: anchor and
// heading text both run through artifact.SlugifyHeading before comparing)
// and returns its own whole-line span plus its PROSE's end offset — the
// next heading's own start, or end of document when it is the last
// section. Zero or more than one match fails closed: a creation-time
// caller always knows exactly which single, just-rendered section it
// means, so an unresolvable or ambiguous anchor is a defect worth
// surfacing, never a silent no-op or an arbitrary pick.
func (d *Doc) findSection(anchor string) (heading headingLine, proseEnd int, err error) {
	headings, err := d.scanHeadings()
	if err != nil {
		return headingLine{}, 0, err
	}
	slug := artifact.SlugifyHeading(strings.TrimPrefix(anchor, "#"))
	found := -1
	for idx, h := range headings {
		if h.slug != slug {
			continue
		}
		if found >= 0 {
			return headingLine{}, 0, fmt.Errorf("splice: more than one heading resolves anchor %q; fail closed", anchor)
		}
		found = idx
	}
	if found == -1 {
		return headingLine{}, 0, fmt.Errorf("splice: no heading resolves anchor %q", anchor)
	}
	heading = headings[found]
	proseEnd = len(d.src)
	if found+1 < len(headings) {
		proseEnd = headings[found+1].start
	}
	return heading, proseEnd, nil
}

// SetSectionText replaces the PROSE of the body section whose heading
// resolves anchor — everything from just after the heading line through
// the next heading (or end of document) — with newText in the template's
// own canonical spacing (one blank line, the text, one blank line before
// whatever follows), leaving the heading line itself and every other
// section untouched. newText is inserted verbatim, including embedded
// newlines, so multiline mapped text survives exactly as selected.
func (d *Doc) SetSectionText(anchor, newText string) (Edit, error) {
	heading, proseEnd, err := d.findSection(anchor)
	if err != nil {
		return Edit{}, err
	}
	return Edit{Start: heading.end, End: proseEnd, Replace: "\n" + newText + "\n\n"}, nil
}

// RemoveSection deletes the ENTIRE body section whose heading resolves
// anchor — the heading line itself plus its prose, through the next
// heading (or end of document) — the exact inverse of AppendObject's own
// body-section insertion. It exists to delete a scaffold's now-orphaned
// placeholder section once its frontmatter entry is gone (e.g. the
// removed placeholder acceptance criterion's own "## Ac 1"), never as a
// general prose-removal operation over accepted content.
func (d *Doc) RemoveSection(anchor string) (Edit, error) {
	heading, proseEnd, err := d.findSection(anchor)
	if err != nil {
		return Edit{}, err
	}
	return Edit{Start: heading.start, End: proseEnd}, nil
}

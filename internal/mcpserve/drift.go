// I-17's anchor-drift algorithm: an annotation's target.selector pins a
// heading+quote at a specific commit; this file computes how that anchor
// stands against the CURRENT working tree (never re-resolved against the
// pinned commit — drift measures change SINCE the pin). Three-valued,
// exact-match, conservative toward moved/gone rather than fuzzy-healing
// (PLAN.md ledger I-17: "fuzzy matching (silently heals — forbidden)").
package mcpserve

import (
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// DriftStatus is one target selector's drift state against the current
// working tree (02 §Record schemas: "drift ... computed three-valued
// (fresh / moved / gone)").
type DriftStatus string

const (
	// DriftFresh: the selector's quote is still found within the section
	// under its pinned heading — the anchor holds exactly where it was
	// pinned.
	DriftFresh DriftStatus = "fresh"
	// DriftMoved: the quote is found verbatim somewhere else in the
	// current document (a different heading section, or outside any
	// heading) — the artifact changed shape around the anchor, but the
	// pinned text itself survives.
	DriftMoved DriftStatus = "moved"
	// DriftGone: the quote is not found anywhere in the current document
	// (including the artifact itself no longer existing) — the anchor no
	// longer has anything to point at.
	DriftGone DriftStatus = "gone"
)

// mdSection is one heading-delimited slice of a markdown body: the
// heading's own anchor slug, and the body text from just after that
// heading line up to (but excluding) the next heading line.
type mdSection struct {
	Anchor string
	Body   string
}

// splitSections partitions body into heading-delimited sections, keyed by
// the same anchor-slug convention internal/lint/headings.go's VL-014
// resolution uses (lowercase, spaces/hyphens preserved as hyphens, every
// other rune dropped) — reimplemented here rather than imported, since
// that helper is private to package lint; the two are kept in sync by
// TestMDSlugify_MatchesLintConvention below pinning shared example inputs.
// A leading section with no heading yet (anchor "") holds any preamble
// text before the document's first heading.
func splitSections(body string) []mdSection {
	lines := strings.Split(body, "\n")
	var sections []mdSection
	cur := mdSection{}
	var curLines []string

	flush := func() {
		cur.Body = strings.Join(curLines, "\n")
		sections = append(sections, cur)
		curLines = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimLeft(strings.TrimRight(line, "\r"), " \t")
		if level, text, ok := parseATXHeading(trimmed); ok {
			flush()
			cur = mdSection{Anchor: mdSlugify(text)}
			_ = level
			continue
		}
		curLines = append(curLines, line)
	}
	flush()
	return sections
}

// parseATXHeading reports whether trimmed is an ATX heading line ("# " .. "######")
// and returns its level and heading text.
func parseATXHeading(trimmed string) (level int, text string, ok bool) {
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	if i == 0 || i > 6 {
		return 0, "", false
	}
	rest := trimmed[i:]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return 0, "", false // e.g. "#foo" is not a heading
	}
	text = strings.TrimSpace(rest)
	if text == "" {
		return 0, "", false
	}
	return i, text, true
}

// mdSlugify computes a heading's anchor slug — see splitSections's doc for
// why this duplicates (rather than imports) internal/lint/headings.go's
// private slugify.
func mdSlugify(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// ComputeDrift implements I-17 for one selector against currentBody (the
// target artifact's CURRENT working-tree body — an empty currentBody,
// which also covers "the artifact no longer resolves at all", correctly
// falls through to DriftGone since no section can contain the quote).
func ComputeDrift(sel artifact.Selector, currentBody string) DriftStatus {
	pinnedAnchor := mdSlugify(sel.Heading)
	sections := splitSections(currentBody)

	// fresh: the quote is still under its pinned heading.
	for _, s := range sections {
		if s.Anchor == pinnedAnchor && strings.Contains(s.Body, sel.Quote) {
			return DriftFresh
		}
	}
	// moved: the quote survives verbatim somewhere else in the document.
	if sel.Quote != "" && strings.Contains(currentBody, sel.Quote) {
		return DriftMoved
	}
	return DriftGone
}

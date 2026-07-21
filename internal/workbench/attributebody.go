package workbench

// The placard body seam (05 §Workbench board polish): a case-file
// placard's problem/outcome ATTRIBUTE text (the concise frontmatter
// headline) is deliberately short; the fuller authored argument lives in
// the spec BODY under the "## Problem"/"## Outcome" heading the
// attribute's own anchor resolves to (02 §Object model). This file
// resolves that section and renders it through the SAME body-render path
// the corpus artifact page uses (render.RenderMarkdown, the markdown half
// of render.RenderBody — internal/workbench/corpus.go) — never a second
// markdown implementation — so a follow-on client pass can show it in a
// click-to-read-full-prose dialog. Pure function of the spec body and the
// attribute's anchor: no clock, no randomness, fail-soft throughout (this
// is read-only reference content, never load-bearing spec data — an
// absent or unresolvable anchor is normal, per artifact.Attribute's own
// optional-anchor callers, and must never fail the board render).

import (
	"html/template"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/render"
)

// attributeBodyHTML resolves attr's anchor to its body section (the
// markdown from just after the matching heading up to the next heading of
// any level) and renders it through render.RenderMarkdown — the artifact
// this section comes from is always a spec (only spec frontmatter carries
// problem:/outcome: attributes at all, 02 §Object model), so there is no
// kind to dispatch on the way render.RenderBody would for a whole
// artifact body; this is that same markdown path with the diagram
// special-case never in play.
//
// Returns "" (never an error) when attr is nil, its anchor is empty, the
// anchor resolves to no heading in body, the resolved section is blank,
// or rendering fails: this is presentational reference content, and a
// missing section falls back to the attribute's own headline text at the
// client (a follow-on Fable pass), never a defect worth failing the whole
// board render over.
func attributeBodyHTML(body []byte, attr *artifact.Attribute) template.HTML {
	if attr == nil {
		return ""
	}
	section, ok := bodySection(body, attr.Anchor)
	if !ok || section == "" {
		return ""
	}
	out, err := render.RenderMarkdown(section)
	if err != nil {
		return ""
	}
	return template.HTML(out)
}

// bodySection returns the markdown text of the heading section anchor (a
// "#slug" or bare-slug reference — 02 §Object model's anchor-resolution
// rule) resolves to: from just after the matching ATX heading line up to
// (excluding) the next ATX heading of any level, or the end of the
// document for the last section. The returned text is trimmed of leading
// and trailing blank lines (goldmark renders identically either way; this
// just keeps the "is there really anything here" emptiness check exact).
//
// ok is false — never an error — when anchor is empty or resolves to no
// heading in body, mirroring the "skip, don't fail" posture
// artifact.ResolveObjectAnchors already documents for an absent anchor.
//
// Resolution is slug-symmetric (spec/ritual-traps ac-1, superseding the
// earlier exact-match rule): BOTH the heading text AND the anchor pass
// through artifact.SlugifyHeading — the SAME transform
// artifact.HeadingAnchors/ResolveAnchor and internal/lint's VL-014 apply on
// each side — before the two slugs are compared. So an anchor written in
// the heading's own case (anchor: AC-1 against ## AC-1, X-1's witness)
// resolves here exactly as artifact.ResolveAnchor now resolves it, and a
// section this function finds is always the one the spec's own anchor
// validation would resolve to. Slugifying only the heading side (the pre-ac-1
// shape) reopened X-1's asymmetry invisibly at this render seam: a mixed-case
// anchor that validates and lints green found no section, and the placard
// dropped its authored prose with no finding anywhere. Only the
// ATX-heading-LINE recognition below (atxHeadingText) is local to this
// package, matching the same minimal "#".."######" shape
// artifact.HeadingAnchors and internal/mcpserve's own drift algorithm
// each recognize independently; none of the three import one another's
// private heading scanner.
func bodySection(body []byte, anchor string) (string, bool) {
	anchor = strings.TrimPrefix(anchor, "#")
	if anchor == "" {
		return "", false
	}

	lines := strings.Split(string(body), "\n")
	start := -1
	for i, line := range lines {
		text, ok := atxHeadingText(line)
		if !ok {
			continue
		}
		if start == -1 {
			if artifact.SlugifyHeading(text) == artifact.SlugifyHeading(anchor) {
				start = i + 1
			}
			continue
		}
		// The next heading of any level, once we're inside the matched
		// section, closes it.
		return strings.TrimSpace(strings.Join(lines[start:i], "\n")), true
	}
	if start == -1 {
		return "", false
	}
	return strings.TrimSpace(strings.Join(lines[start:], "\n")), true
}

// atxHeadingText reports whether line is an ATX heading line ("#" through
// "######", followed by a space/tab or nothing else) and, if so, its
// heading text.
func atxHeadingText(line string) (string, bool) {
	trimmed := strings.TrimLeft(strings.TrimRight(line, "\r"), " \t")
	if !strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	if i == 0 || i > 6 {
		return "", false
	}
	rest := trimmed[i:]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", false // e.g. "#foo" is not a heading
	}
	text := strings.TrimSpace(rest)
	return text, text != ""
}

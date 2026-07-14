package render

// The diagram-tier disclosure seam (spec/illustrative-class dc-1/dc-2):
// every mermaid render this package emits is wrapped, server-side and at
// this ONE seam, in a deterministic badged figure disclosing its
// verification tier — so the dex, the workbench corpus page, the placard
// body dialog, and the reference peek all inherit the badge without a
// second markdown implementation and without any client-side badge
// computation. The tier is decidable from the artifact bytes alone
// (dc-2, co-1): a fenced body figure is illustrative BY LOCATION, a
// diagram-kind artifact without class: proposal is illustrative BY
// CLASS, and a class: proposal carries the verification extractor's own
// grammar-coverage verdict (full/partial — spec/diagram-proposals dc-3's
// vocabulary, computed by internal/diagramverify and consumed here,
// never reimplemented). No LLM, no clock, no randomness, no store
// lookup: same bytes in, same bytes out.

import (
	"fmt"
	"html"

	"github.com/jyang234/verdi/internal/diagramverify"
)

// illustrativeBadgeText is the dc-1 figcaption chip, verbatim.
const illustrativeBadgeText = "illustrative · not deterministically verifiable"

// mermaidPre wraps mermaid diagram source in the `<pre class="mermaid">`
// element the vendored client-side mermaid.js turns into an SVG diagram.
// The source is HTML-escaped because mermaid reads the element's
// textContent — escaping is the correct (and only) transform, so `-->`
// survives as diagram syntax rather than becoming an HTML entity the
// diagram engine never sees.
func mermaidPre(source string) string {
	return fmt.Sprintf("<pre class=\"mermaid\">%s</pre>", html.EscapeString(source))
}

// illustrativeFigure is the dc-1 badged figure: the mermaid pre wrapped in
// a figure carrying data-diagram-tier="illustrative" (the machine-readable
// marker) and the visible figcaption badge chip. Static deterministic
// markup — the badge is a pure function of source (co-1).
func illustrativeFigure(source string) string {
	return `<figure class="diagram-figure" data-diagram-tier="illustrative">` +
		mermaidPre(source) +
		`<figcaption class="diagram-tier-badge">` + illustrativeBadgeText + `</figcaption>` +
		"</figure>\n"
}

// proposalFigure renders a class: proposal diagram body: the same figure
// grammar, but wearing the verification extractor's own grammar-coverage
// tier (full/partial — spec/illustrative-class ac-3: "a verified
// proposal's surfaces carry the extractor-computed tier instead") and
// NEVER the illustrative badge (ac-2's negative case: a false
// "illustrative" on a proposal would be the blending lie inverted).
//
// The tier is diagramverify.Parse's whole-artifact Coverage verdict over
// the source alone (an empty truth namespace): full/partial is defined by
// the declared grammar (spec/diagram-proposals dc-3 — "within the
// generator's vocabulary" / "beyond it"), a pure function of the artifact
// bytes (co-1). Truth regeneration (flowmap) never runs here — that is
// the alignment verdict's job, not a page render's.
func proposalFigure(source string) string {
	tier := string(diagramverify.Parse(source, nil).Coverage)
	return `<figure class="diagram-figure" data-diagram-tier="` + tier + `">` +
		mermaidPre(source) +
		`<figcaption class="diagram-tier-badge diagram-tier-badge--proposal">proposal · ` + tier + ` coverage</figcaption>` +
		"</figure>\n"
}

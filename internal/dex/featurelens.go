// The feature lens' dex edition (V1-P8; 05 §Lenses feature lens,
// §Verdi-dex IA): the frozen stub plan ALWAYS paired with the computed
// live `implements` mapping under the explicit "acceptance-time plan;
// current mapping computed below" banner — never the frozen stubs alone.
// The live mapping is the index's computed backlink inversion
// (03 §The feature fold: "the authoritative AC→story mapping is computed
// … the set of story specs whose implements edges name the feature AC"),
// the same source cmd/verdi's feature matrix reads — read-only here,
// never editable, never a second source of truth.
package dex

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/OWNER/verdi/internal/index"
)

// featureLensHTML renders the paired stub-plan/live-mapping section for a
// round-four feature page, or "" for every other page. A feature with no
// declared stubs still gets the full paired section — an honestly empty
// plan next to the computed mapping (mirroring `verdi matrix`'s "(none
// declared)"), so the reader always sees plan and reality side by side.
func featureLensHTML(ix *index.Index, known map[string]bool, p *artifactPage) template.HTML {
	if !isRoundFourFeaturePage(p) {
		return ""
	}

	var b strings.Builder
	b.WriteString("<section class=\"feature-lens\">\n")
	b.WriteString("<h2>Stories</h2>\n")
	b.WriteString(`<div class="acceptance-plan-banner" data-testid="acceptance-plan-banner"><span class="temporal-dot" aria-hidden="true"></span>acceptance-time plan; current mapping computed below</div>` + "\n")

	// The frozen plan: one entry per declared stub, in frontmatter order
	// (the acceptance-time record's own order).
	b.WriteString(`<div class="stub-plan" data-testid="stub-plan">` + "\n")
	b.WriteString("<h3>Planned stubs <span class=\"lens-note\">frozen at acceptance</span></h3>\n")
	if len(p.Meta.Stubs) == 0 {
		b.WriteString(`<p class="empty">No stubs declared.</p>` + "\n")
	} else {
		b.WriteString(`<ul class="stub-list">` + "\n")
		for _, stub := range p.Meta.Stubs {
			slug := template.HTMLEscapeString(stub.Slug)
			b.WriteString(`<li data-testid="stub-` + slug + `"><code>` + slug + `</code> <span class="stub-acs">&rarr; ` + template.HTMLEscapeString(strings.Join(stub.AcceptanceCriteria, ", ")) + `</span></li>` + "\n")
		}
		b.WriteString("</ul>\n")
	}
	b.WriteString("</div>\n")

	// The computed live mapping: per feature AC, the implementing stories
	// discovered from the index's backlink inversion. The feature is
	// downward-blind (02 §Link taxonomy) — this table is computed, never
	// declared.
	b.WriteString(`<div class="live-mapping" data-testid="live-mapping">` + "\n")
	b.WriteString("<h3>Current mapping <span class=\"lens-note\">computed from implements edges</span></h3>\n")
	b.WriteString("<table><thead><tr><th>AC</th><th>Text</th><th>Implementing stories</th></tr></thead><tbody>\n")
	for _, ac := range p.Meta.AcceptanceCriteria {
		fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>", template.HTMLEscapeString(ac.ID), template.HTMLEscapeString(ac.Text))
		stories := implementingStoryRefs(ix, p.Entry.Ref, ac.ID)
		if len(stories) == 0 {
			b.WriteString(`<span class="empty">no implementing story</span>`)
		} else {
			for i, ref := range stories {
				if i > 0 {
					b.WriteString(", ")
				}
				if url, ok := resolvableLinkURL(ref, known); ok {
					b.WriteString(`<a href="` + template.HTMLEscapeString(url) + `">` + template.HTMLEscapeString(ref) + `</a>`)
				} else {
					b.WriteString(template.HTMLEscapeString(ref))
				}
			}
		}
		b.WriteString("</td></tr>\n")
	}
	b.WriteString("</tbody></table>\n</div>\n</section>\n")
	return template.HTML(b.String())
}

// implementingStoryRefs returns the sorted refs of every story whose
// `implements` edge names featureRef's acID fragment — ix.Backlinks
// already sorts by (type, from), so filtering preserves a deterministic
// order.
func implementingStoryRefs(ix *index.Index, featureRef, acID string) []string {
	var refs []string
	for _, bl := range ix.Backlinks(featureRef + "#" + acID) {
		if bl.Type == "implemented-by" {
			refs = append(refs, bl.From)
		}
	}
	return refs
}

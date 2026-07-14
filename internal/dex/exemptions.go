// The per-ADR exemption page (V1-P8; 05 §Lenses: "A per-ADR exemption
// page (the human face of verdi audit) lists an ADR's active exemptions
// and the exempting specs, computed and countable — 'ADR-7: 9 active
// exemptions'"; 03 §Exemption audit: "per-ADR exemption backlinks are
// computed and surfaced — a lens/dex page — over every exempts edge in
// the live corpus that targets that ADR"). The counts come from the SAME
// scan `verdi audit` runs (decisionsweep.ScanExemptions) — no separate
// logic path — and every ADR gets a page, a zero count stated as "0
// active exemptions" rather than omitted (silence is never a pass).
package dex

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/jyang234/verdi/internal/decisionsweep"
)

// exemptionsURL is the per-ADR exemption page's URL, a subpage of the
// ADR's own permalink so it survives nothing (ADRs never move) and reads
// as what it is: the ADR's audit face.
func exemptionsURL(adrName string) string {
	return "/a/adr/" + adrName + "/exemptions/"
}

// exemptionsTitle is the page's countable heading — "ADR-7: 9 active
// exemptions" in this store's naming (the ADR's ref name).
func exemptionsTitle(adrName string, count int) string {
	if count == 1 {
		return fmt.Sprintf("%s: 1 active exemption", adrName)
	}
	return fmt.Sprintf("%s: %d active exemptions", adrName, count)
}

// writeExemptionPages writes one exemption page per ADR page in the
// corpus. The stated count IS the rendered list's length by construction
// — both come from the same ExemptionCount.Sources slice.
func writeExemptionPages(outDir string, stamp buildStamp, pages []*artifactPage, exemptions map[string]*decisionsweep.ExemptionCount, known map[string]bool) error {
	for _, p := range pages {
		if p.Entry.Kind != "adr" {
			continue
		}
		name := strings.TrimPrefix(p.Entry.Ref, "adr/")

		var sources []decisionsweep.ExemptSource
		if c, ok := exemptions[p.Entry.Ref]; ok {
			sources = c.Sources
		}

		var b strings.Builder
		fmt.Fprintf(&b, `<p>Every <code>exempts</code> edge in the live corpus targeting <a href="%s">%s</a>, computed by the same scan as <code>verdi audit</code> (03 §Exemption audit).</p>`+"\n",
			template.HTMLEscapeString(permalinkURL(p.Entry.Ref)), template.HTMLEscapeString(p.Entry.Ref))
		b.WriteString(`<div class="exemption-list" data-testid="exemption-list">` + "\n")
		if len(sources) == 0 {
			b.WriteString(`<p class="empty">No active exemptions.</p>` + "\n")
		} else {
			b.WriteString(`<ul class="entry-list">` + "\n")
			for i, src := range sources {
				fmt.Fprintf(&b, `<li data-testid="exemption-%d">`, i+1)
				if url, ok := resolvableLinkURL(src.SpecRef, known); ok {
					b.WriteString(`<a href="` + template.HTMLEscapeString(url) + `">` + template.HTMLEscapeString(src.SpecRef) + `</a>`)
				} else {
					b.WriteString(template.HTMLEscapeString(src.SpecRef))
				}
				b.WriteString(` <span class="link-type">` + template.HTMLEscapeString(src.DecisionID) + `</span>`)
				if src.Reason != "" {
					b.WriteString(` <span class="sub">` + template.HTMLEscapeString(src.Reason) + `</span>`)
				}
				b.WriteString("</li>\n")
			}
			b.WriteString("</ul>\n")
		}
		b.WriteString("</div>\n")

		data := pageData{
			Title: exemptionsTitle(name, len(sources)),
			Breadcrumb: []breadcrumbEntry{
				{Label: "Home", URL: "/"},
				{Label: "Decisions", URL: "/by-kind/adr/"},
				{Label: p.Entry.Title, URL: permalinkURL(p.Entry.Ref)},
				{Label: "Exemptions", URL: ""},
			},
			Banner:   livingGatedBanner(stamp),
			BodyHTML: template.HTML(b.String()),
		}
		out, err := renderPage(data)
		if err != nil {
			return err
		}
		if err := writeFile(outDir, strings.TrimPrefix(exemptionsURL(name), "/")+"index.html", out); err != nil {
			return err
		}
	}
	return nil
}

// adrExemptionsConnection is the ADR permalink page's link to its
// exemption page — always present, with the honest count, so the audit
// face is one click away even (especially) when the count is zero.
func adrExemptionsConnection(p *artifactPage, exemptions map[string]*decisionsweep.ExemptionCount) *connection {
	if p.Entry.Kind != "adr" {
		return nil
	}
	n := 0
	if c, ok := exemptions[p.Entry.Ref]; ok {
		n = c.Count()
	}
	label := fmt.Sprintf("%d active exemptions", n)
	if n == 1 {
		label = "1 active exemption"
	}
	return &connection{Type: "exemptions", Ref: label, URL: exemptionsURL(strings.TrimPrefix(p.Entry.Ref, "adr/"))}
}

// The index's top-bar controls (spec/index-v2 ac-5; SI-366 (5), (17);
// handoff "Screen 2 — Index", Bar), drawn into the bar's controls slot
// through pageData.BarControls: the view toggle, the Disclosures link
// carrying the disclosures count, the existing import link, and parent
// workbench-redesign dc-9's New feature — a link to the import page with
// its one-line CLI hint. Search (dc-11) and the identity pill (dc-4) are
// not built.
package workbench

import (
	"context"
	stdhtml "html"
	"html/template"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/disclosure"
)

// indexBarControls renders the index's controls slot. words resolves the
// New feature link's class word through the model's display vocabulary.
func indexBarControls(ctx context.Context, root string, extras []disclosure.Disclosure, words classWords) template.HTML {
	var b strings.Builder
	// The view toggle: Pipeline is the view this page draws; List is the
	// placeholder the list view wires (ac-6, lane F7c) — disabled and
	// saying so until then, never a link that leads nowhere.
	b.WriteString(`<div class="topbar-view" data-testid="index-view-toggle" role="group" aria-label="View"><span class="topbar-view-current" aria-current="page">Pipeline</span><button type="button" class="topbar-view-list" disabled title="the list view is not built yet">List</button></div>`)
	writeDisclosuresLink(&b, ctx, root, extras)
	// The mechanical spec importer (spec-import-contract: "The page is
	// discoverable from home before new statements are requested"): the
	// bar precedes the directory's columns, so an existing spec can be
	// brought in before anyone is asked to write statements.
	b.WriteString(`<a class="topbar-import" href="` + routeSpecImportPage + `" data-testid="home-import-link" title="bring an existing Markdown or native spec onto a new design branch as it is: previewed and mapped mechanically, nothing invented, nothing created until you confirm">Import existing spec</a>`)
	b.WriteString(`<span class="topbar-new-feature"><a class="btn-primary home-new-feature" data-testid="home-new-feature" href="` + routeSpecImportPage + `">New ` + stdhtml.EscapeString(words.word("feature")) + `</a>`)
	// vocab:identity — CLI verb name / usage grammar (design.go's designVerbUsage)
	b.WriteString(` <span class="topbar-cli-hint">or <code>verdi design start --kind feature --name &lt;name&gt;</code></span></span>`)
	return template.HTML(b.String()) //nolint:gosec // every fact is escaped above; the rest is this file's own markup
}

// writeDisclosuresLink writes the bar's Disclosures link (spec/disclosures-
// panel's landing-page pointer, now the bar's toggle: spec/index-coverage
// ac-3; SI-295, SI-366 (5)), carrying the disclosures count both as the
// non-visible data-disclosures-count carrier and as its visible number —
// the number of entries /disclosures shows for the same inputs, the same
// extras, one countDisclosures call per render. When the enumeration
// fails the link carries data-disclosures-unproven with the reason and a
// visible unproven mark: never a false "0", never a silent omission.
func writeDisclosuresLink(b *strings.Builder, ctx context.Context, root string, extras []disclosure.Disclosure) {
	n, err := countDisclosures(ctx, root, extras...)
	b.WriteString(`<a class="home-disclosures"`)
	if err != nil {
		b.WriteString(` data-disclosures-unproven="` + stdhtml.EscapeString(err.Error()) + `"`)
	} else {
		b.WriteString(` data-disclosures-count="` + strconv.Itoa(n) + `"`)
	}
	b.WriteString(` href="/disclosures" data-testid="home-disclosures" title="every claim this checkout is currently not proving, in one view">Disclosures`)
	if err != nil {
		b.WriteString(` <span class="dir-unproven" title="` + stdhtml.EscapeString(err.Error()) + `">count unproven</span>`)
	} else {
		b.WriteString(` <span class="count">` + strconv.Itoa(n) + `</span>`)
	}
	b.WriteString(`</a>`)
}

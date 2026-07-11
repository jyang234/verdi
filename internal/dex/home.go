package dex

import "html/template"

// writeHome emits the site's root index page: a simple hub linking to
// every top-level surface (05 §Verdi-dex IA's three axes, minus by-story
// — 05 §v0 thin slice: "dex by-story axis absent" — plus the changelog and
// search).
func writeHome(outDir string, stamp buildStamp) error {
	body := `<ul class="entry-list">
<li><a href="/by-kind/">By kind</a> — specs (active/archive), decisions, diagrams, contracts and APIs</li>
<li><a href="/by-service/">By service</a> — description, boundary contract, obligations, active specs, dependency mini-map</li>
<li><a href="/changelog/">What changed</a> — the git log of .verdi/</li>
<li><a href="/search/">Search</a></li>
</ul>`
	data := pageData{
		Title:      "verdi dex",
		Breadcrumb: []breadcrumbEntry{{Label: "Home", URL: ""}},
		Banner:     livingGatedBanner(stamp),
		BodyHTML:   template.HTML(body),
	}
	out, err := renderPage(data)
	if err != nil {
		return err
	}
	return writeFile(outDir, "index.html", out)
}

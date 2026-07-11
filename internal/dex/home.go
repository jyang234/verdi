package dex

import "html/template"

// writeHome emits the site's root index page: a simple hub linking to
// every top-level surface (05 §Verdi-dex IA's three axes — by-story
// landed with V1-P8 — plus the changelog and search).
func writeHome(outDir string, stamp buildStamp) error {
	body := `<p>The team's record of record: specs, decisions, contracts, and evidence,
rebuilt from main on every merge. Every page states its claim to currency —
<span class="temporal-key temporal-key--living-gated">machine-maintained</span>,
<span class="temporal-key temporal-key--authored-living">human-maintained</span>, or a
<span class="temporal-key temporal-key--frozen">point-in-time record</span> — so a
reader can never mistake an acceptance-time spec for current architecture.</p>
<ul class="entry-list">
<li><a href="/by-kind/">By kind</a> — specs (active/archive), decisions, diagrams, contracts and APIs</li>
<li><a href="/by-service/">By service</a> — description, boundary contract, obligations, active specs, dependency mini-map</li>
<li><a href="/by-story/">By story</a> — the archived quartet: spec, board, rollup, deviation report</li>
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

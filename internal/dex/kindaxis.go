package dex

import (
	"fmt"
	"html/template"
	"sort"
	"strings"
)

// listItem is one row of a by-kind/by-service listing page's entry list.
type listItem struct {
	Title  string
	URL    string
	Status string
	Sub    string // small secondary text (e.g. a service name, a ref)
}

// renderEntryList renders items as dex's standard <ul class="entry-list">.
func renderEntryList(items []listItem) template.HTML {
	if len(items) == 0 {
		return template.HTML(`<p class="empty">Nothing here yet.</p>`)
	}
	var b strings.Builder
	b.WriteString(`<ul class="entry-list">` + "\n")
	for _, it := range items {
		b.WriteString("<li>")
		b.WriteString(`<a href="` + template.HTMLEscapeString(it.URL) + `">` + template.HTMLEscapeString(it.Title) + `</a>`)
		if it.Status != "" {
			b.WriteString(` <span class="badge badge-` + template.HTMLEscapeString(it.Status) + `">` + template.HTMLEscapeString(it.Status) + `</span>`)
		}
		if it.Sub != "" {
			b.WriteString(` <span class="sub">` + template.HTMLEscapeString(it.Sub) + `</span>`)
		}
		b.WriteString("</li>\n")
	}
	b.WriteString("</ul>\n")
	return template.HTML(b.String())
}

// isArchivedSpec reports whether a spec page's source lives under
// specs/archive/ (the directory truth pageBreadcrumb also uses).
func isArchivedSpec(relPath string) bool {
	return strings.Contains(relPath, "/specs/archive/")
}

// writeKindAxis emits the by-kind axis: a hub page plus one listing page
// per kind grouping (05 §Verdi-dex IA: "specs active/archive, decisions,
// diagrams, contracts and APIs" — contracts-and-APIs is written by
// writeContractsAxis in serviceaxis.go, since it draws on discovered
// services rather than committed-zone pages).
func writeKindAxis(outDir string, stamp buildStamp, pages []*artifactPage) error {
	var specsActive, specsArchive, adrs, diagrams, attestations, waivers, conflicts []listItem

	for _, p := range pages {
		item := listItem{Title: p.Entry.Title, URL: permalinkURL(p.Entry.Ref), Status: p.Entry.Status}
		switch p.Entry.Kind {
		case "spec":
			if isArchivedSpec(p.RelPath) {
				specsArchive = append(specsArchive, item)
			} else {
				specsActive = append(specsActive, item)
			}
		case "adr":
			adrs = append(adrs, item)
		case "diagram":
			diagrams = append(diagrams, item)
		case "attestation":
			attestations = append(attestations, item)
		case "waiver":
			waivers = append(waivers, item)
		case "conflict":
			conflicts = append(conflicts, item)
		}
	}

	groups := []struct {
		title, url string
		items      []listItem
	}{
		{"Specs — active", "/by-kind/spec/active/", specsActive},
		{"Specs — archive", "/by-kind/spec/archive/", specsArchive},
		{"Decisions (ADRs)", "/by-kind/adr/", adrs},
		{"Diagrams", "/by-kind/diagram/", diagrams},
		{"Attestations", "/by-kind/attestation/", attestations},
		{"Waivers", "/by-kind/waiver/", waivers},
		{"Conflicts", "/by-kind/conflict/", conflicts},
	}

	for _, g := range groups {
		if err := writeListingPage(outDir, g.url, g.title, []breadcrumbEntry{{Label: "Home", URL: "/"}, {Label: "By kind", URL: "/by-kind/"}, {Label: g.title, URL: ""}}, stamp, g.items); err != nil {
			return err
		}
	}

	var hub []listItem
	for _, g := range groups {
		hub = append(hub, listItem{Title: g.title, URL: g.url, Sub: fmt.Sprintf("%d", len(g.items))})
	}
	return writeListingPage(outDir, "/by-kind/", "By kind", []breadcrumbEntry{{Label: "Home", URL: "/"}, {Label: "By kind", URL: ""}}, stamp, hub)
}

// writeListingPage renders and writes a single by-kind/by-service listing
// page at url ("/by-kind/adr/" etc.) — always living-gated, since it is
// recomputed fresh on every dex build.
func writeListingPage(outDir, url, title string, breadcrumb []breadcrumbEntry, stamp buildStamp, items []listItem) error {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Title != items[j].Title {
			return items[i].Title < items[j].Title
		}
		return items[i].URL < items[j].URL
	})
	data := pageData{
		Title:      title,
		Breadcrumb: breadcrumb,
		Banner:     livingGatedBanner(stamp),
		BodyHTML:   renderEntryList(items),
	}
	out, err := renderPage(data)
	if err != nil {
		return err
	}
	return writeFile(outDir, listingOutPath(url), out)
}

// listingOutPath maps a listing page's URL ("/by-kind/adr/") to its
// output-relative index.html path ("by-kind/adr/index.html").
func listingOutPath(url string) string {
	trimmed := strings.Trim(url, "/")
	if trimmed == "" {
		return "index.html"
	}
	return trimmed + "/index.html"
}

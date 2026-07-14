package dex

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/store"
)

// dependencyMapCap is the anti-hairball law's cap for a service's
// dependency mini-map (05 §Lenses: "every graph view is rooted and capped;
// above the cap, render an index of entry points to root at — never a
// hairball"). Kept small and named so a later phase can tune it without
// hunting for a magic number.
const dependencyMapCap = 12

// boundaryContract is dex's own deliberately partial, loosely-decoded view
// of a service's `.flowmap/boundary-contract.json` (upstream/verdi-computed
// schema `flowmap.boundary/v1`, not a verdi frontmatter schema — dex reads
// it as a guest, same posture as artifact.DecodeFlowmapLoose, so an
// unrecognized field is never an error): enough structure to render a
// readable summary, with the full document always rendered alongside as a
// highlighted JSON fallback so nothing upstream adds next is ever hidden.
type boundaryContract struct {
	Service     string `json:"service"`
	Entrypoints struct {
		HTTP []struct {
			Method string `json:"method"`
			Route  string `json:"route"`
			Tier   int    `json:"tier"`
		} `json:"http"`
	} `json:"entrypoints"`
	BlindSpots []struct {
		Kind   string `json:"kind"`
		Site   string `json:"site"`
		Detail string `json:"detail"`
	} `json:"blind_spots"`
}

// renderBoundaryContract loads path and renders it: a structured summary
// (entrypoints, blind spots) when those fields are present, plus the full
// document, canonicalized and chroma-highlighted, underneath (05 §Verdi-dex
// IA: "boundary contract rendered from the decoded JSON").
func renderBoundaryContract(path string) (template.HTML, error) {
	full, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("dex: reading boundary contract %s: %w", path, err)
	}

	var bc boundaryContract
	// Best-effort structured summary: an upstream schema change that adds
	// or renames fields degrades this to an empty summary, never an error
	// — the full-JSON fallback below still shows everything.
	_ = json.Unmarshal(full, &bc)

	var b strings.Builder
	if len(bc.Entrypoints.HTTP) > 0 {
		b.WriteString("<h3>Entrypoints</h3>\n<table><thead><tr><th>Method</th><th>Route</th><th>Tier</th></tr></thead><tbody>\n")
		for _, e := range bc.Entrypoints.HTTP {
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td></tr>\n",
				template.HTMLEscapeString(e.Method), template.HTMLEscapeString(e.Route), template.HTMLEscapeString(strconv.Itoa(e.Tier)))
		}
		b.WriteString("</tbody></table>\n")
	}
	if len(bc.BlindSpots) > 0 {
		b.WriteString("<h3>Blind spots</h3>\n<table><thead><tr><th>Kind</th><th>Site</th><th>Detail</th></tr></thead><tbody>\n")
		for _, s := range bc.BlindSpots {
			fmt.Fprintf(&b, "<tr><td>%s</td><td><code>%s</code></td><td>%s</td></tr>\n",
				template.HTMLEscapeString(s.Kind), template.HTMLEscapeString(s.Site), template.HTMLEscapeString(s.Detail))
		}
		b.WriteString("</tbody></table>\n")
	}

	pretty, err := json.MarshalIndent(jsonGeneric(full), "", "  ")
	if err != nil {
		return "", fmt.Errorf("dex: pretty-printing boundary contract %s: %w", path, err)
	}
	code, err := highlightCode(string(pretty), "json")
	if err != nil {
		return "", err
	}
	b.WriteString("<h3>Full boundary contract (JSON)</h3>\n")
	b.WriteString(string(code))

	return template.HTML(b.String()), nil
}

// jsonGeneric round-trips data through a generic decode so
// json.MarshalIndent re-formats it (rather than re-embedding data's own,
// potentially non-canonical, byte formatting).
func jsonGeneric(data []byte) interface{} {
	var v interface{}
	_ = json.Unmarshal(data, &v)
	return v
}

// dependencyEdge is one edge of a service's capped dependency mini-map,
// derived from feature specs' `declares.boundaries[]` (from/to/via) that
// name this service.
type dependencyEdge struct {
	From, To, Via string
}

// serviceDependencyMap computes svc's dependency mini-map from every
// feature spec's declared boundaries — the anti-hairball law (05 §Lenses):
// rooted at svc (only edges touching it), capped at dependencyMapCap, and
// when the true edge count exceeds the cap, degraded to a plain index of
// entry points (the distinct neighboring service names) rather than a
// truncated-but-still-dense graph.
func serviceDependencyMap(svc string, pages []*artifactPage) (edges []dependencyEdge, entryPoints []string, capped bool) {
	seen := map[dependencyEdge]bool{}
	var all []dependencyEdge
	for _, p := range pages {
		if p.Entry.Kind != "spec" || p.Meta.Declares == nil {
			continue
		}
		for _, b := range p.Meta.Declares.Boundaries {
			if b.From != svc && b.To != svc {
				continue
			}
			e := dependencyEdge{From: b.From, To: b.To, Via: b.Via}
			if seen[e] {
				continue
			}
			seen[e] = true
			all = append(all, e)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].From != all[j].From {
			return all[i].From < all[j].From
		}
		if all[i].To != all[j].To {
			return all[i].To < all[j].To
		}
		return all[i].Via < all[j].Via
	})

	if len(all) <= dependencyMapCap {
		return all, nil, false
	}

	neighbors := map[string]bool{}
	for _, e := range all {
		if e.From != svc {
			neighbors[e.From] = true
		}
		if e.To != svc {
			neighbors[e.To] = true
		}
	}
	var names []string
	for n := range neighbors {
		names = append(names, n)
	}
	sort.Strings(names)
	return nil, names, true
}

// renderDependencyMap renders serviceDependencyMap's result as dex's HTML
// fragment: an edge table under the cap, or — per the anti-hairball law —
// a plain linked index of entry points above it.
func renderDependencyMap(svc string, edges []dependencyEdge, entryPoints []string, capped bool) template.HTML {
	var b strings.Builder
	if capped {
		b.WriteString("<p>Too many declared boundaries to render as a graph (anti-hairball law, 05 §Lenses). Entry points to root at:</p>\n<ul>\n")
		for _, n := range entryPoints {
			fmt.Fprintf(&b, "<li><a href=\"/by-service/%s/\">%s</a></li>\n", template.HTMLEscapeString(n), template.HTMLEscapeString(n))
		}
		b.WriteString("</ul>\n")
		return template.HTML(b.String())
	}
	if len(edges) == 0 {
		return template.HTML(`<p class="empty">No declared boundaries reference this service yet.</p>`)
	}
	b.WriteString("<table><thead><tr><th>From</th><th>Via</th><th>To</th></tr></thead><tbody>\n")
	for _, e := range edges {
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			serviceCell(e.From, svc), template.HTMLEscapeString(e.Via), serviceCell(e.To, svc))
	}
	b.WriteString("</tbody></table>\n")
	return template.HTML(b.String())
}

// serviceCell renders one dependency-map table cell: a link to the named
// service's by-service page when it is not the rooted service itself
// (self-reference has no page to link to within this table's context).
func serviceCell(name, root string) string {
	if name == root {
		return template.HTMLEscapeString(name)
	}
	return fmt.Sprintf(`<a href="/by-service/%s/">%s</a>`, template.HTMLEscapeString(name), template.HTMLEscapeString(name))
}

// specsImpacting returns, sorted by ref, every feature/component spec page
// whose `impacts:` list names svc — 05 §Verdi-dex IA's "active specs"
// entry on a by-service page.
func specsImpacting(svc string, pages []*artifactPage) []listItem {
	var items []listItem
	for _, p := range pages {
		if p.Entry.Kind != "spec" {
			continue
		}
		for _, imp := range p.Meta.Impacts {
			if imp == svc {
				items = append(items, listItem{Title: p.Entry.Title, URL: permalinkURL(p.Entry.Ref), Status: p.Entry.Status})
				break
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Title < items[j].Title })
	return items
}

// writeServiceAxis emits the by-service axis (05 §Verdi-dex IA: "by
// service (description, boundary contract, OpenAPI and event contracts,
// obligations registry, active specs, ADRs, capped dependency mini-map)"):
// a hub listing every discovered service, plus one page per service.
func writeServiceAxis(outDir string, stamp buildStamp, services []store.Service, pages []*artifactPage) error {
	var hub []listItem
	for _, svc := range services {
		hub = append(hub, listItem{Title: svc.Name, URL: "/by-service/" + svc.Name + "/", Sub: fmt.Sprintf("%d obligation(s)", len(svc.Obligations))})

		body, err := renderServiceBody(svc, pages)
		if err != nil {
			return err
		}
		data := pageData{
			Title:      svc.Name,
			Breadcrumb: []breadcrumbEntry{{Label: "Home", URL: "/"}, {Label: "By service", URL: "/by-service/"}, {Label: svc.Name, URL: ""}},
			Banner:     livingGatedBanner(stamp),
			BodyHTML:   body,
		}
		out, err := renderPage(data)
		if err != nil {
			return err
		}
		if err := writeFile(outDir, "by-service/"+svc.Name+"/index.html", out); err != nil {
			return err
		}
	}

	return writeListingPage(outDir, "/by-service/", "By service", []breadcrumbEntry{{Label: "Home", URL: "/"}, {Label: "By service", URL: ""}}, stamp, hub)
}

// renderServiceBody renders one service's by-service page body: a short
// description, the obligations registry (from .flowmap.yaml's obligation
// names), the boundary contract (if discovered), the OpenAPI doc link (if
// discovered), the specs impacting it, and its capped dependency mini-map.
func renderServiceBody(svc store.Service, pages []*artifactPage) (template.HTML, error) {
	var b strings.Builder

	fmt.Fprintf(&b, "<p>Service root: <code>%s</code>.</p>\n", template.HTMLEscapeString(svc.Dir))

	b.WriteString("<h2>Obligations registry</h2>\n")
	if len(svc.Obligations) == 0 {
		b.WriteString(`<p class="empty">No obligations declared in .flowmap.yaml.</p>` + "\n")
	} else {
		// Promoted presentation: machine-checked guarantees published as
		// documentation are this page's headline — a registry of green
		// stamps, not a bullet list.
		b.WriteString("<p>Machine-checked guarantees, enforced by the upstream toolchain and published here as documentation.</p>\n")
		b.WriteString(`<ul class="obligations-registry">` + "\n")
		for _, o := range svc.Obligations {
			fmt.Fprintf(&b, "<li class=\"obligation\"><a href=\"%s\">%s</a></li>\n", permalinkURL(fmt.Sprintf("svc/%s/obligations/%s", svc.Name, o)), template.HTMLEscapeString(o))
		}
		b.WriteString("</ul>\n")
	}

	b.WriteString("<h2>Boundary contract</h2>\n")
	if svc.BoundaryContractPath != "" {
		fmt.Fprintf(&b, `<p><a href="%s">View the boundary contract</a></p>`+"\n", permalinkURL(fmt.Sprintf("svc/%s/boundary-contract", svc.Name)))
	} else {
		b.WriteString(`<p class="empty">Not discovered for this service.</p>` + "\n")
	}

	b.WriteString("<h2>API</h2>\n")
	if svc.OpenAPIPath != "" {
		fmt.Fprintf(&b, `<p><a href="%s">View the API reference</a></p>`+"\n", permalinkURL(fmt.Sprintf("svc/%s/api", svc.Name)))
	} else {
		b.WriteString(`<p class="empty">No OpenAPI document discovered for this service.</p>` + "\n")
	}

	b.WriteString("<h2>Active specs</h2>\n")
	b.WriteString(string(renderEntryList(specsImpacting(svc.Name, pages))))

	b.WriteString("<h2>Dependency mini-map</h2>\n")
	edges, entryPoints, capped := serviceDependencyMap(svc.Name, pages)
	b.WriteString(string(renderDependencyMap(svc.Name, edges, entryPoints, capped)))

	return template.HTML(b.String()), nil
}

// writeContractsAxis emits the by-kind "contracts and APIs" listing (05
// §Verdi-dex IA's fourth by-kind grouping): every discovered service's
// boundary contract and API permalink in one place, distinct from the
// by-service axis's per-service grouping.
func writeContractsAxis(outDir string, stamp buildStamp, services []store.Service) error {
	var items []listItem
	for _, svc := range services {
		if svc.BoundaryContractPath != "" {
			items = append(items, listItem{Title: svc.Name + " boundary contract", URL: permalinkURL(fmt.Sprintf("svc/%s/boundary-contract", svc.Name)), Sub: svc.Name})
		}
		if svc.OpenAPIPath != "" {
			items = append(items, listItem{Title: svc.Name + " API", URL: permalinkURL(fmt.Sprintf("svc/%s/api", svc.Name)), Sub: svc.Name})
		}
	}
	return writeListingPage(outDir, "/by-kind/contracts/", "Contracts and APIs", []breadcrumbEntry{{Label: "Home", URL: "/"}, {Label: "By kind", URL: "/by-kind/"}, {Label: "Contracts and APIs", URL: ""}}, stamp, items)
}

// writeExternalPages emits one permalink page per index-minted external
// (svc/...) ref (05 §Verdi-dex mechanics: permalinks "+ /a/svc/... for
// external refs"). API pages additionally get a build-emitted openapi.json
// data file alongside, which openapi-renderer.js's script tag reads.
func writeExternalPages(outDir string, stamp buildStamp, ix *index.Index, known map[string]bool, services []store.Service) error {
	byName := make(map[string]store.Service, len(services))
	for _, s := range services {
		byName[s.Name] = s
	}

	for _, e := range ix.All() {
		if e.Kind != "external" {
			continue
		}
		if err := writeExternalPage(outDir, stamp, ix, known, e, byName); err != nil {
			return err
		}
	}
	return nil
}

func writeExternalPage(outDir string, stamp buildStamp, ix *index.Index, known map[string]bool, e *index.Entry, byName map[string]store.Service) error {
	parts := strings.Split(e.Ref, "/")
	if len(parts) < 3 || parts[0] != "svc" {
		return fmt.Errorf("dex: unexpected external ref shape %q", e.Ref)
	}
	svcName, artifactType := parts[1], parts[2]

	var body template.HTML
	var openAPIJSONPath string
	switch artifactType {
	case "boundary-contract":
		rendered, err := renderBoundaryContract(e.Path)
		if err != nil {
			return fmt.Errorf("dex: rendering boundary contract for %s: %w", e.Ref, err)
		}
		body = rendered
	case "obligations":
		body = template.HTML(fmt.Sprintf("<p>A machine-checked obligation declared in <code>%s</code>'s <code>.flowmap.yaml</code>: <strong>%s</strong>. Enforced by the upstream toolchain (`groundwork review`), published here as documentation.</p>", template.HTMLEscapeString(svcName), template.HTMLEscapeString(e.Body)))
	case "api":
		jsonData, err := transcodeOpenAPI(e.Path)
		if err != nil {
			return fmt.Errorf("dex: transcoding OpenAPI doc for %s: %w", e.Ref, err)
		}
		openAPIJSONPath = permalinkURL(e.Ref) + "openapi.json"
		if err := writeFile(outDir, path.Join("a", e.Ref, "openapi.json"), jsonData); err != nil {
			return err
		}
		body = template.HTML("<p>OpenAPI document discovered at <code>" + template.HTMLEscapeString(relOrPath(e.Path, byName[svcName])) + "</code>.</p>")
	default:
		body = template.HTML(fmt.Sprintf("<p>%s</p>", template.HTMLEscapeString(e.Body)))
	}

	data := pageData{
		Title:           e.Title,
		Breadcrumb:      externalBreadcrumb(svcName, e.Title),
		Banner:          livingGatedBanner(stamp),
		BodyHTML:        body,
		Connections:     allConnections(ix, e.Ref, e.Links, known),
		CopyRef:         e.Ref + "@" + stamp.SHA,
		OpenAPIJSONPath: openAPIJSONPath,
	}
	out, err := renderPage(data)
	if err != nil {
		return err
	}
	return writeFile(outDir, permalinkOutPath(e.Ref), out)
}

// relOrPath returns svc.Dir-relative path if svc is known, else path
// unchanged — cosmetic only (the body text showing where the OpenAPI doc
// was discovered).
func relOrPath(path string, svc store.Service) string {
	if svc.Dir == "" || !strings.HasPrefix(path, svc.Dir) {
		return path
	}
	return strings.TrimPrefix(strings.TrimPrefix(path, svc.Dir), "/")
}

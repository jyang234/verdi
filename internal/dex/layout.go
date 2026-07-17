package dex

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// metaRow is one line of a page's metadata card (05 §Verdi-dex page
// anatomy: "metadata card (owners, decided/frozen, supersession links,
// provenance path)").
type metaRow struct {
	Label string
	Value string
}

// connection is one entry in a page's connections panel — either an
// outgoing typed link or a computed backlink (05: "connections panel
// (typed links plus computed backlinks)").
type connection struct {
	Type string // link type, or a computed inverse (e.g. "implemented-by")
	Ref  string // the raw ref string, always shown
	URL  string // "" when ref has no dex page of its own (e.g. a story ref)
	Note string
}

// pageData is the one shape every dex page — artifact permalink, external
// (svc/...) permalink, by-kind/by-service listing, changelog, search, and
// home — renders through, so there is exactly one HTML template to keep
// consistent (breadcrumb, badge, banner, TOC, connections, copy-reference).
type pageData struct {
	Title  string
	Status string // "" suppresses the badge (listing/utility pages)
	// StatusLabel is the model's display word for Status
	// (spec/vocabulary-surfaces ac-2) — the badge's visible text only;
	// empty falls back to Status, and badge-<Status> keeps the bare id
	// (a rename never moves an addressing surface).
	StatusLabel string
	// LadderBadges are a story page's computed ladder-state flags
	// ("spec-stale", "pending-supersession" — 05 §Lenses story lens),
	// rendered as badges beside the status badge; empty everywhere else.
	// A badge renders iff the flag is COMPUTED to stand — an unprovable
	// flag (no forge) is disclosed in the metadata card instead, never
	// silently dropped and never rendered as if proven. Each view carries
	// the flag id (CSS/testid addressing) beside its display label.
	LadderBadges []ladderBadgeView
	Breadcrumb   []breadcrumbEntry
	Banner       string
	// BannerClass is the temporal stamp's class-specific styling hook
	// ("temporal--frozen", "temporal--authored-living",
	// "temporal--living-gated") — presentation only: the banner TEXT is the
	// honest record and never varies with styling. renderPage defaults it
	// to living-gated, the class of every dex-synthesized listing page;
	// artifact pages set it explicitly from their classified temporal class.
	BannerClass string
	MetaRows    []metaRow
	BodyHTML    template.HTML
	Connections []connection
	TOC         []TOCEntry
	CopyRef     string // "" suppresses the copy-reference button
	// CopyRefDisplay is CopyRef with its pin sha visually shortened for the
	// button label. The full pinned form stays in data-copy-ref (what the
	// clipboard receives) and in title/aria-label; only the visible text is
	// truncated. Computed in renderPage.
	CopyRefDisplay string
	// DispositionsHTML is the I-5 dispositions table, pre-rendered to HTML
	// (feature-spec pages only); empty elsewhere.
	DispositionsHTML template.HTML
	// FeatureLensHTML is the feature lens' paired stub-plan/live-mapping
	// section (V1-P8, 05 §Lenses), pre-rendered; empty on every page that
	// is not a round-four feature spec.
	FeatureLensHTML template.HTML
	// OpenAPIJSONPath is set on the one API page per service that has a
	// discovered OpenAPI doc: the site-relative path to the build-emitted
	// openapi.json the openapi-renderer.js script tag reads via its
	// data-openapi-json attribute (05 §Verdi-dex mechanics: "an OpenAPI
	// renderer (script tag per API page ...)").
	OpenAPIJSONPath string
	// HasMermaid gates the mermaid client script + init on this page. It is
	// computed in renderPage from whether BodyHTML actually carries a
	// `<pre class="mermaid">` (a diagram-kind body or an inline fenced
	// ```mermaid block), so the ~10-line script pair is emitted only where a
	// diagram exists rather than on every page. The mermaid.min.js asset stays
	// vendored once regardless — the three-JS-file budget counts served files,
	// not `<script>` tags — so this trims dead tags without touching it.
	HasMermaid bool
}

// pageTemplate is dex's single HTML shell. Asset and cross-page links use
// root-relative absolute paths ("/assets/...", "/a/...", "/by-kind/...")
// — the simplest form that is correct for a Pages site served from its
// forge project's root, which is how both GitLab Pages and GitHub Pages
// serve a project by default (a documented v0 assumption, not something
// this build detects or corrects for a custom sub-path mount).
var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{if eq .Title "verdi dex"}}verdi dex{{else}}{{.Title}} · verdi dex{{end}}</title>
<link rel="stylesheet" href="/assets/style.css">
</head>
<body>
<header class="site-head">
<a class="wordmark" href="/"><span class="leafmark" aria-hidden="true"></span>verdi<span class="wordmark-surface">dex</span></a>
<nav class="site-nav"><a href="/by-kind/">by kind</a> <a href="/by-service/">by service</a> <a href="/by-story/">by story</a> <a href="/changelog/">what changed</a> <a href="/search/">search</a></nav>
</header>
<nav class="breadcrumb">
{{range $i, $c := .Breadcrumb}}{{if $i}} <span class="sep">/</span> {{end}}{{if $c.URL}}<a href="{{$c.URL}}">{{$c.Label}}</a>{{else}}<span class="current">{{$c.Label}}</span>{{end}}{{end}}
</nav>
<header class="page-header">
<h1>{{.Title}}</h1>
{{if .Status}}<span class="badge badge-{{.Status}}">{{if .StatusLabel}}{{.StatusLabel}}{{else}}{{.Status}}{{end}}</span>{{end}}
{{range .LadderBadges}}<span class="badge badge-{{.ID}}" data-testid="badge-{{.ID}}">{{if .Label}}{{.Label}}{{else}}{{.ID}}{{end}}</span>{{end}}
</header>
<div class="temporal-banner {{.BannerClass}}"><span class="temporal-dot" aria-hidden="true"></span>{{.Banner}}</div>
{{if .CopyRef}}<div><button type="button" class="copy-ref" data-copy-ref="{{.CopyRef}}" title="{{.CopyRef}}" aria-label="Copy full reference {{.CopyRef}}">Copy reference <code>{{.CopyRefDisplay}}</code></button></div>{{end}}
<div class="page-body">
<main class="content">
{{if .MetaRows}}<aside class="metadata-card"><dl>
{{range .MetaRows}}<dt>{{.Label}}</dt><dd>{{.Value}}</dd>
{{end}}</dl></aside>{{end}}
{{if .DispositionsHTML}}<section class="dispositions">
<h2>Dispositions</h2>
{{.DispositionsHTML}}
</section>{{end}}
{{.FeatureLensHTML}}
{{.BodyHTML}}
{{if .OpenAPIJSONPath}}<section class="openapi">
<h2>API reference</h2>
<div id="openapi-root"></div>
<script src="/assets/openapi-renderer.js" data-openapi-json="{{.OpenAPIJSONPath}}" defer></script>
</section>{{end}}
</main>
<aside class="side-rail">
{{if .TOC}}<nav class="toc"><h2>On this page</h2><ul>
{{range .TOC}}<li class="toc-level-{{.Level}}"><a href="#{{.ID}}">{{.Text}}</a></li>
{{end}}</ul></nav>{{end}}
{{if .Connections}}<section class="connections"><h2>Connections</h2><ul>
{{range .Connections}}<li><span class="link-type">{{.Type}}</span> {{if .URL}}<a href="{{.URL}}">{{.Ref}}</a>{{else}}<span>{{.Ref}}</span>{{end}}{{if .Note}} — {{.Note}}{{end}}</li>
{{end}}</ul></section>{{end}}
</aside>
</div>
{{if .HasMermaid}}<script src="/assets/mermaid.min.js"></script>
<script>mermaid.initialize({startOnLoad:true,securityLevel:"strict",theme:window.matchMedia&&window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"default"});</script>
{{end}}<script src="/assets/search.js" defer></script>
</body>
</html>
`))

// renderPage executes pageTemplate against data, gating the mermaid client
// on whether the page body actually carries a mermaid block, defaulting the
// temporal stamp's styling hook (living-gated — every dex-synthesized page's
// class), and deriving the copy-reference button's sha-shortened display
// text from the full pinned form.
func renderPage(data pageData) ([]byte, error) {
	data.HasMermaid = strings.Contains(string(data.BodyHTML), `<pre class="mermaid">`)
	if data.BannerClass == "" {
		data.BannerClass = bannerClass(classLivingGated)
	}
	if data.CopyRef != "" && data.CopyRefDisplay == "" {
		data.CopyRefDisplay = displayRef(data.CopyRef)
	}
	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("dex: rendering page %q: %w", data.Title, err)
	}
	return buf.Bytes(), nil
}

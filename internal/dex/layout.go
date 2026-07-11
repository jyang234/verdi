package dex

import (
	"bytes"
	"fmt"
	"html/template"
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
	Title       string
	Status      string // "" suppresses the badge (listing/utility pages)
	Breadcrumb  []breadcrumbEntry
	Banner      string
	MetaRows    []metaRow
	BodyHTML    template.HTML
	Connections []connection
	TOC         []TOCEntry
	CopyRef     string // "" suppresses the copy-reference button
	// DispositionsHTML is the I-5 dispositions table, pre-rendered to HTML
	// (feature-spec pages only); empty elsewhere.
	DispositionsHTML template.HTML
	// OpenAPIJSONPath is set on the one API page per service that has a
	// discovered OpenAPI doc: the site-relative path to the build-emitted
	// openapi.json the openapi-renderer.js script tag reads via its
	// data-openapi-json attribute (05 §Verdi-dex mechanics: "an OpenAPI
	// renderer (script tag per API page ...)").
	OpenAPIJSONPath string
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
<title>{{.Title}} · verdi dex</title>
<link rel="stylesheet" href="/assets/style.css">
</head>
<body>
<nav class="breadcrumb">
{{range $i, $c := .Breadcrumb}}{{if $i}} <span class="sep">/</span> {{end}}{{if $c.URL}}<a href="{{$c.URL}}">{{$c.Label}}</a>{{else}}<span class="current">{{$c.Label}}</span>{{end}}{{end}}
</nav>
<header class="page-header">
<h1>{{.Title}}</h1>
{{if .Status}}<span class="badge badge-{{.Status}}">{{.Status}}</span>{{end}}
</header>
<div class="temporal-banner">{{.Banner}}</div>
{{if .CopyRef}}<button type="button" class="copy-ref" data-copy-ref="{{.CopyRef}}">Copy reference ({{.CopyRef}})</button>{{end}}
<div class="page-body">
<main class="content">
{{if .MetaRows}}<aside class="metadata-card"><dl>
{{range .MetaRows}}<dt>{{.Label}}</dt><dd>{{.Value}}</dd>
{{end}}</dl></aside>{{end}}
{{if .DispositionsHTML}}<section class="dispositions">
<h2>Dispositions</h2>
{{.DispositionsHTML}}
</section>{{end}}
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
<script src="/assets/mermaid.min.js"></script>
<script>mermaid.initialize({startOnLoad:true,securityLevel:"strict",theme:window.matchMedia&&window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"default"});</script>
<script src="/assets/search.js" defer></script>
</body>
</html>
`))

// renderPage executes pageTemplate against data.
func renderPage(data pageData) ([]byte, error) {
	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("dex: rendering page %q: %w", data.Title, err)
	}
	return buf.Bytes(), nil
}

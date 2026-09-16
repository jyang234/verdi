// The workbench's one HTML shell (05 §Workbench: "every page is
// server-rendered ... except one deliberately fat page: the board"), used
// by every read page (corpus, verdict viewer, advisory matrix) so
// navigation and asset links stay consistent. The board page (board.go)
// renders its own shell inline — it carries a client-side script and JSON
// state payload the other pages don't need.
package workbench

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"html/template"
	"strings"

	"github.com/jyang234/verdi/internal/buildinfo"
)

// shellFuncs is the template FuncMap every workbench shell — this shared
// read-page shell and the three inline shells (board.go's v0 board,
// boardspecrender.go's v1 board, boarddiagramrender.go's diagram editor)
// — parses with, so the page footer is defined once and rendered on
// every page (spec/uat-round-1 ac-1: "the workbench renders it in its
// page footer").
var shellFuncs = template.FuncMap{"buildFooter": buildIdentificationFooter}

// buildIdentificationFooter renders the one page footer: the build
// identification line internal/buildinfo.Line() derives — the SAME string
// `verdi version` prints and `verdi serve` logs at startup, so the three
// surfaces can never disagree about which build produced a page (closes
// UAT-003). From a linked-worktree build it honestly reads "verdi (devel)"
// (dc-8); the footer never fabricates a version. Quiet chrome in the
// shell's existing muted note style and monospace code chip.
func buildIdentificationFooter() template.HTML {
	return template.HTML(`<footer class="site-foot" data-testid="build-footer"><p class="ritual-note"><small>Build <code data-testid="build-identification">` + stdhtml.EscapeString(buildinfo.Line()) + `</code></small></p></footer>`)
}

// metaRow is one line of a page's frontmatter card — deliberately the
// same shape as internal/dex's own metaRow (05 §Verdi-dex page anatomy:
// "metadata card"), kept as a small, independent copy here rather than a
// third shared package: the two surfaces' page anatomy differs enough
// (dex has a TOC/connections/OpenAPI sidebar; the workbench's read pages
// are simpler) that forcing one shared template would couple two things
// that only coincidentally look similar today.
type metaRow struct {
	Label string
	Value string
}

// pageData is the shape every non-board workbench page renders through.
type pageData struct {
	Title    string
	Nav      template.HTML // small top-of-page nav links, pre-rendered HTML
	MetaRows []metaRow
	BodyHTML template.HTML
	// DispositionsHTML is the I-5 dispositions table (feature-spec corpus
	// pages only); empty elsewhere.
	DispositionsHTML template.HTML
	// ExtraHTML is any page-specific content appended after the body
	// (links/backlinks panel, verdict diff table, matrix table, ...).
	ExtraHTML template.HTML
	// HasMermaid gates the mermaid client script + init, computed in
	// renderPage from whether BodyHTML carries a `<pre class="mermaid">` (a
	// diagram-kind body or an inline fenced ```mermaid block). Same reasoning
	// as internal/dex: the vendored asset is unaffected — this only drops the
	// script pair from the pages (verdict viewer, matrix, most corpus pages)
	// that never contain a diagram.
	HasMermaid bool
}

var pageTemplate = template.Must(template.New("page").Funcs(shellFuncs).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · verdi workbench</title>
<link rel="stylesheet" href="/assets/style.css">
</head>
<body>
<header class="site-head">
<a class="wordmark" href="/"><span class="leafmark" aria-hidden="true"></span>verdi<span class="wordmark-surface">workbench</span></a>
<nav class="site-nav workbench-nav">{{.Nav}}</nav>
</header>
<header class="page-header"><h1>{{.Title}}</h1></header>
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
{{.ExtraHTML}}
</main>
</div>
{{buildFooter}}
{{if .HasMermaid}}<script src="/assets/mermaid.min.js"></script>
<script>mermaid.initialize({startOnLoad:true,securityLevel:"strict",theme:window.matchMedia&&window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"default"});</script>
{{end}}</body>
</html>
`))

// renderPage executes pageTemplate against data, gating the mermaid client
// on whether the page body actually carries a mermaid block.
func renderPage(data pageData) ([]byte, error) {
	data.HasMermaid = strings.Contains(string(data.BodyHTML), `<pre class="mermaid">`)
	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("workbench: rendering page %q: %w", data.Title, err)
	}
	return buf.Bytes(), nil
}

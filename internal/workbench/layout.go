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
	"html/template"
)

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
}

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · verdi workbench</title>
<link rel="stylesheet" href="/assets/style.css">
</head>
<body>
<nav class="workbench-nav"><a href="/">workbench</a> {{.Nav}}</nav>
<header class="page-header"><h1>{{.Title}}</h1></header>
<div class="page-body">
{{if .MetaRows}}<aside class="metadata-card"><dl>
{{range .MetaRows}}<dt>{{.Label}}</dt><dd>{{.Value}}</dd>
{{end}}</dl></aside>{{end}}
{{if .DispositionsHTML}}<section class="dispositions">
<h2>Dispositions</h2>
{{.DispositionsHTML}}
</section>{{end}}
<main class="content">{{.BodyHTML}}</main>
{{.ExtraHTML}}
</div>
<script src="/assets/mermaid.min.js"></script>
<script>mermaid.initialize({startOnLoad:true,securityLevel:"strict",theme:window.matchMedia&&window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"default"});</script>
</body>
</html>
`))

// renderPage executes pageTemplate against data.
func renderPage(data pageData) ([]byte, error) {
	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("workbench: rendering page %q: %w", data.Title, err)
	}
	return buf.Bytes(), nil
}

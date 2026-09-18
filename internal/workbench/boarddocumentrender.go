package workbench

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// The Document tab's page: a reading surface beside the board's working
// surface. The tab strip stays minimal (index · Board · Document), the
// kind switch and the three controls are plain links and buttons — the
// page is fully usable before its script loads (the kind switch and the
// download are ordinary GETs; only Refresh and Copy need JS) — and the
// rendered document carries the page: its own h1 is the page's h1.
var boardDocumentPageTemplate = template.Must(template.New("boarddocument").Funcs(shellFuncs).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<link rel="stylesheet" href="/assets/style.css">
</head>
<body class="document-page">
<a class="skip-link" href="#document-region">Skip to the document</a>
<header class="site-head">
<a class="wordmark" href="/"><span class="leafmark" aria-hidden="true"></span>verdi<span class="wordmark-surface">workbench</span></a>
<nav class="site-nav workbench-nav"><a href="/">index</a> · <a href="{{.BoardHref}}" data-testid="document-tab-board">Board</a> · <span class="current" aria-current="page" data-testid="document-tab-document">Document</span></nav>
</header>
<header class="document-head">
<p class="eyebrow"><code>{{.Ref}}</code> · document</p>
<nav class="document-kinds" aria-label="Document kind">{{range .Kinds}}{{if .Current}}<span class="current" aria-current="page" data-testid="document-kind-{{.Kind}}">{{.Label}}</span>{{else}}<a href="{{.Href}}" data-testid="document-kind-{{.Kind}}">{{.Label}}</a>{{end}}{{end}}</nav>
<div class="document-actions">
<button type="button" id="document-refresh" data-testid="document-refresh">Refresh</button>
<button type="button" id="document-copy" data-testid="document-copy">Copy Markdown</button>
<a id="document-download" data-testid="document-download" href="{{.DownloadHref}}" download="{{.DownloadName}}">Download {{.DownloadName}}</a>
<span id="document-status" class="document-status" role="status" aria-live="polite"></span>
</div>
</header>
<main id="document-region" class="content document-region" data-revision="{{.Revision}}" data-snapshot-href="{{.SnapshotHref}}" data-testid="document-region">{{.HTML}}</main>
<pre id="document-markdown" class="document-source" hidden>
{{.Markdown}}</pre>
{{buildFooter}}
<script src="/assets/specdocument.js"></script>
</body>
</html>
`))

type documentKindLink struct {
	Kind, Label, Href string
	Current           bool
}

type documentPageData struct {
	Title        string
	Ref          string
	BoardHref    string
	Kinds        []documentKindLink
	DownloadHref string
	DownloadName string
	Revision     string
	SnapshotHref string
	HTML         template.HTML
	Markdown     string
}

// renderBoardDocumentPage builds the Document tab. Every sibling link is
// derived from the request path (already escaped), never from a raw ref,
// so the page works identically under the root and the /b/{branch}
// mounts. The Markdown rides a hidden <pre> as HTML text — entity-escaped
// on the way out and decoded back by the parser — so Copy hands over the
// exact bytes the download and the snapshot carry. The template emits one
// deliberate newline right after the <pre> tag: the HTML parser drops
// exactly one newline there, so without it a Markdown that began with
// "\n" would lose that byte on the way to the clipboard.
func renderBoardDocumentPage(requestPath, name string, snap documentSnapshot) ([]byte, error) {
	boardHref := strings.TrimSuffix(requestPath, "/document")
	kinds := []documentKindLink{
		{Kind: "spec", Label: "Spec"},
		{Kind: "plan", Label: "Plan"},
		{Kind: "tasks", Label: "Tasks"},
	}
	for i := range kinds {
		kinds[i].Href = requestPath + "?kind=" + kinds[i].Kind
		kinds[i].Current = kinds[i].Kind == snap.Kind
	}
	data := documentPageData{
		Title:        name + " — document · verdi workbench",
		Ref:          snap.Ref,
		BoardHref:    boardHref,
		Kinds:        kinds,
		DownloadHref: requestPath + "?format=md&kind=" + snap.Kind,
		DownloadName: name + "-" + snap.Kind + ".md",
		Revision:     snap.Revision,
		SnapshotHref: requestPath + "/snapshot?kind=" + snap.Kind,
		HTML:         template.HTML(snap.HTML), //nolint:gosec // the fragment is our own renderer's output (specdoc.RenderHTML over escaped object text)
		Markdown:     snap.Markdown,
	}
	var buf bytes.Buffer
	if err := boardDocumentPageTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("workbench: rendering document page: %w", err)
	}
	return buf.Bytes(), nil
}

package workbench

import (
	"bytes"
	"context"
	"fmt"
	stdhtml "html"
	"html/template"
	"strings"
)

// The Document tab's page: a reading surface beside the board's working
// surface (spec/document-page-v2; ledger SI-340, SI-343; the redesign
// handoff's Screen 4). The bar carries the Wall and Document switch with
// Copy and Download (SI-340 (13)); the page head keeps the kind switch,
// Refresh, and the status line; then the chrome the page states around
// the shared body, every piece server-rendered from the page's facts so
// it reads before any script runs (ac-4): the temporal stamp — the state
// words and the commit in the authored or accepted token, with a slot the
// browser fills with the refreshed time (dc-2) — the identity card, and
// the contents rail beside the body. The body is the shared renderer's,
// byte for byte (ac-3): the stamp and the card sit above it, the rail
// beside it, and nothing is placed between its own h1 and its sections
// (SI-340 (3)). The id chips are never rendered here: the chip anchors
// ride a JSON script and /assets/documentpage.js draws them (dc-1), so a
// script-less reader never meets a chip that cannot work.
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
{{.TopBar}}
<header class="document-head">
<nav class="document-kinds" aria-label="Document kind">{{range .Kinds}}{{if .Current}}<span class="current" aria-current="page" data-testid="document-kind-{{.Kind}}">{{.Label}}</span>{{else}}<a href="{{.Href}}" data-testid="document-kind-{{.Kind}}">{{.Label}}</a>{{end}}{{end}}</nav>
<div class="document-actions">
<button type="button" id="document-refresh" data-testid="document-refresh">Refresh</button>
<span id="document-status" class="document-status" role="status" aria-live="polite"></span>
</div>
</header>
<div class="document-layout">
<div class="document-grid">
<p id="document-stamp" class="document-stamp document-stamp--{{.Facts.Stamp.State}}" data-testid="document-stamp" data-state="{{.Facts.Stamp.State}}" data-commit="{{.Facts.Stamp.Commit}}"><span class="document-stamp-dot" aria-hidden="true"></span><span class="document-stamp-words" data-testid="document-stamp-words">{{.Facts.Stamp.Words}}</span><span class="document-stamp-sep" aria-hidden="true">·</span><span class="document-stamp-at">commit <code data-testid="document-stamp-commit" title="{{.Facts.Stamp.Commit}}">{{.CommitShort}}</code></span><span id="document-refreshed" class="document-stamp-refreshed" data-testid="document-refreshed"></span></p>
<dl id="document-identity" class="document-identity" data-testid="document-identity">
<dt>ref</dt><dd data-testid="document-identity-ref"><code>{{.Facts.Identity.Ref}}</code></dd>
<dt>class</dt><dd data-testid="document-identity-class"{{if .Facts.Identity.ClassLabel}}>{{.Facts.Identity.ClassLabel}}{{else}} data-state="none">not declared{{end}}</dd>
<dt>branch</dt><dd data-testid="document-identity-branch"{{if .Facts.Identity.Branch.Unproven}} data-state="unproven" title="{{.Facts.Identity.Branch.Unproven}}">{{.Facts.Identity.Branch.Text}}<span class="document-identity-why">{{.Facts.Identity.Branch.Unproven}}</span>{{else if .Facts.Identity.Detached}} data-state="proven" data-detached="true">detached HEAD{{else}} data-state="proven">{{.Facts.Identity.Branch.Text}}{{end}}</dd>
<dt>owners</dt><dd data-testid="document-identity-owners"{{if .Facts.Identity.Owners}}>{{range $i, $o := .Facts.Identity.Owners}}{{if $i}}, {{end}}{{$o}}{{end}}{{else}} data-state="none">none declared{{end}}</dd>
<dt>files</dt><dd data-testid="document-identity-files">{{range .Facts.Identity.Files}}<code>{{.}}</code>{{end}}</dd>
</dl>
<nav class="document-rail" aria-label="Contents" data-testid="document-contents">
<span class="document-contents-label" aria-hidden="true">contents</span>
<ol id="document-contents-list" class="document-contents-list">{{range .Facts.Rail}}<li data-testid="document-contents-{{.ID}}"{{if .Count}} data-count="{{.Count}}"{{end}}><a href="#{{.ID}}"><span class="document-contents-text">{{.Text}}</span>{{if .Count}}<span class="document-contents-count" data-testid="document-contents-{{.ID}}-count">{{.Count}}</span>{{end}}</a></li>{{end}}</ol>
</nav>
<main id="document-region" class="content document-region" data-revision="{{.Revision}}" data-snapshot-href="{{.SnapshotHref}}" data-board-href="{{.BoardHref}}" data-testid="document-region">{{.HTML}}</main>
</div>
</div>
<pre id="document-markdown" class="document-source" hidden>
{{.Markdown}}</pre>
<script type="application/json" id="document-chips">{{.Facts.Chips}}</script>
{{buildFooter}}
<script src="/assets/specdocument.js"></script>
<script src="/assets/documentpage.js"></script>
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
	// Bar is the top bar's facts (SI-323 (2)), which TopBar draws: the
	// spec's title as plain text (the rendered document carries the
	// page's one h1), and the index, Board, and Document links in its nav.
	Bar    barFacts
	TopBar template.HTML
	// Facts is the page's chrome facts (spec/document-page-v2, SI-340):
	// the stamp, identity card, contents rail, and chip anchors, the same
	// value the snapshot carries, for the chrome around the body.
	Facts documentPageFacts
	// CommitShort is the stamp's commit as the eye reads it (shortCommit);
	// the full commit rides the stamp's data attribute and title.
	CommitShort string
}

// shortCommit is the commit as the stamp shows it: its first eight
// characters, the handoff's abbreviation, or the whole value when it is
// no longer than that.
func shortCommit(commit string) string {
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
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
func renderBoardDocumentPage(ctx context.Context, requestPath, name string, snap documentSnapshot, bar barFacts) ([]byte, error) {
	data := documentPageView(requestPath, name, snap, bar)
	esc := stdhtml.EscapeString
	// The Wall and Document switch, in the bar's controls slot (dc-3), with
	// the wall's own two labels, then Copy and Download (SI-340 (13)): the
	// handoff's bar, the ids the page's. The download's file name rides
	// its title and its download attribute.
	controls := `<nav class="topbar-tabs" aria-label="Wall or Document"><a href="` + esc(data.BoardHref) + `" data-testid="document-tab-board">Wall</a><span class="current" aria-current="page" data-testid="document-tab-document">Document</span></nav>` +
		`<button type="button" class="document-action" id="document-copy" data-testid="document-copy">Copy Markdown</button>` +
		`<a class="document-action" id="document-download" data-testid="document-download" href="` + esc(data.DownloadHref) + `" download="` + esc(data.DownloadName) + `" title="` + esc(data.DownloadName) + `">Download .md</a>`
	data.TopBar = renderTopBar(&data.Bar, topBarOptions{Nav: `<a href="/">index</a>`, Controls: template.HTML(controls)}) //nolint:gosec // every value in the controls is escaped above; the markup is this renderer's own
	observeBar(ctx, data.Bar)
	var buf bytes.Buffer
	if err := boardDocumentPageTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("workbench: rendering document page: %w", err)
	}
	return buf.Bytes(), nil
}

// documentPageView is the page template's data for snap under requestPath:
// every sibling link derived from the request path, the body and its
// revision as the snapshot carries them, the bar's facts, and the page's
// chrome facts. The top bar's markup is renderBoardDocumentPage's.
func documentPageView(requestPath, name string, snap documentSnapshot, bar barFacts) documentPageData {
	kinds := []documentKindLink{
		{Kind: "spec", Label: "Spec"},
		{Kind: "plan", Label: "Plan"},
		{Kind: "tasks", Label: "Tasks"},
	}
	for i := range kinds {
		kinds[i].Href = requestPath + "?kind=" + kinds[i].Kind
		kinds[i].Current = kinds[i].Kind == snap.Kind
	}
	return documentPageData{
		Title:        name + " — document · verdi workbench",
		Ref:          snap.Ref,
		BoardHref:    strings.TrimSuffix(requestPath, "/document"),
		Kinds:        kinds,
		DownloadHref: requestPath + "?format=md&kind=" + snap.Kind,
		DownloadName: name + "-" + snap.Kind + ".md",
		Revision:     snap.Revision,
		SnapshotHref: requestPath + "/snapshot?kind=" + snap.Kind,
		HTML:         template.HTML(snap.HTML), //nolint:gosec // the fragment is our own renderer's output (specdoc.RenderHTML over escaped object text)
		Markdown:     snap.Markdown,
		Bar:          bar,
		Facts:        snap.Facts,
		CommitShort:  shortCommit(snap.Facts.Stamp.Commit),
	}
}

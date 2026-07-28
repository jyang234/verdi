package workbench

// Server-side rendering for the diagram-proposal editor page
// (spec/board-editor ac-1/ac-5). Like the spec board, the editor region
// has ONE renderer — this file — reused by the full page and by the
// /fragment route; assets/boarddiagram.js drives the live preview
// (client-side mermaid render of the pane text under the one vendored
// asset, dc-3), the autosaving code pane, and the structural-op gestures.
//
// Visual intent (drafting focus): the drawing dominates. Three panels on
// the board's paper vocabulary — the code pane as the draftsman's source
// sheet (mono, quiet), the live preview as the large working sheet where
// render errors are PAINTED (never a blank, never a stale picture), and
// a slim verification rail that informs and never blocks, wearing the
// wall's chip and disclosed-notice voices.

import (
	"bytes"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"html/template"
	"strings"

	"github.com/jyang234/verdi/internal/diagramedit"
)

// diagramClientPayload is the JSON state embedded for boarddiagram.js.
// Nodes/Edges are the server-parsed op model (the gesture layer maps
// rendered SVG elements onto these ids; it never re-parses the source —
// dc-2: no client-side duplicate of the edit logic). The pane text
// itself is NOT duplicated here: the textarea is the state.
type diagramClientPayload struct {
	Name         string             `json:"name"`
	Mode         string             `json:"mode"`
	OpsAvailable bool               `json:"opsAvailable"`
	Nodes        []diagramedit.Node `json:"nodes"`
	Edges        []diagramedit.Edge `json:"edges"`
	Derived      bool               `json:"derived"`
	// ExitHref is the tool view's resolved return target (spec/tool-view-
	// exit dc-2): where Escape navigates. Resolved once, server-side, at
	// render (boarddiagram.go's resolveDiagramExit) — the client never
	// derives or guesses it, only navigates.
	ExitHref string `json:"exitHref"`
}

// diagramModeStampLabels: the editor room's state in words, mirroring the
// board's mode stamp voice.
var diagramModeStampLabels = map[boardModeKind]string{
	// vocab:identity — editor-mode chrome taxonomy (live working copy), not the spec lifecycle state
	modeAuthoring: "authoring · live draft",
	modeReadOnly:  "read-only · sealed record",
}

var diagramEditorPageTemplate = template.Must(template.New("boarddiagram").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Diagram: {{.Name}} · verdi workbench</title>
<link rel="stylesheet" href="/assets/style.css">
</head>
<body class="board-page diagram-editor-page mode-{{.Mode}}">
<header class="site-head">
<a class="wordmark" href="/"><span class="leafmark" aria-hidden="true"></span>verdi<span class="wordmark-surface">workbench</span></a>
<nav class="site-nav workbench-nav"><a href="/">index</a> <a href="/a/diagram/{{.Name}}">artifact</a></nav>
</header>
<header class="page-header board-head">
<a class="diagram-exit-link" data-testid="diagram-exit" href="{{.ExitHref}}">&larr; {{.ExitLabel}}</a>
<h1>{{.Title}}</h1>
<span class="board-mode-tag board-mode-tag--{{.Mode}}">{{.ModeLabel}}</span>
<span class="badge badge-{{.Status}} diagram-status-badge" data-testid="diagram-status-badge">proposal · {{.Status}}</span>
<div id="autosave-status" data-testid="autosave-status" role="status" aria-live="polite"></div>
</header>
<div id="diagram-editor-region">
{{.Region}}
</div>
{{.Dialogs}}
<script>
window.__DIAGRAM__ = {{.StateJSON}};
</script>
<script src="/assets/mermaid.min.js"></script>
<script src="/assets/boarddiagram.js"></script>
</body>
</html>
`))

// renderDiagramEditorPage renders the full editor page.
func renderDiagramEditorPage(v *diagramEditorView) ([]byte, error) {
	available, _, nodes, edges := opsStateOf(v)
	payload := diagramClientPayload{
		Name:         v.Name,
		Mode:         string(v.Mode),
		OpsAvailable: available,
		Nodes:        nodes,
		Edges:        edges,
		Derived:      v.DerivedFrom != nil,
		ExitHref:     v.Exit.Href,
	}
	stateJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("workbench: diagram editor state: %w", err)
	}

	title := v.Title
	if title == "" {
		title = v.Name
	}
	data := struct {
		Name      string
		Title     string
		Status    string
		Mode      string
		ModeLabel string
		ExitHref  string
		ExitLabel string
		Region    template.HTML
		Dialogs   template.HTML
		StateJSON template.JS
	}{
		Name:      v.Name,
		Title:     title,
		Status:    v.Status,
		Mode:      string(v.Mode),
		ModeLabel: diagramModeStampLabels[v.Mode],
		ExitHref:  v.Exit.Href,
		ExitLabel: v.Exit.Label,
		Region:    template.HTML(renderDiagramEditorRegion(v)),
		Dialogs:   template.HTML(renderDiagramEditorDialogs(v)),
		StateJSON: template.JS(stateJSON),
	}
	var buf bytes.Buffer
	if err := diagramEditorPageTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("workbench: rendering diagram editor page: %w", err)
	}
	return buf.Bytes(), nil
}

// renderDiagramEditorRegion renders the editor's three panels — the
// fragment the /fragment route re-serves.
func renderDiagramEditorRegion(v *diagramEditorView) string {
	var b strings.Builder
	esc := stdhtml.EscapeString
	authoring := v.Mode == modeAuthoring

	// Disclosed notices first, in every mode (constitution 2/10): an
	// assumed default branch, and — the ops half of ac-2 — the
	// disclosed-unavailable state for structural operations. The code
	// pane below stays fully live either way.
	b.WriteString(`<div class="board-notices">`)
	if v.GitNotice != "" {
		b.WriteString(`<div class="board-notice" data-testid="board-notice" role="status">` + esc(v.GitNotice) + `</div>`)
	}
	if v.OpsUnavailable != "" {
		b.WriteString(`<div class="board-notice" data-testid="ops-unavailable" role="status">structural operations are unavailable on this source: ` + esc(v.OpsUnavailable) + ` Type in the code pane as usual.</div>`)
	}
	b.WriteString(`</div>`)

	b.WriteString(`<div class="diagram-editor" data-testid="diagram-editor" data-diagram="` + esc(v.Name) + `" data-editor-mode="` + esc(string(v.Mode)) + `">`)

	// -- The code pane: the draftsman's source sheet. ------------------
	b.WriteString(`<section class="diagram-code" aria-label="Mermaid source">`)
	b.WriteString(`<span class="panel-tab" aria-hidden="true">mermaid source</span>`)
	readonly := ""
	if !authoring {
		readonly = ` readonly`
	}
	// The single "\n" after the opening tag is HTML's own dropped leading
	// newline, so a body starting with real content round-trips exactly.
	b.WriteString(`<textarea id="diagram-source" data-testid="diagram-source" aria-label="Mermaid source" spellcheck="false" wrap="off"` + readonly + `>` + "\n" + esc(string(v.Body)) + `</textarea>`)
	if authoring {
		b.WriteString(`<p class="ritual-note diagram-code-note">The pane autosaves as you type. Every save stores your bytes exactly &#8212; nothing reflows or normalizes the text.</p>`)
	}
	b.WriteString(`</section>`)

	// -- The stage: live preview + painted error state + before-peek. --
	b.WriteString(`<section class="diagram-stage" aria-label="Live preview">`)
	b.WriteString(`<span class="panel-tab" aria-hidden="true">live preview</span>`)
	if authoring {
		b.WriteString(`<div class="diagram-toolbar">`)
		opsDisabled := ""
		if v.Doc == nil {
			opsDisabled = ` disabled`
		}
		b.WriteString(`<button type="button" id="add-node-btn" data-testid="add-node-btn"` + opsDisabled + `>Add node</button>`)
		if v.Doc != nil {
			b.WriteString(`<span class="diagram-toolbar-hint">click a node, then another, to connect &#8212; or drag between them; click a node to rename or delete it</span>`)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`<div class="diagram-stage-panels">`)
	b.WriteString(`<div class="diagram-preview-wrap">`)
	b.WriteString(`<div id="diagram-preview" class="diagram-preview" data-testid="diagram-preview" aria-live="polite"></div>`)
	// The render-error state is PAINTED here, in the preview area (ac-1):
	// it replaces the picture — never beside a silently retained one.
	b.WriteString(`<div id="diagram-render-error" class="diagram-render-error" data-testid="diagram-render-error" role="alert" hidden>`)
	b.WriteString(`<span class="diagram-render-error-tag">render error</span>`)
	b.WriteString(`<pre class="diagram-render-error-msg" id="diagram-render-error-msg"></pre>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`) // diagram-preview-wrap
	if v.DerivedFrom != nil {
		// The before-peek panel (ac-4): the pinned base beside the working
		// preview, read-only. Hidden until peeked; a digest mismatch paints
		// the disclosed failure in the same panel.
		b.WriteString(`<div id="diagram-peek" class="diagram-peek" data-testid="peek-panel" hidden>`)
		b.WriteString(`<span class="panel-tab panel-tab--peek" aria-hidden="true">pinned base &#183; read-only</span>`)
		b.WriteString(`<div id="diagram-peek-preview" class="diagram-preview diagram-preview--peek"></div>`)
		b.WriteString(`<div id="diagram-peek-failure" class="diagram-render-error" data-testid="peek-failure" role="alert" hidden>`)
		b.WriteString(`<span class="diagram-render-error-tag">base does not verify</span>`)
		b.WriteString(`<pre class="diagram-render-error-msg" id="diagram-peek-failure-msg"></pre>`)
		b.WriteString(`</div>`)
		// vocab:identity — non-vocabulary homograph: dismiss-this-panel UI button/aria-label plus CSS class/id fragments (identity)
		b.WriteString(`<button type="button" id="peek-close-btn" class="diagram-peek-close" aria-label="Close before-peek">close</button>`)
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`) // diagram-stage-panels
	b.WriteString(`</section>`)

	// -- The slim verification rail (ac-5/dc-4): consumes, never blocks.
	b.WriteString(`<aside class="diagram-rail" data-testid="verification-rail" aria-label="Verification">`)
	b.WriteString(`<section class="rail-verification"><h2>Verification</h2>`)
	switch {
	case v.Verification != nil:
		writeDiagramVerification(&b, v.Verification)
	default:
		b.WriteString(`<div class="rail-unavailable" data-testid="verification-unavailable" role="status">`)
		b.WriteString(`<span class="rail-unavailable-tag">disclosed</span> verification unavailable: ` + esc(v.VerificationUnavailable))
		b.WriteString(`</div>`)
	}
	b.WriteString(`<p class="ritual-note rail-note">Verification informs scrutiny &#8212; it never blocks an edit or a save.</p>`)
	b.WriteString(`</section>`)

	if v.DerivedFrom != nil {
		b.WriteString(`<section class="rail-provenance" data-testid="rail-provenance"><h2>Derived proposal</h2>`)
		b.WriteString(`<p class="rail-provenance-ref">base <code>` + esc(v.DerivedFrom.Ref) + `</code></p>`)
		b.WriteString(`<div class="rail-provenance-actions">`)
		b.WriteString(`<button type="button" id="peek-btn" data-testid="peek-btn" title="render the digest-verified pinned base beside the working preview, read-only">Before-peek</button>`)
		if authoring {
			b.WriteString(`<button type="button" id="reset-btn" data-testid="reset-btn" title="replace the working source with the digest-verified base, byte-for-byte, through the ordinary save">Reset to base</button>`)
		}
		b.WriteString(`</div>`)
		b.WriteString(`<p class="ritual-note rail-note">Both are pure functions of the pinned provenance &#8212; a base that does not verify against its digest is refused, and nothing is written.</p>`)
		b.WriteString(`</section>`)
	}
	b.WriteString(`</aside>`)

	b.WriteString(`</div>`) // diagram-editor
	return b.String()
}

// diagramFindingLabels: each finding kind in the wall's disclosure voice.
var diagramFindingLabels = map[string]string{
	"exists":       "exists in truth",
	"proposed-new": "proposed &#183; new",
	"contradicted": "contradicted",
	"stale-base":   "stale base",
}

// writeDiagramVerification renders the extractor's report VERBATIM
// (ac-5): the tier as given, each finding with its kind as given — the
// rail computes nothing.
func writeDiagramVerification(b *strings.Builder, r *DiagramVerification) {
	esc := stdhtml.EscapeString
	b.WriteString(`<div class="rail-tier-row">coverage <span class="rail-tier rail-tier--` + esc(r.Tier) + `" data-testid="verification-tier" data-tier="` + esc(r.Tier) + `">` + esc(r.Tier) + `</span></div>`)
	if len(r.Findings) == 0 {
		b.WriteString(`<p class="empty">No per-element findings in this report.</p>`)
		return
	}
	b.WriteString(`<ul class="rail-findings">`)
	for _, f := range r.Findings {
		b.WriteString(`<li class="rail-finding" data-testid="finding-` + esc(f.Identity) + `" data-finding-kind="` + esc(f.Kind) + `">`)
		b.WriteString(`<code class="rail-finding-id">` + esc(f.Identity) + `</code>`)
		label := diagramFindingLabels[f.Kind]
		if label == "" {
			// Unreachable through the validated seam; render the literal
			// kind rather than an empty chip if it ever is.
			label = esc(f.Kind)
		}
		b.WriteString(`<span class="rail-finding-kind rail-finding-kind--` + esc(f.Kind) + `">` + label + `</span>`)
		if f.Witness != "" {
			// A CANDIDATE witness only (verification-extractor dc-4).
			b.WriteString(`<span class="rail-finding-witness" title="candidate witness commit — a pickaxe hit, never a verified cause">candidate witness <code>` + esc(f.Witness) + `</code></span>`)
		}
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ul>`)
}

// renderDiagramEditorDialogs renders the page-level dialogs: authoring
// only (a read-only editor is a document), matching the board's posture.
func renderDiagramEditorDialogs(v *diagramEditorView) string {
	if v.Mode != modeAuthoring {
		return ""
	}
	var b strings.Builder
	b.WriteString(`
<div class="modal-backdrop" id="modal-backdrop" hidden></div>
<div role="dialog" aria-label="Add node" class="board-dialog" id="add-node-dialog" hidden>
<h2>Add node</h2>
<p class="ritual-note">Appends one <code>n&lt;k&gt;["label"]</code> line to the source &#8212; the renderer places it; nothing stores a position.</p>
<div class="field"><label for="add-node-label">Label</label><input id="add-node-label" autocomplete="off"></div>
<div class="dialog-actions"><button type="button" id="add-node-ok">Add</button>
<button type="button" id="add-node-cancel">Cancel</button></div>
</div>`)
	if v.DerivedFrom != nil {
		b.WriteString(`
<div role="alertdialog" aria-label="Reset to base" class="board-dialog confirm" id="reset-confirm" hidden aria-describedby="reset-confirm-consequence">
<h2>Reset to base</h2>
<p id="reset-confirm-consequence" class="ritual-note">Replaces the working source with the digest-verified pinned base, byte-for-byte, through the ordinary save. Your delta on this proposal is discarded.</p>
<div class="dialog-actions"><button type="button" id="reset-confirm-ok">Reset</button>
<button type="button" id="reset-confirm-cancel">Cancel</button></div>
</div>`)
	}
	return b.String()
}

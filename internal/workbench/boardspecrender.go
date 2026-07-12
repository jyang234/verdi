package workbench

// Server-side rendering for the v1 board page. The board region (canvas
// + side rail) has ONE renderer — this file — reused by the full page
// and by the post-mutation fragment; assets/boardspec.js only positions
// yarn, drives dialogs, and swaps this fragment back in.

import (
	"bytes"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"html/template"
	"strconv"
	"strings"

	"github.com/OWNER/verdi/internal/boardlayout"
)

// boardClientPayload is the JSON state embedded for boardspec.js: the
// picker's legality table and consequence labels (mirrors of the Go
// tables — the single source of truth stays in edgetypes.go), plus the
// git state the dialogs need. Board content itself is NOT duplicated
// here: the DOM is the state.
type boardClientPayload struct {
	Spec         string              `json:"spec"`
	Mode         string              `json:"mode"`
	Git          *boardGitState      `json:"git"`
	Legal        map[string][]string `json:"legal"`
	Consequences map[string]string   `json:"consequences"`
	Gate         []string            `json:"gate"`
}

// legalPairTable flattens legalEdgeTypes over every source/target kind
// the board can present, keyed "source|target".
func legalPairTable() map[string][]string {
	sources := []string{
		string(boardlayout.ZoneAC), string(boardlayout.ZoneConstraint),
		string(boardlayout.ZoneDecision), string(boardlayout.ZoneOpenQuestion),
	}
	targets := append([]string{"adr", "spec", "spec-fragment", "diagram"}, sources...)
	table := map[string][]string{}
	for _, s := range sources {
		for _, t := range targets {
			if types := legalEdgeTypes(s, t); len(types) > 0 {
				table[s+"|"+t] = types
			}
		}
	}
	return table
}

var boardSpecPageTemplate = template.Must(template.New("boardspec").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Board: {{.Name}} · verdi workbench</title>
<link rel="stylesheet" href="/assets/style.css">
</head>
<body class="board-page boardv2-page">
<header class="site-head">
<a class="wordmark" href="/"><span class="leafmark" aria-hidden="true"></span>verdi<span class="wordmark-surface">workbench</span></a>
<nav class="site-nav workbench-nav"><a href="/">index</a></nav>
</header>
<header class="page-header board-head">
<h1>{{.Title}}</h1>
<span class="board-mode-tag">{{.Mode}}</span>
<div id="autosave-status" data-testid="autosave-status" role="status" aria-live="polite"></div>
</header>
<div id="boardv2-region">
{{.Region}}
</div>
{{.Dialogs}}
<script>
window.__BOARDV2__ = {{.StateJSON}};
</script>
<script src="/assets/boardspec.js"></script>
</body>
</html>
`))

// renderBoardSpecPage renders the full board page.
func renderBoardSpecPage(p *BoardProjection, git *boardGitState) ([]byte, error) {
	payload := boardClientPayload{
		Spec:         p.Spec,
		Mode:         string(p.Mode),
		Git:          git,
		Legal:        legalPairTable(),
		Consequences: consequenceLabels,
		Gate:         []string{"supersedes", "exempts"},
	}
	stateJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("workbench: board state: %w", err)
	}

	data := struct {
		Name      string
		Title     string
		Mode      string
		Region    template.HTML
		Dialogs   template.HTML
		StateJSON template.JS
	}{
		Name:      p.Spec,
		Title:     p.Title,
		Mode:      string(p.Mode),
		Region:    template.HTML(renderBoardRegion(p, git)),
		Dialogs:   template.HTML(renderBoardDialogs(p.Mode)),
		StateJSON: template.JS(stateJSON),
	}
	var buf bytes.Buffer
	if err := boardSpecPageTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("workbench: rendering board page: %w", err)
	}
	return buf.Bytes(), nil
}

// renderBoardRegion renders the placards, canvas, and side rail — the
// fragment swapped in after every mutation.
func renderBoardRegion(p *BoardProjection, git *boardGitState) string {
	var b strings.Builder
	esc := stdhtml.EscapeString
	authoring := p.Mode == modeAuthoring

	// Disclosed-unavailable notices (I-1(b)/I-2/M-4): a configured-but-
	// unreachable review feed, or an assumed default branch. Rendered
	// FIRST, in every mode, and included in the post-mutation fragment
	// (renderBoardRegion feeds both page and fragment) so the board never
	// renders as if a skipped input were simply absent (constitution 2/10).
	if len(p.Notices) > 0 {
		b.WriteString(`<div class="board-notices">`)
		for _, n := range p.Notices {
			b.WriteString(`<div class="board-notice" data-testid="board-notice" role="status">` + esc(n) + `</div>`)
		}
		b.WriteString(`</div>`)
	}

	// Attribute placards (element taxonomy row 1): the spec's problem
	// and outcome, pinned across the top like the two index cards a
	// murder board starts from.
	b.WriteString(`<header class="board-placards">`)
	if p.Problem != "" {
		b.WriteString(`<div class="placard" data-testid="placard-problem"><span class="placard-tag">problem</span><p>` + esc(p.Problem) + `</p></div>`)
	}
	if p.Outcome != "" {
		b.WriteString(`<div class="placard" data-testid="placard-outcome"><span class="placard-tag">outcome</span><p>` + esc(p.Outcome) + `</p></div>`)
	}
	b.WriteString(`</header>`)

	b.WriteString(`<div class="board-layout">`)
	// The canvas is sized to its content plus a working margin — a pure
	// function of the projection's positions (deterministic), so a sparse
	// board is a shallow board, not a fixed void.
	b.WriteString(`<div id="board-canvas" class="board-canvas boardv2-canvas" data-testid="board" data-board-mode="` + esc(string(p.Mode)) + `" data-spec="` + esc(p.Spec) + `" style="min-height:` + px(canvasMinHeight(p)) + `">`)

	// Object cards. The header is a one-line lockup — kind label left,
	// id right — and the clamped text carries its full form in title
	// (the uniform-footprint card never grows past its index-card size).
	for _, c := range p.Cards {
		b.WriteString(`<div class="objcard objcard--` + esc(c.Kind) + `" data-testid="card-` + esc(c.ID) + `" data-id="` + esc(c.ID) + `" data-object-kind="` + esc(c.Kind) + `" style="left:` + px(c.X) + `;top:` + px(c.Y) + `">`)
		b.WriteString(`<span class="card-kind"><span class="card-kind-label">` + esc(strings.ReplaceAll(c.Kind, "-", " ")) + `</span><span class="card-kind-id">` + esc(c.ID) + `</span></span>`)
		b.WriteString(`<p class="card-text" title="` + esc(c.Text) + `">` + esc(c.Text) + `</p>`)
		if authoring {
			b.WriteString(`<button type="button" class="yarn-handle" data-testid="yarn-handle-` + esc(c.ID) + `" aria-label="Draw yarn from ` + esc(c.ID) + `" title="drag to another card to string yarn"></button>`)
		}
		for _, rs := range c.Anchored {
			writeReviewSticky(&b, rs)
		}
		b.WriteString(`</div>`)
	}

	// Reference cards: external edge targets, so every declared edge has
	// a visible endpoint.
	for _, rc := range p.RefCards {
		b.WriteString(`<div class="refcard" data-testid="` + esc(refCardTestID(rc.Ref)) + `" data-ref="` + esc(rc.Ref) + `" data-ref-kind="` + esc(refKindOf(rc.Ref)) + `" style="left:` + px(rc.X) + `;top:` + px(rc.Y) + `">`)
		b.WriteString(`<span class="card-kind"><span class="card-kind-label">reference</span><span class="card-kind-id">` + esc(refKindOf(rc.Ref)) + `</span></span>`)
		b.WriteString(`<span class="card-ref" title="` + esc(rc.Ref) + `">` + esc(rc.Ref) + `</span>`)
		b.WriteString(`</div>`)
	}

	// Scratch stickies (the scratch tier, 05 §Workbench).
	for _, s := range p.Stickies {
		b.WriteString(`<div class="sticky sticky--` + stickyTypeClass(s.Type) + `" data-testid="sticky-` + esc(s.ID) + `" data-id="` + esc(s.ID) + `" data-annotation-type="` + esc(s.Type) + `" style="left:` + px(s.X) + `;top:` + px(s.Y) + `">`)
		b.WriteString(`<span class="sticky-type">` + esc(s.Type) + `</span>`)
		b.WriteString(`<p class="sticky-body">` + esc(s.Body) + `</p>`)
		if s.Author != "" {
			b.WriteString(`<span class="sticky-meta">` + esc(s.Author) + `</span>`)
		}
		if authoring {
			b.WriteString(`<button type="button" class="graduate-btn" data-graduate="sticky">Graduate</button>`)
		}
		b.WriteString(`</div>`)
	}

	// Yarn chips: one HTML element per edge carrying the contract's data
	// attributes; boardspec.js lays them on the thread's midpoint and
	// draws the SVG thread itself (pure decoration, no data attributes).
	for _, e := range p.Edges {
		b.WriteString(`<div class="yarn-chip yarn-chip--` + esc(e.Layer) + `" data-edge-type="` + esc(e.Type) + `" data-from="` + esc(e.From) + `" data-to="` + esc(e.To) + `" data-layer="` + esc(e.Layer) + `"`)
		if e.AnnotationID != "" {
			b.WriteString(` data-annotation-id="` + esc(e.AnnotationID) + `"`)
		}
		b.WriteString(`>`)
		b.WriteString(`<span class="yarn-chip-type">` + esc(e.Type) + `</span>`)
		if authoring && e.Layer == "annotation" {
			b.WriteString(`<button type="button" class="graduate-btn" data-graduate="thread">Graduate</button>`)
		}
		b.WriteString(`</div>`)
	}

	b.WriteString(`</div>`) // board-canvas

	// The side rail.
	b.WriteString(`<aside class="board-side">`)
	switch p.Mode {
	case modeAuthoring:
		writeGitPanel(&b, git)
		b.WriteString(`<section class="scratch-panel"><h2>Scratch</h2>` +
			`<p class="ritual-note">Stickies and untyped threads stay in the annotation layer — they never enter the spec until graduated.</p>` +
			`<button type="button" id="add-sticky-btn">Add sticky</button></section>`)
	case modeReview:
		writeInboxTray(&b, p.Tray)
	default:
		b.WriteString(`<section class="scratch-panel"><h2>Read-only</h2><p class="ritual-note">This spec is not on a design branch. Change means supersession (the amendment ladder).</p></section>`)
	}
	b.WriteString(`</aside>`)
	b.WriteString(`</div>`) // board-layout

	return b.String()
}

func writeReviewSticky(b *strings.Builder, rs reviewStickyView) {
	esc := stdhtml.EscapeString
	b.WriteString(`<div class="review-sticky" data-annotation-type="review"`)
	if rs.Anchor != "" {
		b.WriteString(` data-anchor="` + esc(rs.Anchor) + `"`)
	}
	b.WriteString(`>`)
	b.WriteString(`<span class="sticky-type">review`)
	if rs.Resolved {
		b.WriteString(` · resolved`)
	}
	b.WriteString(`</span>`)
	b.WriteString(`<p class="sticky-body">` + esc(rs.Body) + `</p>`)
	if rs.Author != "" {
		b.WriteString(`<span class="sticky-meta">` + esc(rs.Author) + `</span>`)
	}
	b.WriteString(`</div>`)
}

// writeInboxTray renders the review-mode inbox tray: every comment whose
// token is missing or unresolvable — never dropped (05 §Review stickies).
func writeInboxTray(b *strings.Builder, tray []reviewStickyView) {
	b.WriteString(`<section class="inbox-tray" role="region" aria-label="Inbox tray"><h2>Inbox tray</h2>`)
	if len(tray) == 0 {
		b.WriteString(`<p class="empty">Every comment is anchored.</p>`)
	}
	for _, rs := range tray {
		writeReviewSticky(b, rs)
	}
	b.WriteString(`</section>`)
}

// writeGitPanel renders the board-owned git affordance (05 §Workbench:
// commit/push button, persistent uncommitted-changes indicator,
// branch switcher behind the guard).
func writeGitPanel(b *strings.Builder, git *boardGitState) {
	esc := stdhtml.EscapeString
	b.WriteString(`<section class="git-panel"><h2>Working tree</h2>`)
	b.WriteString(`<span class="uncommitted" data-testid="uncommitted-indicator"`)
	if !git.Dirty {
		b.WriteString(` hidden`)
	}
	b.WriteString(`>uncommitted changes</span>`)
	// The page's most consequential action wears primary weight.
	b.WriteString(`<button type="button" id="commit-push-btn" class="btn-primary">Commit &amp; push</button>`)
	b.WriteString(`<div class="branch-row"><span class="branch-label">branch</span>`)
	b.WriteString(`<button type="button" class="branch-switcher" data-testid="branch-switcher" aria-haspopup="menu">` + esc(git.Branch) + `</button></div>`)
	b.WriteString(`<div role="menu" class="branch-menu" id="branch-menu" hidden aria-label="Switch branch">`)
	for _, br := range git.Branches {
		b.WriteString(`<button type="button" role="menuitem" data-branch="` + esc(br) + `">` + esc(br) + `</button>`)
	}
	b.WriteString(`</div></section>`)
}

// renderBoardDialogs renders the page-level dialogs. Only authoring mode
// gets any: review is a mirror, read-only a document (05 §Workbench).
func renderBoardDialogs(mode boardModeKind) string {
	if mode != modeAuthoring {
		return ""
	}
	return `
<div class="modal-backdrop" id="modal-backdrop" hidden></div>
<div role="dialog" aria-label="Edge type" class="board-dialog picker" id="edge-picker" hidden>
<h2>Edge type</h2>
<p class="ritual-note" id="edge-picker-pair"></p>
<div id="edge-picker-items"></div>
</div>
<div role="alertdialog" aria-label="" class="board-dialog confirm" id="edge-confirm" hidden aria-describedby="edge-confirm-consequence">
<h2 id="edge-confirm-title"></h2>
<p id="edge-confirm-consequence" class="ritual-note"></p>
<div class="field" id="edge-confirm-reason-field" hidden><label for="edge-confirm-reason">Reason</label><input id="edge-confirm-reason" autocomplete="off"></div>
<div class="dialog-actions"><button type="button" id="edge-confirm-ok">Confirm</button>
<button type="button" id="edge-confirm-cancel">Cancel</button></div>
</div>
<div role="dialog" aria-label="Commit &amp; push" class="board-dialog" id="commit-dialog" hidden>
<h2>Commit &amp; push</h2>
<p class="ritual-note">Commits the working tree on this design branch and pushes it.</p>
<div class="field"><label for="commit-message">Commit message</label><input id="commit-message" autocomplete="off"></div>
<div class="dialog-actions"><button type="button" id="commit-dialog-ok">Commit</button>
<button type="button" id="commit-dialog-cancel">Cancel</button></div>
</div>
<div role="alertdialog" aria-label="Uncommitted changes" class="board-dialog confirm" id="branch-guard" hidden>
<h2>Uncommitted changes</h2>
<p class="ritual-note">This working tree has uncommitted board work. Switching branches now would carry or lose it — commit first.</p>
<div class="dialog-actions"><button type="button" id="branch-guard-stay">Stay on branch</button></div>
</div>
<div role="menu" class="board-dialog graduate-menu" id="graduate-menu" hidden aria-label="Graduate to">
<button type="button" role="menuitem" data-object-kind="acceptance-criterion">Acceptance criterion</button>
<button type="button" role="menuitem" data-object-kind="constraint">Constraint</button>
<button type="button" role="menuitem" data-object-kind="decision">Decision</button>
<button type="button" role="menuitem" data-object-kind="open-question">Open question</button>
</div>`
}

// refKindOf classifies a ref-card's target kind for the picker's
// legality lookup (data-ref-kind).
func refKindOf(ref string) string {
	return targetKindOf(nil, ref)
}

// canvasMinHeight sizes the canvas to its content plus one empty row of
// working margin (floor: enough for two rows of empty board). A pure
// function of the projection's positions — no viewport, clock, or
// randomness — so the page stays a deterministic render.
func canvasMinHeight(p *BoardProjection) float64 {
	const (
		floor        = 416 // two card rows + margins: an empty board is still a board
		margin       = 176 // one card row of working room below the content
		stickyHeight = 150 // sticky min footprint (est.: stickies grow with text)
	)
	bottom := float64(floor)
	for _, c := range p.Cards {
		if y := c.Y + boardlayout.CardHeight; y > bottom {
			bottom = y
		}
	}
	for _, rc := range p.RefCards {
		if y := rc.Y + boardlayout.RefCardHeight; y > bottom {
			bottom = y
		}
	}
	for _, s := range p.Stickies {
		if y := s.Y + stickyHeight; y > bottom {
			bottom = y
		}
	}
	return bottom + margin
}

func px(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64) + "px"
}

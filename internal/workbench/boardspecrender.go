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
	// Removals are the gate-bearing types' removal consequences — the
	// confirmation ritual mirrors creation (owner UAT round 6, item 3).
	Removals map[string]string `json:"removals"`
	Gate     []string          `json:"gate"`
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

// modeStampLabels is the mode stamp's copy — the room's state in words,
// not just an enum value: authoring is the live wall, review is a
// mirror of someone else's MR, read-only is the sealed record.
var modeStampLabels = map[boardModeKind]string{
	modeAuthoring: "authoring · live wall",
	modeReview:    "review · mirror of the MR",
	modeReadOnly:  "read-only · sealed record",
}

var boardSpecPageTemplate = template.Must(template.New("boardspec").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Board: {{.Name}} · verdi workbench</title>
<link rel="stylesheet" href="/assets/style.css">
</head>
<body class="board-page boardv2-page mode-{{.Mode}}">
<header class="site-head">
<a class="wordmark" href="/"><span class="leafmark" aria-hidden="true"></span>verdi<span class="wordmark-surface">workbench</span></a>
<nav class="site-nav workbench-nav"><a href="/">index</a></nav>
</header>
<header class="page-header board-head">
<h1>{{.Title}}</h1>
<span class="board-mode-tag board-mode-tag--{{.Mode}}">{{.ModeLabel}}</span>
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
		Removals:     removalConsequenceLabels,
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
		ModeLabel string
		Region    template.HTML
		Dialogs   template.HTML
		StateJSON template.JS
	}{
		Name:      p.Spec,
		Title:     p.Title,
		Mode:      string(p.Mode),
		ModeLabel: modeStampLabels[p.Mode],
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

	// The case file (element taxonomy row 1): the spec's problem and
	// outcome as ONE header lockup — the folder a murder board opens
	// with. Problem wears the violated accent, outcome the evidenced
	// one, and the arrow between them is the whole story's arc: the
	// wall below exists to get from the left card to the right one.
	// A spec carrying neither attribute (grandfathered v0 artifacts)
	// gets no header at all — never an empty folder tab.
	if p.Problem != "" || p.Outcome != "" {
		b.WriteString(`<header class="board-placards case-file">`)
		b.WriteString(`<span class="case-tab" aria-hidden="true">case file</span>`)
		if p.Problem != "" {
			b.WriteString(`<div class="placard placard--problem" data-testid="placard-problem"><span class="placard-tag">problem</span><p>` + esc(p.Problem) + `</p></div>`)
		}
		if p.Problem != "" && p.Outcome != "" {
			b.WriteString(`<div class="case-arrow" aria-hidden="true">&#8594;</div>`)
		}
		if p.Outcome != "" {
			b.WriteString(`<div class="placard placard--outcome" data-testid="placard-outcome"><span class="placard-tag">outcome</span><p>` + esc(p.Outcome) + `</p></div>`)
		}
		b.WriteString(`</header>`)
	}

	b.WriteString(`<div class="board-layout">`)
	// The canvas is sized to its content plus a working margin — a pure
	// function of the projection's positions (deterministic), so a sparse
	// board is a shallow board, not a fixed void.
	b.WriteString(`<div id="board-canvas" class="board-canvas boardv2-canvas" data-testid="board" data-board-mode="` + esc(string(p.Mode)) + `" data-spec="` + esc(p.Spec) + `" style="min-height:` + px(canvasMinHeight(p)) + `">`)

	// Zone labels: the filing scheme the zoned layout already uses, made
	// visible — tape strips over each kind's column band. Authoring
	// labels every band (an empty band is an invitation: this is where
	// decisions land); review and read-only label only what the record
	// holds. Pure function of the projection + boardlayout constants.
	writeZoneLabels(&b, p)

	// The empty wall: a board with no pinned facts teaches instead of
	// voiding (authoring) or states its emptiness plainly (elsewhere).
	// Reference cards don't count as facts — the leanest valid story
	// spec already hangs its implements thread, and its wall must still
	// read as the invitation it is.
	if len(p.Cards) == 0 && len(p.Stickies) == 0 {
		b.WriteString(`<div class="board-empty" data-testid="board-empty">`)
		if authoring {
			b.WriteString(`<p class="board-empty-lead">Nothing pinned yet.</p>`)
			b.WriteString(`<p class="board-empty-how">Pin your first fact: <strong>Add sticky</strong> (in the rail), write what you know, and graduate it into the spec when it firms up &#8212; or declare an acceptance criterion in the spec file and it lands here as a card.</p>`)
		} else {
			b.WriteString(`<p class="board-empty-lead">Nothing is declared on this spec yet.</p>`)
		}
		b.WriteString(`</div>`)
	}

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

	// Reference cards: external edge targets (every declared edge has a
	// visible endpoint) and pinned references (the scratch tier's planning
	// material, 05) — the SAME paper, one card per ref; a live pin adds
	// the pushpin marking, the scratch lean, and the drag affordance.
	for _, rc := range p.RefCards {
		cls := "refcard"
		pinAttrs := ""
		label := "reference"
		if rc.Pinned {
			cls += " refcard--pinned"
			pinAttrs = ` data-pin-id="` + esc(rc.PinID) + `"`
			label = "pinned reference"
		}
		b.WriteString(`<div class="` + cls + `" data-testid="` + esc(refCardTestID(rc.Ref)) + `" data-ref="` + esc(rc.Ref) + `" data-ref-kind="` + esc(refKindOf(rc.Ref)) + `"` + pinAttrs + ` style="left:` + px(rc.X) + `;top:` + px(rc.Y) + `">`)
		b.WriteString(`<span class="card-kind"><span class="card-kind-label">` + label + `</span><span class="card-kind-id">` + esc(refKindOf(rc.Ref)) + `</span></span>`)
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
			b.WriteString(`<button type="button" class="delete-btn" data-delete="sticky" aria-label="Delete sticky" title="the sticky dies; the spec is untouched">×</button>`)
		}
		b.WriteString(`</div>`)
	}

	// Yarn chips: one HTML element per edge carrying the contract's data
	// attributes; boardspec.js lays them on the thread's midpoint and
	// draws the SVG thread itself (pure decoration, no data attributes).
	// Authoring affordances (owner UAT round 6, item 3 + the retype
	// directive): annotation chips graduate or die; a spec-layer chip
	// drawn from a decision retypes in place (its type label is the
	// affordance) or is removed — the inverse of drawing it. A
	// document-level chip (From "spec") gets neither: its edge lives in
	// the frontmatter links: block the board cannot edit.
	for _, e := range p.Edges {
		editableSpecEdge := authoring && e.Layer == "spec" && e.From != "spec"
		chipClass := "yarn-chip yarn-chip--" + esc(e.Layer)
		if e.From == "spec" {
			chipClass += " yarn-chip--doc"
		}
		b.WriteString(`<div class="` + chipClass + `" data-edge-type="` + esc(e.Type) + `" data-from="` + esc(e.From) + `" data-to="` + esc(e.To) + `" data-layer="` + esc(e.Layer) + `"`)
		if e.AnnotationID != "" {
			b.WriteString(` data-annotation-id="` + esc(e.AnnotationID) + `"`)
		}
		b.WriteString(`>`)
		if e.From == "spec" {
			// The document is not a card — its thread runs off the top of
			// the wall — so its chip says whose edge this is.
			b.WriteString(`<span class="yarn-chip-doc">this spec</span>`)
		}
		if editableSpecEdge {
			b.WriteString(`<button type="button" class="yarn-chip-type" data-retype aria-label="Change ` + esc(e.Type) + ` edge type" title="change this relationship's type">` + esc(e.Type) + `</button>`)
		} else {
			b.WriteString(`<span class="yarn-chip-type">` + esc(e.Type) + `</span>`)
		}
		if authoring && e.Layer == "annotation" {
			b.WriteString(`<button type="button" class="graduate-btn" data-graduate="thread">Graduate</button>`)
			b.WriteString(`<button type="button" class="delete-btn" data-delete="thread" aria-label="Delete thread" title="the thread dies; the spec is untouched">×</button>`)
		}
		if editableSpecEdge {
			b.WriteString(`<button type="button" class="delete-btn" data-delete="edge" aria-label="Remove ` + esc(e.Type) + ` edge" title="remove this relationship from the spec">×</button>`)
		}
		b.WriteString(`</div>`)
	}

	b.WriteString(`</div>`) // board-canvas

	// The side rail, top-down by consequence: the commit affordance (the
	// page's one write to the record), then the scratch tools, then the
	// reading aids (yarn key), then the learning aid (the four-move
	// guide) — quiet last, discoverable, never front-loaded.
	b.WriteString(`<aside class="board-side">`)
	switch p.Mode {
	case modeAuthoring:
		writeGitPanel(&b, git)
		b.WriteString(`<section class="scratch-panel"><h2>Scratch</h2>` +
			`<p class="ritual-note">Think here first. Stickies and untyped threads stay in the annotation layer &#8212; they never enter the spec until graduated.</p>` +
			`<button type="button" id="add-sticky-btn">Add sticky</button></section>`)
		writeYarnKey(&b, p)
		writeGuide(&b)
	case modeReview:
		b.WriteString(`<section class="mirror-note"><h2>Review mirror</h2>` +
			`<p class="ritual-note">This board mirrors the merge request. Comments that name a card ride on it; everything else lands in the tray below &#8212; nothing is dropped.</p></section>`)
		writeInboxTray(&b, p.Tray)
		writeYarnKey(&b, p)
	default:
		b.WriteString(`<section class="scratch-panel sealed-panel"><h2>Sealed record</h2><p class="ritual-note">This spec is accepted; the wall is its photograph. Change means supersession (the amendment ladder).</p></section>`)
		writeYarnKey(&b, p)
	}
	b.WriteString(`</aside>`)
	b.WriteString(`</div>`) // board-layout

	return b.String()
}

// zoneLabelText names each zone band as a newcomer reads it.
var zoneLabelText = map[boardlayout.ZoneKind]string{
	boardlayout.ZoneAC:           "acceptance criteria",
	boardlayout.ZoneConstraint:   "constraints",
	boardlayout.ZoneDecision:     "decisions",
	boardlayout.ZoneOpenQuestion: "open questions",
	boardlayout.ZoneReference:    "references",
	boardlayout.ZoneScratch:      "scratch",
}

// writeZoneLabels renders the tape strips over the zoned columns, plus
// the scratch lane (where comment/agent-task stickies land).
// Decorative teaching (aria-hidden, pointer-events off in CSS): every
// card already names its own kind for assistive tech.
func writeZoneLabels(b *strings.Builder, p *BoardProjection) {
	occupied := map[boardlayout.ZoneKind]bool{}
	for _, c := range p.Cards {
		occupied[boardlayout.ZoneKind(c.Kind)] = true
	}
	if len(p.RefCards) > 0 {
		occupied[boardlayout.ZoneReference] = true
	}
	// The scratch lane is occupied by geometry: any sticky whose
	// footprint currently sits in the band (a dragged-away sticky stops
	// counting — the label follows the paper, not the paper's history).
	sc := boardlayout.ScratchColumn()
	for _, st := range p.Stickies {
		if st.X < float64(sc.X+sc.Width) && float64(sc.X) < st.X+boardlayout.CardWidth {
			occupied[boardlayout.ZoneScratch] = true
			break
		}
	}
	authoring := p.Mode == modeAuthoring

	b.WriteString(`<div class="zone-labels" aria-hidden="true">`)
	for _, col := range append(boardlayout.ZoneColumns(), sc) {
		if !occupied[col.Kind] && !authoring {
			continue // the record shows what it has; it does not invite
		}
		cls := "zone-label zone-label--" + string(col.Kind)
		if !occupied[col.Kind] {
			cls += " zone-label--empty"
		}
		b.WriteString(`<span class="` + cls + `" data-testid="zone-label-` + string(col.Kind) + `" style="left:` +
			strconv.Itoa(col.X) + `px;width:` + strconv.Itoa(col.Width) + `px">` + zoneLabelText[col.Kind] + `</span>`)
	}
	b.WriteString(`</div>`)
}

// yarnKeyOrder is the legend's canonical order: the minimum path's edge
// first, the gate-bearing amendments late, scratch last.
var yarnKeyOrder = []string{"implements", "resolves", "depends-on", "supersedes", "exempts", "relates"}

// yarnKeyMeanings is one clause per type — what the thread claims, in a
// PM's words (the consequence labels stay the picker's fuller voice).
var yarnKeyMeanings = map[string]string{
	"implements": "this spec delivers it",
	"resolves":   "this spec answers it",
	"depends-on": "needed background",
	"supersedes": "amends it for everyone",
	"exempts":    "this spec is excused from it",
	"relates":    "scratch thread — not in the spec",
}

// writeYarnKey renders the wall's legend: exactly the edge types
// present, in canonical order — a key to this board, never the closed
// enum's vocabulary lesson.
func writeYarnKey(b *strings.Builder, p *BoardProjection) {
	present := map[string]bool{}
	for _, e := range p.Edges {
		present[e.Type] = true
	}
	if len(present) == 0 {
		return
	}
	b.WriteString(`<section class="yarn-key" data-testid="yarn-key"><h2>Yarn on this wall</h2><ul>`)
	for _, t := range yarnKeyOrder {
		if !present[t] {
			continue
		}
		b.WriteString(`<li data-edge-type="` + t + `"><span class="yarn-key-swatch" aria-hidden="true"></span><span class="yarn-key-type">` + t + `</span><span class="yarn-key-what">` + yarnKeyMeanings[t] + `</span></li>`)
	}
	b.WriteString(`</ul></section>`)
}

// writeGuide renders the four-move guide (05 §Workbench "The
// four-concept minimum path": story spec + ACs + implements + commit),
// collapsed by default — the newcomer's whole path in one quiet
// disclosure, everything further learned from the wall itself.
func writeGuide(b *strings.Builder) {
	b.WriteString(`<details class="board-guide" data-testid="board-guide"><summary>New to the wall? Four moves.</summary>` +
		`<ol class="guide-moves">` +
		`<li><strong>Read the case file</strong> &#8212; the problem and outcome placards above the wall are the spec&#8217;s own header.</li>` +
		`<li><strong>Pin acceptance criteria</strong> &#8212; the first column says what must be true. Drag cards anywhere; double-click one to edit its text.</li>` +
		`<li><strong>String yarn</strong> &#8212; drag the pin on a decision card to another card to type a relationship. A thread running off the top edge belongs to the spec document itself (its implements/resolves edges).</li>` +
		`<li><strong>Commit &amp; push</strong> &#8212; the wall autosaves as you work; committing files it on the design branch.</li>` +
		`</ol>` +
		`<p class="guide-more">Everything else &#8212; stickies, graduation, exemptions &#8212; is on the wall when you need it.</p>` +
		`</details>`)
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
<div class="dialog-actions"><button type="button" id="edge-picker-cancel">Cancel</button></div>
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
<button type="button" id="graduate-menu-cancel">Cancel</button>
</div>
<!-- The supply toolbox (owner directive): the wall's box of pins — one
     quiet tab at the screen's lower-left, one click to the picker, no
     residue when it closes. Authoring only, like every write affordance. -->
<div class="pin-toolbox" id="pin-toolbox" data-testid="pin-toolbox">
<div class="pin-tray" id="pin-tray" role="dialog" aria-label="Pin an artifact" hidden>
<h2>Pin an artifact</h2>
<p class="ritual-note">Put an existing record on the wall as planning material. It stays scratch &#8212; graduate it by drawing a typed edge, or it dies.</p>
<input id="pin-search" type="search" aria-label="Search artifacts" placeholder="search the corpus&#8230;" autocomplete="off">
<div id="pin-results" data-testid="pin-results"></div>
</div>
<button type="button" id="pin-toolbox-tab" class="pin-toolbox-tab" aria-expanded="false" aria-controls="pin-tray"><span class="pin-head" aria-hidden="true"></span>Pin an artifact</button>
</div>
<!-- The trash target (owner directive): fades in near the lower-right
     while a wall element is dragged; dropping removes per tier. A pure
     drop zone — pointer-events stay off; the gesture code measures it. -->
<div id="board-trash" class="board-trash" data-testid="board-trash" aria-hidden="true">
<svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true" focusable="false"><path d="M4 7h16M9 7V5.2A1.2 1.2 0 0 1 10.2 4h3.6A1.2 1.2 0 0 1 15 5.2V7M6.5 7l1 13h9l1-13M10 10.5v6M14 10.5v6" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>
<span class="board-trash-label" aria-hidden="true"></span>
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

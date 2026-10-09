package workbench

// The wall's top-bar controls (spec/wall-strip-and-drawer-v2 ac-3, ac-4,
// ac-6; ledger SI-368 (2), (7), (8), (15), (18); lane F3a): the readiness
// pill, Commit and push with its changes fragment, the sealed wall's
// primary actions, the ⋯ button the drawer lane wires, and the authoring
// wall's branch menu at the body level. The bar's controls are static
// markup outside every swapped fragment; what moves on a refresh — the
// Commit and push fragment, the pill's facts and the branch list — rides
// the snapshot, and wallstrip.js applies it.

import (
	stdhtml "html"
	"net/url"
	"strconv"
	"strings"
)

// The pill's data-state values.
const (
	pillStateStep        = "step"
	pillStateReady       = "ready"
	pillStateUnavailable = "unavailable"
	pillStateUnloaded    = "unloaded"
)

// readinessPillWords is the pill's text for its facts (SI-368 (2),
// (24)(b)), in two parts — a lead the bar may fold away where its row is
// narrow (the step, or the word "readiness"), and the rest, which always
// shows: the current step and that step's own unresolved count; "Ready"
// when every step is proven; the disclosure when the wall's readiness
// cannot be read; or the plain name when this view composed no facts, so
// the control is never missing from the bar. The third value is the
// pill's data-state.
func readinessPillWords(pill *wallPill) (string, string, string) {
	switch {
	case pill == nil:
		return "", "Readiness", pillStateUnloaded
	case pill.Unavailable != "":
		return "readiness ", "unavailable", pillStateUnavailable
	case pill.Step == 0:
		return "", "Ready", pillStateReady
	}
	return "Step " + strconv.Itoa(pill.Step) + " · ", strconv.Itoa(pill.Unresolved) + " to resolve", pillStateStep
}

// writeReadinessPill writes the bar's readiness pill: a link to the
// readiness page for this spec (SI-368 (15): the no-JavaScript path),
// carrying the data-drawer-tab hook the drawer lane enhances into the
// Readiness tab's opener, its facts as data attributes, its full words as
// its tooltip (the bar folds the lead away on a narrow row), and, when
// the readiness cannot be read, the reason as the tooltip and the
// screen-reader text instead.
func writeReadinessPill(b *strings.Builder, spec string, pill *wallPill) {
	esc := stdhtml.EscapeString
	lead, rest, state := readinessPillWords(pill)
	b.WriteString(`<a class="readiness-pill" data-testid="readiness-pill" data-drawer-tab="readiness" data-state="` + state + `" href="/readiness?spec=` + esc(url.QueryEscape(spec)) + `"`)
	if pill != nil && pill.Unavailable == "" {
		b.WriteString(` data-step="` + strconv.Itoa(pill.Step) + `" data-unresolved="` + strconv.Itoa(pill.Unresolved) + `"`)
	}
	title := lead + rest
	if pill != nil && pill.Unavailable != "" {
		title += ": " + pill.Unavailable
	}
	b.WriteString(` title="` + esc(title) + `">`)
	if lead != "" {
		b.WriteString(`<span class="readiness-pill-lead">` + esc(lead) + `</span>`)
	}
	b.WriteString(esc(rest))
	if pill != nil && pill.Unavailable != "" {
		b.WriteString(`<span class="topbar-sr">: ` + esc(pill.Unavailable) + `</span>`)
	}
	b.WriteString(`</a>`)
}

// writeWallControls writes the wall page's controls slot (SI-323 (5), dc-3):
// the Wall and Document switch; the readiness pill; in authoring, Commit
// and push with its changes fragment beside it (uncommitted is the
// snapshot's fragment, SI-368 (8)) under the anchor the readiness
// review/worktree concern points at (SI-368 (3)); on the sealed wall, New
// story and Revise as the bar's primary actions (ac-6; the ids the index
// call to action and the dialogs rely on; revise is the dialogs' own
// decision, renderBoardDialogs); the ⋯ button the drawer lane wires
// (ac-4); and the autosave status and live region.
func writeWallControls(b *strings.Builder, p *BoardProjection, pill *wallPill, uncommitted string, revise bool) {
	esc := stdhtml.EscapeString
	if p.DocumentHref != "" {
		// The Wall and Document switch, in the controls slot (dc-3), with
		// the same two labels the Document page's switch carries.
		b.WriteString(`<nav class="topbar-tabs" aria-label="Wall or Document"><span class="current" aria-current="page">Wall</span><a href="` + esc(p.DocumentHref) + `" data-testid="board-tab-document">Document</a></nav>`)
	}
	writeReadinessPill(b, p.Spec, pill)
	switch p.Mode {
	case modeAuthoring:
		// The button's words are "Commit & push" in full (its accessible
		// name, which the dialog's evidence locates it by); on a narrow row
		// the bar folds " & push" away from the eye.
		b.WriteString(`<div class="wall-commit-wrap" id="asd-git" data-testid="wall-commit-wrap"><button type="button" id="commit-push-btn" class="btn-primary">Commit<span class="wall-commit-push"> &amp; push</span></button>` + uncommitted + `</div>`)
	case modeReadOnly:
		writeSealedActions(b, p, revise)
	}
	b.WriteString(`<button type="button" class="wall-more-btn" data-testid="wall-more" aria-haspopup="menu" aria-expanded="false" aria-label="More">&#8943;</button>`)
	b.WriteString(`<div id="autosave-status" data-testid="autosave-status" role="status" aria-live="polite"></div>` +
		`<div id="asd-live" data-testid="asd-live" role="status" aria-live="polite" class="asd-live"></div>`)
}

// writeSealedActions writes the sealed wall's actions in the bar (ac-6):
// New story, rendered only when the loader attached the creation form's
// field descriptors (the same sealed-accepted-feature gate the create
// action enforces), as the primary action; and Revise, when the page's
// dialogs carry the revise dialog (the one sealed-accepted-feature
// decision, made where the lifecycle audit allows it), beside it. Every
// spoken class word is display prose and resolves (vocabulary.go); the
// ids and test ids stay bare. Each button's words are its full ones —
// "New <story>", "Revise this <feature>" — behind a decorative glyph; on
// a narrow row the bar folds the glyph and the words after the verb away
// from the eye, as it folds Commit's " & push", and the names stay whole.
func writeSealedActions(b *strings.Builder, p *BoardProjection, revise bool) {
	esc := stdhtml.EscapeString
	if len(p.CreateFields) > 0 {
		storyWord := p.words.word("story")
		b.WriteString(`<button type="button" id="create-spec-btn" class="btn-primary create-spec-btn" data-testid="create-spec-btn"><span class="wall-action-glyph" aria-hidden="true">&#8853;</span> New<span class="wall-action-rest"> ` + esc(storyWord) + `</span></button>`)
	}
	if revise {
		featureWord := p.words.word("feature")
		b.WriteString(`<button type="button" id="revise-spec-btn" class="create-spec-btn revise-spec-btn" data-testid="revise-spec-btn"><span class="wall-action-glyph" aria-hidden="true">&#8635;</span> Revise<span class="wall-action-rest"> this ` + esc(featureWord) + `</span></button>`)
	}
}

// renderBranchMenu renders the authoring wall's branch menu (ac-4; SI-368
// (7)) at the body level — never inside the bar, whose every control must
// be visible — filled from the wall's branch list under the ids the
// wall's script handles; wallstrip.js places it under the bar's switcher
// and refills it from each snapshot's list. Every other wall renders none.
func renderBranchMenu(p *BoardProjection, git *boardGitState) string {
	if p.Mode != modeAuthoring || git == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div role="menu" class="branch-menu wall-branch-menu" id="branch-menu" hidden aria-label="Switch branch">`)
	writeBranchMenuItems(&b, git.Branches)
	b.WriteString(`</div>`)
	return b.String()
}

// writeBranchMenuItems writes the menu's items, one per branch.
func writeBranchMenuItems(b *strings.Builder, branches []string) {
	esc := stdhtml.EscapeString
	for _, br := range branches {
		b.WriteString(`<button type="button" role="menuitem" data-branch="` + esc(br) + `">` + esc(br) + `</button>`)
	}
}

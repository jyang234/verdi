package workbench

// The wall's record drawer and its ⋯ menu (spec/wall-strip-and-drawer-v2
// ac-4, ac-5, co-1, co-2, dc-2; ledger SI-368 (6), (9), (10), (11), (15),
// (21), (22); lane F3b). Rendered at the body level beside the page's
// dialogs, in every wall mode — never inside the swapped region, so no
// refresh wipes it (SI-368 (9)). The markup is the drawer's frame and
// its static tabs: the seven tabs, an intro and a footer per reading
// aid, the Moves tab's copy, the Keys tab's keyboard table, and the
// Provenance tab's import origin (SI-368 (6)). Nothing here is fetched:
// walldrawer.js loads the Readiness, Provenance, Review and Context
// projections when their tab opens (SI-368 (10)), renders the Repo tab
// from the bar's posture facts and the Keys tab's yarn key from the
// hidden source in the region, and loads the menu's counts only when the
// menu opens (SI-368 (11)). Without JavaScript the drawer stays shut:
// the pill is a link to the readiness page, and the CLI hints say where
// the projections are (SI-368 (15)).

import (
	stdhtml "html"
	"strings"

	"github.com/jyang234/verdi/internal/model"
)

// recordTab is one tab of the record drawer: its id (the hook the pill,
// the menu and the toolbar open it by), its label, and, for a projection
// tab, the application operations it loads when it opens.
type recordTab struct {
	ID, Label string
	Ops       []string
}

// recordTabs is the drawer's seven tabs, in the handoff's order (ac-5):
// the four on-demand projections (SI-368 (10)), then the bar's posture
// facts, the moves and the keys, which load nothing.
func recordTabs() []recordTab {
	return []recordTab{
		{ID: "readiness", Label: "Readiness"},
		{ID: "provenance", Label: "Provenance", Ops: []string{"get_design_provenance"}},
		{ID: "review", Label: "Review", Ops: []string{"prepare_design_review"}},
		// The Context tab carries the capabilities families SI-368 (3)
		// homes there beside the design context.
		{ID: "context", Label: "Context", Ops: []string{"get_design_context", "get_design_capabilities"}},
		{ID: "repo", Label: "Repo"},
		{ID: "moves", Label: "Moves"},
		{ID: "keys", Label: "Keys"},
	}
}

// renderRecordDrawer renders the drawer, its scrim, the ⋯ menu and the
// no-JavaScript hints for one wall.
func renderRecordDrawer(p *BoardProjection, asd *asdView) string {
	var b strings.Builder
	esc := stdhtml.EscapeString
	ref := "spec/" + p.Spec
	b.WriteString(`<div class="record-drawer-scrim" id="record-drawer-scrim" data-testid="record-drawer-scrim" hidden></div>`)
	b.WriteString(`<section class="record-drawer" id="record-drawer" data-testid="record-drawer" role="dialog" aria-label="Record drawer" data-spec="` + esc(p.Spec) + `" hidden>`)
	b.WriteString(`<div class="record-drawer-head"><div class="record-drawer-tabs" role="tablist" aria-label="Record drawer tabs">`)
	for _, t := range recordTabs() {
		b.WriteString(`<button type="button" role="tab" class="record-drawer-tab" id="record-tab-` + t.ID + `" data-testid="record-tab-` + t.ID + `" data-record-tab="` + t.ID + `" aria-controls="record-panel-` + t.ID + `" aria-selected="false" tabindex="-1">` + t.Label + `</button>`)
	}
	// vocab:identity — non-vocabulary homograph: closing a UI panel, never the close lifecycle verb
	b.WriteString(`</div><button type="button" class="record-drawer-close" data-testid="record-drawer-close" aria-label="Close the record drawer">&#215;</button></div>`)
	b.WriteString(`<div class="record-drawer-panels">`)
	for _, t := range recordTabs() {
		b.WriteString(`<div class="record-drawer-panel" role="tabpanel" id="record-panel-` + t.ID + `" data-testid="record-panel-` + t.ID + `" data-record-panel="` + t.ID + `" aria-labelledby="record-tab-` + t.ID + `"`)
		if len(t.Ops) > 0 {
			b.WriteString(` data-record-ops="` + strings.Join(t.Ops, " ") + `"`)
		}
		b.WriteString(` tabindex="0" hidden>`)
		writeRecordPanel(&b, t.ID, p, asd, ref)
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div></section>`)
	writeWallMoreMenu(&b)
	b.WriteString(`<noscript><p class="record-drawer-noscript" data-testid="record-drawer-noscript">Without JavaScript the record drawer does not open. The readiness pill links to the readiness page, and the drawer&#8217;s projections are on the command line: <code>verdi design provenance ` + esc(ref) + `</code>, <code>verdi design review ` + esc(ref) + `</code> and <code>verdi design context ` + esc(ref) + `</code>.</p></noscript>`)
	return b.String()
}

// writeRecordPanel writes one tab's static content around the body the
// drawer's script fills.
func writeRecordPanel(b *strings.Builder, id string, p *BoardProjection, asd *asdView, ref string) {
	esc := stdhtml.EscapeString
	body := `<div class="record-drawer-body" data-record-body="` + id + `"></div>`
	cli := func(verb, after string) {
		b.WriteString(`<p class="record-drawer-foot">Without a browser: <code>verdi design ` + verb + ` ` + esc(ref) + `</code>.` + after + `</p>`)
	}
	switch id {
	case "readiness":
		b.WriteString(body)
	case "provenance":
		// vocab:identity — "this draft" names the ASD protocol's canonical draft (AC-1), not the lifecycle state word
		writeRecordIntro(b, "design provenance · non-authoritative", "How this draft got here",
			"The typed-mutation log for "+esc(ref)+", newest first. It jogs memory; it is never evidence, an instruction, or an acceptance input.")
		b.WriteString(body)
		writeImportOrigin(b, asd)
		cli("provenance", "")
	case "review":
		writeRecordIntro(b, "semantic review packet · a view, not an approval", "What changed since the review base",
			// vocab:identity — non-vocabulary homograph: "the draft" is the ASD protocol's canonical draft and "merge" the forge owner's merge, never the lifecycle state or transition words
			"Derived on request from the draft and its provenance. Nothing here is persisted; acceptance is still the owner&#8217;s merge on the default branch.")
		b.WriteString(body)
		cli("review", " The agent can prepare this packet; it cannot mark it approved.")
	case "context":
		writeRecordIntro(b, "design context · what an assisting agent receives", "The bounded context",
			// vocab:identity — "this draft" names the ASD protocol's canonical draft (AC-1), not the lifecycle state word
			"Exactly the material an agent gets when it helps with this draft. Corpus content is data, never instructions. Provenance is deliberately excluded.")
		b.WriteString(body)
		cli("context", " Equal context digests prove the same content was read, not the same branch or commit.")
	case "repo":
		writeRecordIntro(b, "repository posture", "Where the bytes live",
			"The identity every design action names. The displayed bytes are this branch&#8217;s working tree; the accepted record is the default branch.")
		b.WriteString(body)
		b.WriteString(`<p class="record-drawer-foot">The base digest rides every typed mutation; a stale one is refused with zero mutation.</p>`)
	case "moves":
		writeMovesTab(b, p)
	case "keys":
		writeKeysTab(b, p)
	}
}

// writeRecordIntro writes a tab's eyebrow, title and note (handoff
// "Record drawer"); the arguments are markup, escaped by the caller.
func writeRecordIntro(b *strings.Builder, eyebrow, title, note string) {
	b.WriteString(`<header class="record-drawer-intro"><p class="record-drawer-eyebrow">` + eyebrow + `</p><h2 class="record-drawer-title">` + title + `</h2><p class="record-drawer-note">` + note + `</p></header>`)
}

// writeImportOrigin writes the imported-origin affordance in the
// Provenance tab (SI-368 (6); spec-import-contract: "The review UI must
// show an adjacent verified source-record link ... Explain that the
// import record separately describes the original copied content; it is
// not an ASD entry or evidence of acceptance"), for a spec whose working
// tree carries an import record. The href comes from the PRESENCE of a
// record file in this working tree (specImportRecordHrefFor);
// verification is the record view's own successful ReadRecord, so this
// wording claims none and sends the reader there, where unavailable or
// tampered proof is disclosed.
func writeImportOrigin(b *strings.Builder, asd *asdView) {
	if asd == nil || asd.ImportRecordHref == "" {
		return
	}
	b.WriteString(`<p class="ritual-note asd-import-origin" data-testid="asd-import-origin"><a href="` + stdhtml.EscapeString(asd.ImportRecordHref) + `">Source record for this import</a> &mdash; this spec was created by importing existing content, and a record file is present in this working tree; it is not verified here. Open the record view to check the original copied content against the branch's committed bytes; it discloses unavailable or tampered proof. The record is not an ASD provenance entry (the review packet may still classify the creation as unclassified) and not evidence of acceptance.</p>`)
}

// writeRecordRows writes one section of key and value rows (handoff
// "Generic panel rows"), its rows already escaped markup.
func writeRecordRows(b *strings.Builder, heading string, rows [][2]string) {
	b.WriteString(`<section class="record-section"><div class="record-section-head"><h3 class="record-section-title">` + heading + `</h3></div><dl class="record-rows">`)
	for _, r := range rows {
		b.WriteString(`<div class="record-row"><dt class="record-key">` + r[0] + `</dt><dd class="record-value">` + r[1] + `</dd></div>`)
	}
	b.WriteString(`</dl></section>`)
}

// writeMovesTab writes the Moves tab (dc-2; SI-368 (22)): the handoff's
// four moves for the new gestures, with the store's class words (parent
// co-4), and parent dc-13's kept gestures described as they are. Like
// the four-move guide it rewrites, it is class-aware: a feature wall
// says it carries outcome criteria and stubs, never its stories. A wall
// that takes no edit says where the moves are made.
func writeMovesTab(b *strings.Builder, p *BoardProjection) {
	esc := stdhtml.EscapeString
	feature := p.Class == "feature"
	featureWord := esc(p.words.word("feature"))
	storyWord := esc(p.words.word("story"))
	spikeWord := esc(p.words.word("spike"))
	featureArticle := model.Article(p.words.word("feature"))
	eyebrow := "four moves"
	if p.Class != "" {
		eyebrow += " · " + esc(p.words.word(p.Class)) + " wall"
	}
	note := ""
	if feature {
		note = `This is ` + featureArticle + ` ` + featureWord + ` wall: outcome ACs and ` + storyWord + ` stubs. Each ` + storyWord + ` is its own spec that points up at these ACs with implements yarn &#8212; ` + featureArticle + ` ` + featureWord + ` never lists its ` + esc(p.words.plural("story")) + `.`
	}
	if p.Mode != modeAuthoring {
		if note != "" {
			note += " "
		}
		note += `These moves are made on a design branch&#8217;s authoring wall; this wall takes no edit.`
	}
	writeRecordIntro(b, eyebrow, "New to the wall?", note)
	criteria := `<strong>Pin acceptance criteria</strong> &#8212; the first column says what must be true. Use the dotted slot at the foot of a column to declare one.`
	if feature {
		criteria = `<strong>Pin acceptance criteria</strong> &#8212; the first column says what must be true when the ` + featureWord + ` lands. Use the dotted slot at the foot of a column to declare one.`
	}
	writeRecordRows(b, "The minimum path", [][2]string{
		{"1", `<strong>Read the case file</strong> &#8212; the problem and outcome above the wall are the spec&#8217;s own header. Click either to edit it in place.`},
		{"2", criteria},
		{"3", `<strong>String yarn</strong> &#8212; click a card, then drag its pin onto another card and pick the relationship.`},
		{"4", `<strong>Commit &amp; push</strong> &#8212; the wall autosaves as you work; committing files it on the design branch.`},
	})
	writeRecordRows(b, "Kept as they were", [][2]string{
		{"sticky", `A ` + storyWord + ` or ` + spikeWord + ` sticky parked in the stubs band keeps its handwritten face; graduating it typesets it.`},
		{"trash", `The trash still takes a dropped card, and it and the Delete key refuse a declared stub in plain words.`},
		{"stub pin", `A stub card&#8217;s pin anchors its coverage yarn but strings no thread, and its toolbar offers no graduate, delete or retype.`},
		{"drops", `A sticky&#8217;s attribution yarn and its graduation drops keep their paths; the type picker opens only for a drag between two spec objects.`},
	})
	b.WriteString(`<p class="record-drawer-foot">Everything else &#8212; stickies, graduation, exemptions &#8212; is on the wall when you need it.</p>`)
}

// writeKeysTab writes the Keys tab (dc-2): the wall's keyboard table —
// the keys that act on the selection only where the wall takes edits —
// and the yarn key, which the drawer's script copies from the hidden
// source the region keeps (SI-368 (9)), so it tracks the wall's edges.
func writeKeysTab(b *strings.Builder, p *BoardProjection) {
	writeRecordIntro(b, "keyboard", "Hands on the keys", "The keys act on the wall while no field or control has the focus.")
	writeRecordRows(b, "Selection", [][2]string{
		{`<kbd>&#8593;</kbd> <kbd>&#8595;</kbd>`, "the previous or next card in the column"},
		{`<kbd>&#8592;</kbd> <kbd>&#8594;</kbd>`, "the same row in the neighbouring column"},
		{`<kbd>Esc</kbd>`, "shut the open layer, one per press, then clear the selection"},
	})
	if p.Mode == modeAuthoring && p.DomainRefusal == "" {
		writeRecordRows(b, "Acting on the selection", [][2]string{
			{`<kbd>&#9166;</kbd>`, "edit the card&#8217;s text in place"},
			{`<kbd>&#9003;</kbd> <kbd>Del</kbd>`, "delete the card or thread, through its confirmation"},
			{"drag pin", "string a thread; the type picker opens where you drop it"},
		})
	}
	b.WriteString(`<div class="record-drawer-body" data-record-body="keys"></div>`)
	b.WriteString(`<p class="record-drawer-foot">Shortcuts never write anything the toolbar would not; every one routes through the same typed operation.</p>`)
}

// wallMoreItem is one row of the ⋯ menu: the tab it opens, its label,
// and its aside — a count the script loads when the menu opens
// (counted), or a fixed word.
type wallMoreItem struct {
	Tab, Label, Aside string
	Counted           bool
}

// writeWallMoreMenu writes the ⋯ menu (SI-368 (11)) at the body level,
// never inside the bar (SI-368 (7)'s rule for the branch menu): the
// design record's three reading aids, then this wall's three, each
// opening its tab. Provenance, Review and Repo carry counts, which load
// only when the menu opens; until then they say nothing.
func writeWallMoreMenu(b *strings.Builder) {
	groups := []struct {
		id, label string
		items     []wallMoreItem
	}{
		{"wall-more-record", "Design record", []wallMoreItem{
			{Tab: "provenance", Label: "Provenance", Counted: true},
			{Tab: "review", Label: "Semantic review", Counted: true},
			{Tab: "context", Label: "Design context", Aside: "what agents see"},
		}},
		{"wall-more-wall", "This wall", []wallMoreItem{
			{Tab: "repo", Label: "Repository details", Counted: true},
			{Tab: "moves", Label: "Four moves", Aside: "guide"},
			{Tab: "keys", Label: "Keyboard", Aside: "Esc &#8593;&#8595;&#8592;&#8594; &#9166; &#9003;"},
		}},
	}
	b.WriteString(`<div role="menu" class="wall-more-menu" id="wall-more-menu" data-testid="wall-more-menu" aria-label="More" hidden>`)
	for i, g := range groups {
		if i > 0 {
			b.WriteString(`<div role="separator" class="wall-more-sep"></div>`)
		}
		b.WriteString(`<div role="group" aria-label="` + g.label + `"><p class="wall-more-group" aria-hidden="true">` + g.label + `</p>`)
		for _, it := range g.items {
			b.WriteString(`<button type="button" role="menuitem" class="wall-more-item" data-record-tab="` + it.Tab + `" data-testid="wall-more-` + it.Tab + `"><span class="wall-more-label">` + it.Label + `</span>`)
			if it.Counted {
				b.WriteString(`<span class="wall-more-aside" data-count="` + it.Tab + `" data-testid="wall-more-count-` + it.Tab + `"></span>`)
			} else {
				b.WriteString(`<span class="wall-more-aside">` + it.Aside + `</span>`)
			}
			b.WriteString(`</button>`)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
}

package workbench

import (
	"strings"
	"testing"
)

// drawerPage renders the wall page of p over a minimal git state.
func drawerPage(t *testing.T, p *BoardProjection, asd *asdView) string {
	t.Helper()
	git := &boardGitState{Branch: "design/s", DefaultBranch: "main", Branches: []string{"main", "design/s"}}
	page, err := renderBoardSpecPage(t.Context(), p, git, asd)
	if err != nil {
		t.Fatalf("renderBoardSpecPage: %v", err)
	}
	return string(page)
}

// between is the slice of s from the first open up to the first close
// after it; the test fails when either is missing.
func between(t *testing.T, s, open, close string) string {
	t.Helper()
	at := strings.Index(s, open)
	if at < 0 {
		t.Fatalf("no %s in:\n%.600s", open, s)
	}
	end := strings.Index(s[at:], close)
	if end < 0 {
		t.Fatalf("no %s after %s", close, open)
	}
	return s[at : at+end]
}

// TestRecordDrawer_OutsideTheRegionInEveryMode (ac-5; SI-368 (9), (10);
// co-1, co-2): on every wall mode the page carries one record drawer and
// one ⋯ menu at the body level, after the swapped region — so no refresh
// wipes them — with the seven tabs in the handoff's order, each a tab
// controlling its own panel, shut until opened; only the four projection
// tabs name operations, and nothing in the drawer is a fetched or counted
// value: the menu's counted asides are empty until the menu opens, and no
// JSON rides the markup. The region the snapshot swaps carries none of it,
// and the derivation drawer keeps its own dialog beside it.
func TestRecordDrawer_OutsideTheRegionInEveryMode(t *testing.T) {
	ops := map[string]string{
		"provenance": "get_design_provenance",
		"review":     "prepare_design_review",
		"context":    "get_design_context get_design_capabilities",
	}
	for _, p := range []*BoardProjection{
		{Spec: "s", Mode: modeAuthoring, Class: "feature", Problem: "p", Outcome: "o"},
		{Spec: "s", Mode: modeReview, Class: "story", Problem: "p", Outcome: "o"},
		{Spec: "s", Mode: modeReadOnly, Class: "feature", Status: "accepted-pending-build", Problem: "p", Outcome: "o"},
	} {
		t.Run(string(p.Mode), func(t *testing.T) {
			page := drawerPage(t, p, testASDView())
			for _, once := range []string{` id="record-drawer"`, ` id="record-drawer-scrim"`, ` id="wall-more-menu"`, `data-testid="record-drawer-noscript"`} {
				if n := strings.Count(page, once); n != 1 {
					t.Errorf("%s appears %d times, want once", once, n)
				}
			}
			regionEnd := strings.Index(page, `</main>`)
			for _, at := range []string{` id="record-drawer"`, ` id="wall-more-menu"`} {
				if strings.Index(page, at) < regionEnd {
					t.Errorf("%s sits inside the region or the bar, not at the body level", at)
				}
			}
			if !strings.Contains(page, `<section class="record-drawer" id="record-drawer" data-testid="record-drawer" role="dialog" aria-label="Record drawer" data-spec="s" hidden>`) {
				t.Error("the drawer is not a shut, labelled dialog naming its spec")
			}
			tablist := between(t, page, `role="tablist"`, `</div>`)
			var order []string
			for _, tab := range recordTabs() {
				order = append(order, tab.ID)
				button := `<button type="button" role="tab" class="record-drawer-tab" id="record-tab-` + tab.ID + `" data-testid="record-tab-` + tab.ID + `" data-record-tab="` + tab.ID + `" aria-controls="record-panel-` + tab.ID + `" aria-selected="false" tabindex="-1">` + tab.Label + `</button>`
				if !strings.Contains(tablist, button) {
					t.Errorf("the tablist lacks %s", button)
				}
				panel := between(t, page, `id="record-panel-`+tab.ID+`"`, `>`)
				if !strings.Contains(panel, `aria-labelledby="record-tab-`+tab.ID+`"`) || !strings.HasSuffix(panel, ` tabindex="0" hidden`) {
					t.Errorf("the %s panel is not a shut tabpanel labelled by its tab: %s", tab.ID, panel)
				}
				if want, ok := ops[tab.ID]; ok != strings.Contains(panel, `data-record-ops="`+want+`"`) || (!ok && strings.Contains(panel, `data-record-ops`)) {
					t.Errorf("the %s panel's operations = %s, want %q", tab.ID, panel, want)
				}
			}
			if got := strings.Join(order, ","); got != "readiness,provenance,review,context,repo,moves,keys" {
				t.Errorf("tab order = %s", got)
			}
			drawer := between(t, page, ` id="record-drawer-scrim"`, `<script>`)
			for _, never := range []string{`{"`, `"schema"`, `asd-panel-json`, `data-holds-projection`, `badge-drawer`} {
				if strings.Contains(drawer, never) {
					t.Errorf("the drawer markup carries %s", never)
				}
			}
			snap := newASDSnapshot(p, &boardGitState{}, testASDView())
			for _, never := range []string{`record-drawer`, `wall-more-menu`} {
				if strings.Contains(snap.HTML, never) {
					t.Errorf("the swapped region carries %s", never)
				}
			}
		})
	}
}

// TestWallMoreMenu_CountsLoadOnOpen (ac-4; SI-368 (11)): the ⋯ menu lists
// the design record's three tabs and this wall's three, each a menu item
// naming the tab it opens; Provenance, Review and Repo carry an empty
// count the drawer's script fills only when the menu opens, and the
// others a fixed word. The bar's ⋯ button names the menu it controls.
func TestWallMoreMenu_CountsLoadOnOpen(t *testing.T) {
	var b strings.Builder
	writeWallMoreMenu(&b)
	menu := b.String()
	if !strings.HasPrefix(menu, `<div role="menu" class="wall-more-menu" id="wall-more-menu" data-testid="wall-more-menu" aria-label="More" hidden>`) {
		t.Fatalf("the menu is not a shut, labelled menu:\n%s", menu)
	}
	for _, tc := range []struct{ tab, label, aside string }{
		{"provenance", "Provenance", `<span class="wall-more-aside" data-count="provenance" data-testid="wall-more-count-provenance"></span>`},
		{"review", "Semantic review", `<span class="wall-more-aside" data-count="review" data-testid="wall-more-count-review"></span>`},
		{"context", "Design context", `<span class="wall-more-aside">what agents see</span>`},
		{"repo", "Repository details", `<span class="wall-more-aside" data-count="repo" data-testid="wall-more-count-repo"></span>`},
		{"moves", "Four moves", `<span class="wall-more-aside">guide</span>`},
		{"keys", "Keyboard", `<span class="wall-more-aside">Esc`},
	} {
		item := `<button type="button" role="menuitem" class="wall-more-item" data-record-tab="` + tc.tab + `" data-testid="wall-more-` + tc.tab + `"><span class="wall-more-label">` + tc.label + `</span>` + tc.aside
		if !strings.Contains(menu, item) {
			t.Errorf("the menu lacks %s", item)
		}
	}
	if strings.Contains(menu, `data-record-tab="readiness"`) {
		t.Error("the menu opens the Readiness tab, which is the pill's")
	}
	if n := strings.Count(menu, `role="menuitem"`); n != 6 {
		t.Errorf("the menu has %d items, want 6", n)
	}
	var controls strings.Builder
	writeWallControls(&controls, &BoardProjection{Spec: "s", Mode: modeReview}, nil, "", false)
	if !strings.Contains(controls.String(), `<button type="button" class="wall-more-btn" data-testid="wall-more" aria-haspopup="menu" aria-controls="wall-more-menu" aria-expanded="false" aria-label="More">&#8943;</button>`) {
		t.Errorf("the ⋯ button does not control the menu:\n%s", controls.String())
	}
}

// TestRecordDrawer_MovesAndKeys (dc-2; SI-368 (22); parent co-4, dc-13):
// the Moves tab is the handoff's four moves and parent dc-13's kept
// gestures in the store's class words — a feature wall's note and
// criteria move name the feature and its stories, a story wall's do not,
// a renamed vocabulary speaks its own words and never the bare ids, and a
// wall that takes no edit says where the moves are made. The Keys tab
// lists the selection keys on every wall and the editing keys only where
// the domain is live, with the yarn key's body for the script to fill.
func TestRecordDrawer_MovesAndKeys(t *testing.T) {
	render := func(p *BoardProjection, id string) string {
		var b strings.Builder
		writeRecordPanel(&b, id, p, testASDView(), "spec/"+p.Spec)
		return b.String()
	}
	feature := render(&BoardProjection{Spec: "s", Mode: modeAuthoring, Class: "feature"}, "moves")
	for _, want := range []string{
		`<p class="record-drawer-eyebrow">four moves · feature wall</p>`,
		`This is a feature wall: outcome ACs and story stubs. Each story is its own spec that points up at these ACs with implements yarn &#8212; a feature never lists its stories.`,
		`must be true when the feature lands. Use the dotted slot`,
		`click a card, then drag its pin onto another card and pick the relationship.`,
		`A story or spike sticky parked in the stubs band keeps its handwritten face`,
		`refuse a declared stub in plain words`,
		`the type picker opens only for a drag between two spec objects`,
	} {
		if !strings.Contains(feature, want) {
			t.Errorf("the feature wall's moves lack %q", want)
		}
	}
	if strings.Contains(feature, "takes no edit") {
		t.Error("the authoring wall's moves say it takes no edit")
	}
	story := render(&BoardProjection{Spec: "s", Mode: modeAuthoring, Class: "story"}, "moves")
	if strings.Contains(story, "This is a") || strings.Contains(story, "when the feature lands") || !strings.Contains(story, `four moves · story wall`) {
		t.Errorf("the story wall's moves speak as a feature wall:\n%s", story)
	}
	classless := render(&BoardProjection{Spec: "s", Mode: modeAuthoring}, "moves")
	if !strings.Contains(classless, `<p class="record-drawer-eyebrow">four moves</p>`) {
		t.Errorf("a class-less wall's eyebrow names a class:\n%s", classless)
	}
	sealed := render(&BoardProjection{Spec: "s", Mode: modeReadOnly, Class: "feature"}, "moves")
	if !strings.Contains(sealed, `These moves are made on a design branch&#8217;s authoring wall; this wall takes no edit.`) {
		t.Errorf("the sealed wall's moves do not say where they are made:\n%s", sealed)
	}

	renamed := &BoardProjection{Spec: "s", Mode: modeAuthoring, Class: "feature"}
	renamed.applyModelVocabulary(vocabTestModel())
	words := render(renamed, "moves")
	for _, want := range []string{"four moves · Initiative wall", "This is an Initiative wall", "Change Request stubs", "A Change Request or Deep Dive sticky"} {
		if !strings.Contains(words, want) {
			t.Errorf("the renamed moves lack %q", want)
		}
	}
	for _, bare := range []string{" feature ", " story ", " spike "} {
		if strings.Contains(words, bare) {
			t.Errorf("the renamed moves speak the bare id %q", bare)
		}
	}

	live := render(&BoardProjection{Spec: "s", Mode: modeAuthoring}, "keys")
	refused := render(&BoardProjection{Spec: "s", Mode: modeAuthoring, DomainRefusal: "not the namesake branch"}, "keys")
	review := render(&BoardProjection{Spec: "s", Mode: modeReview}, "keys")
	for name, keys := range map[string]string{"live": live, "refused": refused, "review": review} {
		for _, want := range []string{`<h3 class="record-section-title">Selection</h3>`, `<div class="record-drawer-body" data-record-body="keys"></div>`} {
			if !strings.Contains(keys, want) {
				t.Errorf("%s keys lack %s", name, want)
			}
		}
		if acting := strings.Contains(keys, "Acting on the selection"); acting != (name == "live") {
			t.Errorf("%s keys list the editing keys = %v", name, acting)
		}
	}
}

// TestRecordDrawer_ImportOriginInProvenance (SI-368 (6);
// spec-import-contract): the import origin rides the Provenance tab, once,
// for a spec whose working tree carries an import record, with every
// sentence that keeps it apart from ASD history and acceptance; the wall's
// region (the retired rail's panels included) never carries it, and a
// never-imported spec has none.
func TestRecordDrawer_ImportOriginInProvenance(t *testing.T) {
	asd := testASDView()
	asd.ImportRecordHref = "/design/import/record?branch=design%2Fs&spec=s"
	page := drawerPage(t, &BoardProjection{Spec: "s", Mode: modeAuthoring, Problem: "p", Outcome: "o"}, asd)
	if n := strings.Count(page, `data-testid="asd-import-origin"`); n != 1 {
		t.Fatalf("the import origin appears %d times, want once", n)
	}
	panel := between(t, page, `id="record-panel-provenance"`, `id="record-panel-review"`)
	origin := between(t, panel, `data-testid="asd-import-origin"`, `</p>`)
	for _, want := range []string{`href="/design/import/record?branch=design%2Fs&amp;spec=s"`, "not verified here", "not an ASD provenance entry", "classify the creation as unclassified", "not evidence of acceptance"} {
		if !strings.Contains(origin, want) {
			t.Errorf("the import origin lacks %q:\n%s", want, origin)
		}
	}
	if region := renderBoardRegion(&BoardProjection{Spec: "s", Mode: modeAuthoring, Problem: "p", Outcome: "o"}, &boardGitState{}, asd); strings.Contains(region, "asd-import-origin") {
		t.Error("the wall's region carries the import origin")
	}
	plain := drawerPage(t, &BoardProjection{Spec: "s", Mode: modeAuthoring, Problem: "p", Outcome: "o"}, testASDView())
	if strings.Contains(plain, "asd-import-origin") {
		t.Error("a never-imported spec carries an import origin")
	}
}

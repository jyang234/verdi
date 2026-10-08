package workbench

import (
	stdhtml "html"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// postureSlugs maps every barPosture field to the row it draws in the
// posture panel (asd-posture-<slug>), or to the summary's working-tree
// span and the branch row's attribute. TestTopBar_RendersEveryFactWithItsState
// walks barPosture by reflection against this map, so a fact the model
// gains and the bar does not draw fails here — the fact-completeness
// proof F1a's byte-identity test of the legacy posture row carried
// (barposture_test.go, retired with the row).
func postureSlugs() map[string]string {
	return map[string]string{
		"Checkout":       "checkout",
		"Branch":         "branch",
		"Detached":       "", // the branch row's data-detached attribute
		"Tree":           "", // the summary's asd-posture-tree span
		"WorktreeHead":   "worktree-head",
		"AcceptedBranch": "accepted-branch",
		"AcceptedHead":   "accepted-head",
		"AheadBehind":    "ahead-behind",
		"Divergence":     "divergence",
		"BaseDigest":     "base-digest",
	}
}

// wantFactRow is the panel row writeBarFact draws for one fact: its
// test-id element carrying exactly today's row's text in its state, then
// — when the text does not already carry the reason — the reason as a
// sibling row (SI-332 (2)).
func wantFactRow(slug string, f barFact) string {
	state := "proven"
	if f.Unproven != "" {
		state = unprovenWord
	}
	row := `<dd data-testid="asd-posture-` + slug + `" data-state="` + state + `"><code>` + stdhtml.EscapeString(f.Text) + `</code></dd>`
	if f.Unproven != "" && !strings.Contains(f.Text, f.Unproven) {
		row += `<dd class="topbar-fact-why" data-testid="asd-posture-` + slug + `-why">` + stdhtml.EscapeString(f.Unproven) + `</dd>`
	}
	return row
}

// expectPostureGroup asserts html — one rendering of writeASDPosture —
// states every fact of f in its three-valued state, and nothing the facts
// do not carry.
func expectPostureGroup(t *testing.T, html string, f *barFacts) {
	t.Helper()
	esc := stdhtml.EscapeString
	p := &f.Posture
	once := func(want, what string) {
		t.Helper()
		if n := strings.Count(html, want); n != 1 {
			t.Errorf("%s: want exactly one %q, got %d in\n%s", what, want, n, html)
		}
	}
	absent := func(fragment, what string) {
		t.Helper()
		if strings.Contains(html, fragment) {
			t.Errorf("%s: %q must be absent in\n%s", what, fragment, html)
		}
	}

	once(`<section class="topbar-posture-group" id="asd-posture" data-testid="asd-posture" aria-label="Repository posture">`, "the group")
	absent(`class="asd-posture"`, "the old row's class")

	// Every posture field, by reflection: a field this table does not
	// name is a fact the bar does not draw.
	rt := reflect.TypeOf(*p)
	rv := reflect.ValueOf(*p)
	slugs := postureSlugs()
	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Name
		slug, known := slugs[name]
		if !known {
			t.Fatalf("barPosture.%s is a fact the bar does not draw: add it to the posture group and to postureSlugs", name)
		}
		if slug == "" {
			continue
		}
		switch v := rv.Field(i).Interface().(type) {
		case barFact:
			row := wantFactRow(slug, v)
			if name == "Branch" && p.Detached && v.Unproven == "" {
				// Today's row printed an empty branch; the bar says why beside it.
				row += `<dd class="topbar-fact-why" data-testid="asd-posture-branch-why" data-detached="true">detached HEAD (no branch is checked out)</dd>`
			}
			once(row, name)
		case *barFact:
			if v == nil {
				absent(`data-testid="asd-posture-`+slug+`"`, name+" (nil)")
			} else {
				once(wantFactRow(slug, *v), name)
			}
		default:
			t.Fatalf("barPosture.%s has type %T, which this test does not read", name, v)
		}
	}
	once(`<span class="asd-posture-tree" data-testid="asd-posture-tree" data-dirty="`+esc(p.Tree.State)+`">working tree: `+esc(p.Tree.Text)+`</span>`, "Tree")
	if p.Tree.Unproven != "" {
		once(`<dt>Working tree</dt><dd class="topbar-fact-why" data-testid="asd-posture-tree-why">`+esc(p.Tree.Unproven)+`</dd>`, "Tree (why)")
	} else {
		absent(`asd-posture-tree-why`, "Tree (why, none)")
	}
	if !p.Detached {
		absent(`data-detached="true"`, "Detached (false)")
	}

	// The branch text, a control of its own, in its three states — and on
	// the authoring wall (every posture group this helper sees for a spec
	// is the wall's, with its Refresh) a proven name is the branch
	// switcher (spec/wall-strip-and-drawer-v2 ac-4; SI-368 (7)).
	switcher := f.Spec != nil && f.Spec.Unproven == "" && f.Spec.Mode == string(modeAuthoring)
	switch {
	case p.Branch.Unproven != "":
		once(`<span class="topbar-branch" data-testid="topbar-branch" data-state="unproven" title="`+esc(p.Branch.Unproven)+`">unproven<span class="topbar-sr">: `+esc(p.Branch.Unproven)+`</span></span>`, "branch text (unproven)")
		absent(`data-testid="branch-switcher"`, "branch switcher (unproven)")
	case p.Detached:
		once(`<span class="topbar-branch" data-testid="topbar-branch" data-state="proven" data-detached="true">detached HEAD</span>`, "branch text (detached)")
		absent(`data-testid="branch-switcher"`, "branch switcher (detached)")
	case switcher:
		once(`<span class="topbar-branch" data-testid="topbar-branch" data-state="proven"><button type="button" class="branch-switcher" data-testid="branch-switcher" aria-haspopup="menu" aria-controls="branch-menu" aria-expanded="false">`+esc(p.Branch.Text)+`</button></span>`, "branch switcher (authoring wall)")
	default:
		once(`<span class="topbar-branch" data-testid="topbar-branch" data-state="proven">`+esc(p.Branch.Text)+`</span>`, "branch text")
		absent(`data-testid="branch-switcher"`, "branch switcher (not the authoring wall)")
	}

	// The spec's facts: the mode chip, its disclosure, the status badge,
	// and the displayed bytes — on a page about one spec whose facts were
	// computed; none elsewhere.
	spec := f.Spec
	if spec != nil && spec.Unproven != "" {
		spec = nil
	}
	if spec == nil {
		absent(`board-mode-tag`, "mode chip (no spec)")
		absent(`asd-posture-bytes`, "displayed bytes (no spec)")
		absent(`board-status-badge`, "status badge (no spec)")
		return
	}
	once(`<span class="board-mode-tag board-mode-tag--`+esc(spec.Mode)+`">`+esc(spec.ModeLabel)+`</span>`, "mode chip")
	if spec.ModeDisclosure != "" {
		once(`data-testid="topbar-mode-disclosure" title="`+esc(spec.ModeDisclosure)+`">review state unproven<span class="topbar-sr">: `+esc(spec.ModeDisclosure)+`</span></span>`, "mode disclosure")
	} else {
		absent(`topbar-mode-disclosure`, "mode disclosure (none)")
	}
	if spec.StatusBadge != "" {
		once(`<span class="badge badge-`+esc(spec.StatusBadge)+` board-status-badge" data-testid="board-status-badge">`+esc(spec.StatusBadgeLabel)+`</span>`, "status badge")
	} else {
		absent(`board-status-badge`, "status badge (none)")
	}
	formal := "asd-posture-formal"
	if spec.Bytes.State == spec.Bytes.Word {
		formal += " topbar-sr"
	}
	once(`<span class="asd-posture-bytes" data-testid="asd-posture-bytes" data-state="`+esc(spec.Bytes.State)+`">displayed bytes: `+esc(spec.Bytes.Word)+` <span class="`+formal+`">(`+esc(spec.Bytes.State)+`)</span></span>`, "displayed bytes")
}

// TestTopBar_RendersEveryFactWithItsState (spec/chrome-and-tokens-v2 ac-2;
// SI-323 (3)): the bar's posture group states every fact the facts model
// carries, each in its own three-valued state — proven text, or
// "unproven: <reason>" marked data-state="unproven" — over synthetic
// postures covering every branch of the model, the wall fixtures' real
// loads (an authoring wall, a sealed record on the default branch, and a
// remote-only branch's sealed wall, whose working tree and base digest
// are disclosed-unproven), and a page not about one spec. The page's bar
// carries the same fragment the snapshot does, byte for byte, with the
// wall's Refresh; a page without the transport carries none.
func TestTopBar_RendersEveryFactWithItsState(t *testing.T) {
	type wallCase struct {
		name string
		p    *BoardProjection
		asd  *asdView
	}
	var cases []wallCase
	postures := map[string]branchPosture{
		"proven, dirty, ahead":   {Checkout: "/srv/a<b>", Branch: "design/s", DefaultBranch: "main", Dirty: true, WorktreeHead: "h1", AcceptedHead: "a1", Ahead: 2, AheadBehindKnown: true},
		"diverged":               {Checkout: "/c", Branch: "design/s", DefaultBranch: "trunk", WorktreeHead: "h", AcceptedHead: "a", Ahead: 1, Behind: 5, AheadBehindKnown: true},
		"behind only":            {Checkout: "/c", Branch: "main", DefaultBranch: "main", WorktreeHead: "h", AcceptedHead: "a", Behind: 2, AheadBehindKnown: true},
		"default branch unknown": {Checkout: "/c", Branch: "x", WorktreeHead: "h"},
		"detached, nothing":      {Checkout: "/c"},
		"no working tree":        {Checkout: "/c", Branch: "design/r", DefaultBranch: "main", WorktreeHead: "h", AcceptedHead: "a", treeWhy: "a remote-only branch has no working tree"},
	}
	projections := map[string]*BoardProjection{
		"authoring":            {Spec: "s", Title: "S <1>", Mode: modeAuthoring, Status: "draft", Class: "feature", ClassLabel: "feature"},
		"review":               {Spec: "s", Mode: modeReview, Status: "draft"},
		"sealed":               {Spec: "s", Mode: modeReadOnly, Status: "accepted-pending-build"},
		"unproven":             {Spec: "s", Mode: modeReadOnly, Status: "unproven"},
		"superseded, renamed":  {Spec: "s", Mode: modeReadOnly, Status: "superseded", StatusLabel: "Replaced & gone"},
		"closed spike, labels": {Spec: "s", Mode: modeReadOnly, Status: "closed", Class: "story", Spike: true, ClassLabel: "Probe"},
	}
	states := []string{"proposed", "draft", "accepted-pending-build", "closed", "superseded", "unproven", "", `odd"state`}
	notices := []string{"", "the review feed could not be consulted: <timeout>"}
	i := 0
	for _, pn := range slices.Sorted(maps.Keys(projections)) {
		for _, bn := range slices.Sorted(maps.Keys(postures)) {
			state, notice := states[i%len(states)], notices[i%len(notices)]
			i++
			cases = append(cases, wallCase{name: pn + " / " + bn + " / " + state, p: projections[pn], asd: &asdView{branchPosture: postures[bn], StateFormal: state, BaseDigest: "sha256:" + pn, reviewNotice: notice}})
		}
	}

	// The wall fixtures' real loads.
	for _, f := range []struct {
		name string
		root func(*testing.T) string
	}{
		{"authoring wall", newBoardFixture},
		{"sealed record on main", func(t *testing.T) string { return newStatuslessBoardFixture(t, false) }},
	} {
		root := f.root(t)
		proj, _, asd, err := (&boardSpecServer{root: root}).loadASD(t.Context(), boardFixtureName)
		if err != nil {
			t.Fatalf("%s: loadASD: %v", f.name, err)
		}
		cases = append(cases, wallCase{name: f.name, p: proj, asd: asd})
	}
	branchRoot := newBranchBoardFixture(t)
	b := newBranchBoards(branchRoot, Deps{}, &boardSpecServer{root: branchRoot})
	proj, _, err := b.loadSealed(t.Context(), "design/remote-only", "origin/design/remote-only", "remote-spec")
	if err != nil {
		t.Fatalf("loadSealed: %v", err)
	}
	cases = append(cases, wallCase{name: "remote-only sealed wall", p: proj, asd: sealedASDView("design/remote-only", "origin/design/remote-only", proj)})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bar := specBarFacts(tc.p, tc.asd)
			group := asdPostureHTML(&bar)
			expectPostureGroup(t, group, &bar)
			if !strings.Contains(group, `data-testid="asd-refresh"`) {
				t.Error("the wall's posture group lacks the Refresh control")
			}
			page := string(renderTopBar(&bar, topBarOptions{Heading: true, Refresh: true}))
			if !strings.Contains(page, group) {
				t.Errorf("the page's bar does not carry the snapshot's posture group byte for byte\n bar: %s\ngroup: %s", page, group)
			}
			if want := `<h1 class="topbar-title" data-testid="topbar-title">` + stdhtml.EscapeString(tc.p.Title) + `</h1>`; !strings.Contains(page, want) {
				t.Errorf("the bar lacks the page title %q", want)
			}
			if bar.Spec.Class != "" {
				if want := `<span class="topbar-chip topbar-chip--class topbar-chip--class-` + bar.Spec.Class + `" data-testid="topbar-class-chip">` + stdhtml.EscapeString(bar.Spec.ClassLabel) + `</span>`; !strings.Contains(page, want) {
					t.Errorf("the bar lacks the class chip %q", want)
				}
			} else if strings.Contains(page, "topbar-class-chip") {
				t.Error("a spec with no class wears a class chip")
			}
		})
	}
}

// TestTopBar_PagesNotAboutOneSpec: a page not about one spec draws the
// branch text and the posture with no spec chips and no displayed bytes;
// a page that could obtain no Git state discloses every fact unproven;
// the title is plain text when the page keeps its own h1, and the Refresh
// control is the wall's alone.
func TestTopBar_PagesNotAboutOneSpec(t *testing.T) {
	proven := (&branchPosture{Checkout: "/c", Branch: "main", DefaultBranch: "main", WorktreeHead: "h", AcceptedHead: "a", Ahead: 1, AheadBehindKnown: true}).facts()
	for _, tc := range []struct {
		name string
		f    barFacts
	}{
		{"proven branch-level facts", barFacts{Title: "Readiness", Posture: proven}},
		{"no git state at all", unprovenBarFacts("Error", "", "no store root is known to this page")},
		{"git state unreadable, checkout known", unprovenBarFacts("Error", "/c", "the checkout's Git state could not be read: boom")},
		{"spec facts unproven", barFacts{Title: "Document", Spec: &barSpec{Name: "s", Unproven: "the wall could not be loaded"}, Posture: proven}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.f
			var b strings.Builder
			writeASDPosture(&b, &f, false)
			group := b.String()
			expectPostureGroup(t, group, &f)
			if strings.Contains(group, "asd-refresh") {
				t.Error("a page without the snapshot transport carries a Refresh control")
			}
			page := string(renderTopBar(&f, topBarOptions{Nav: `<a href="/">index</a>`, Surface: true}))
			for _, want := range []string{
				`<header class="topbar" data-testid="topbar">`,
				`<a class="wordmark topbar-wordmark" data-testid="topbar-wordmark" href="/"><span class="leafmark" aria-hidden="true"></span>verdi<span class="wordmark-surface">workbench</span></a>`,
				`<span class="topbar-title" data-testid="topbar-title">` + f.Title + `</span>`,
				`<nav class="topbar-nav workbench-nav" aria-label="Workbench pages"><a href="/">index</a></nav>`,
				group,
				`<script src="/assets/topbar.js" defer></script></header>`,
			} {
				if !strings.Contains(page, want) {
					t.Errorf("bar lacks %q:\n%s", want, page)
				}
			}
			if strings.Contains(page, "<h1") || strings.Contains(page, "topbar-controls") {
				t.Errorf("bar draws an h1 or an empty controls slot it was not given:\n%s", page)
			}
			if f.Spec != nil && f.Spec.Unproven != "" {
				if want := `data-testid="topbar-spec-unproven" title="` + f.Spec.Unproven + `">spec facts unproven`; !strings.Contains(page, want) {
					t.Errorf("bar does not disclose the spec's unproven facts %q:\n%s", want, page)
				}
			}
		})
	}
	// Facts a page cannot obtain are disclosed in the panel, never
	// silently proven: every row unproven, the tree unproven.
	u := unprovenBarFacts("Error", "", "no store root is known to this page")
	var b strings.Builder
	writeASDPosture(&b, &u, false)
	if got := strings.Count(b.String(), `data-state="unproven"`); got != 7 {
		t.Errorf("unproven facts: %d marks, want 7 (the branch text, then the rows: checkout, branch, worktree head, accepted branch and head, ahead/behind)\n%s", got, b.String())
	}
	if strings.Contains(b.String(), `data-state="proven"`) {
		t.Errorf("a page with no Git state proves a fact:\n%s", b.String())
	}
}

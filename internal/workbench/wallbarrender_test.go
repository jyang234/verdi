package workbench

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/designscaffold"
)

// The wall's top-bar controls (spec/wall-strip-and-drawer-v2 ac-3, ac-4,
// ac-6; ledger SI-368 (2), (7), (8), (15), (18)).

// TestReadinessPill_Words: the pill's words and state for each fact the
// composed refresh can carry — a step with its count, every step proven,
// readiness unreadable with its reason disclosed, and no facts at all —
// and its one address, the readiness page for this spec, never without
// the drawer hook.
func TestReadinessPill_Words(t *testing.T) {
	for _, tc := range []struct {
		name       string
		pill       *wallPill
		wantText   string
		wantState  string
		wantInside []string
		wantAbsent []string
	}{
		{name: "a step with its count", pill: &wallPill{Step: 1, Unresolved: 3}, wantText: "Step 1 \u00b7 3 to resolve", wantState: "step", wantInside: []string{`data-step="1"`, `data-unresolved="3"`}, wantAbsent: []string{"title=", "topbar-sr"}},
		{name: "one to resolve", pill: &wallPill{Step: 4, Unresolved: 1}, wantText: "Step 4 \u00b7 1 to resolve", wantState: "step"},
		{name: "every step proven", pill: &wallPill{}, wantText: "Ready", wantState: "ready", wantInside: []string{`data-step="0"`}},
		{name: "unreadable", pill: &wallPill{Unavailable: `this wall serves <another> branch`}, wantText: "readiness unavailable", wantState: "unavailable", wantInside: []string{`title="this wall serves &lt;another&gt; branch"`, `<span class="topbar-sr">: this wall serves &lt;another&gt; branch</span>`}, wantAbsent: []string{"data-step", "<another>"}},
		{name: "no facts composed", pill: nil, wantText: "Readiness", wantState: "unloaded", wantAbsent: []string{"data-step", "title="}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeReadinessPill(&b, "refi-decline-flow", tc.pill)
			got := b.String()
			head := `<a class="readiness-pill" data-testid="readiness-pill" data-drawer-tab="readiness" data-state="` + tc.wantState + `" href="/readiness?spec=refi-decline-flow"`
			if !strings.HasPrefix(got, head) {
				t.Errorf("pill = %s, want it to open %s", got, head)
			}
			if !strings.Contains(got, `>`+tc.wantText+`<`) {
				t.Errorf("pill = %s, want the words %q", got, tc.wantText)
			}
			for _, want := range tc.wantInside {
				if !strings.Contains(got, want) {
					t.Errorf("pill = %s, want %s", got, want)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("pill = %s, must not carry %s", got, absent)
				}
			}
			if strings.Count(got, "<a ") != 1 || !strings.HasSuffix(got, "</a>") {
				t.Errorf("pill is not one link: %s", got)
			}
		})
	}
	var b strings.Builder
	writeReadinessPill(&b, `odd name&x`, nil)
	if !strings.Contains(b.String(), `href="/readiness?spec=odd+name%26x"`) {
		t.Errorf("the spec name is not query-escaped: %s", b.String())
	}
}

// TestWallBar_AuthoringControls: the authoring wall's page carries, inside
// its one controls slot, the readiness pill, Commit & push with the
// snapshot's changes fragment beside it under the asd-git anchor — the
// uncommitted indicator exactly once on the page, nowhere in the rail —
// and the ⋯ button; the bar's branch text is the switcher, and the branch
// menu sits at the body level, outside the bar, listing every branch under
// the ids the wall's script handles; no git panel remains in the rail.
func TestWallBar_AuthoringControls(t *testing.T) {
	root := newBoardFixture(t)
	page := getBoard(t, NewHandler(root), boardFixtureName).Body.String()
	bar := page[strings.Index(page, `<header class="topbar"`):strings.Index(page, `<main id="boardv2-region">`)]
	controls := bar[strings.Index(bar, `data-testid="topbar-controls"`):]
	for _, want := range []string{
		`<a class="readiness-pill" data-testid="readiness-pill" data-drawer-tab="readiness"`,
		`<div class="wall-commit-wrap" id="asd-git" data-testid="wall-commit-wrap"><button type="button" id="commit-push-btn" class="btn-primary">Commit &amp; push</button><div class="wall-commit" data-testid="wall-commit" data-changes="`,
		`data-testid="uncommitted-indicator"`,
		`<details class="wall-commit-popover" data-testid="wall-commit-popover"><summary class="wall-commit-count" data-testid="wall-commit-count">`,
		`<button type="button" class="wall-more-btn" data-testid="wall-more" aria-haspopup="menu" aria-expanded="false" aria-label="More">&#8943;</button>`,
	} {
		if !strings.Contains(controls, want) {
			t.Errorf("the controls slot lacks %s:\n%s", want, controls)
		}
	}
	for _, once := range []string{`data-testid="uncommitted-indicator"`, `id="commit-push-btn"`, `id="asd-git"`, `data-testid="readiness-pill"`, `data-testid="wall-more"`, `id="branch-menu"`, `data-testid="branch-switcher"`} {
		if n := strings.Count(page, once); n != 1 {
			t.Errorf("%s appears %d times on the page, want exactly once", once, n)
		}
	}
	if !strings.Contains(bar, `<button type="button" class="branch-switcher" data-testid="branch-switcher" aria-haspopup="menu" aria-controls="branch-menu" aria-expanded="false">`) {
		t.Errorf("the bar's branch text is not the switcher:\n%s", bar)
	}
	menuAt := strings.Index(page, `<div role="menu" class="branch-menu wall-branch-menu" id="branch-menu" hidden aria-label="Switch branch">`)
	if menuAt < 0 {
		t.Fatalf("no body-level branch menu:\n%s", page)
	}
	if menuAt < strings.Index(page, `</main>`) {
		t.Error("the branch menu sits inside the bar or the region, not at the body level")
	}
	menu := page[menuAt:]
	menu = menu[:strings.Index(menu, `</div>`)]
	for _, br := range []string{"main", "design/" + boardFixtureName} {
		if !strings.Contains(menu, `<button type="button" role="menuitem" data-branch="`+br+`">`+br+`</button>`) {
			t.Errorf("the branch menu does not list %s:\n%s", br, menu)
		}
	}
	for _, absent := range []string{`class="git-panel"`, `class="branch-row"`, `data-testid="create-spec-btn"`, `data-testid="revise-spec-btn"`} {
		if strings.Contains(page, absent) {
			t.Errorf("the authoring page still carries %s", absent)
		}
	}
}

// TestWallBar_ControlsByMode: the controls of each room, over synthetic
// projections — review carries the pill and ⋯ but no Commit & push, no
// indicator and no branch menu; the sealed accepted feature wall carries
// New story as the primary action and Revise beside it, once each, in the
// bar and not in the rail, and a sealed wall that offers neither carries
// neither; the Document page's bar, without the wall's Refresh, keeps the
// plain branch text.
func TestWallBar_ControlsByMode(t *testing.T) {
	git := &boardGitState{Branch: "design/s", DefaultBranch: "main", Branches: []string{"main", "design/s"}}
	render := func(t *testing.T, p *BoardProjection) string {
		t.Helper()
		page, err := renderBoardSpecPage(t.Context(), p, git, testASDView())
		if err != nil {
			t.Fatalf("renderBoardSpecPage: %v", err)
		}
		return string(page)
	}
	count := func(t *testing.T, page, fragment string, want int) {
		t.Helper()
		if n := strings.Count(page, fragment); n != want {
			t.Errorf("%s appears %d times, want %d:\n%s", fragment, n, want, page)
		}
	}

	review := render(t, &BoardProjection{Spec: "s", Mode: modeReview, Problem: "p", Outcome: "o"})
	count(t, review, `data-testid="readiness-pill"`, 1)
	count(t, review, `data-testid="wall-more"`, 1)
	for _, absent := range []string{`id="commit-push-btn"`, `data-testid="uncommitted-indicator"`, `id="branch-menu"`, `data-testid="branch-switcher"`, `create-spec-btn`, `revise-spec-btn`} {
		count(t, review, absent, 0)
	}

	sealed := render(t, &BoardProjection{
		Spec: "s", Mode: modeReadOnly, Class: "feature", Status: "accepted-pending-build", Problem: "p", Outcome: "o",
		CreateFields: []designscaffold.Field{{Name: "Title", Kind: designscaffold.FieldInput}},
	})
	controls := sealed[strings.Index(sealed, `data-testid="topbar-controls"`):strings.Index(sealed, `<main id="boardv2-region">`)]
	if !strings.Contains(controls, `<button type="button" id="create-spec-btn" class="btn-primary create-spec-btn" data-testid="create-spec-btn">&#8853; New story</button>`) {
		t.Errorf("the sealed wall's bar lacks New story as its primary action:\n%s", controls)
	}
	if !strings.Contains(controls, `<button type="button" id="revise-spec-btn" class="create-spec-btn revise-spec-btn" data-testid="revise-spec-btn">&#8635; Revise this feature</button>`) {
		t.Errorf("the sealed wall's bar lacks Revise:\n%s", controls)
	}
	count(t, sealed, `data-testid="create-spec-btn"`, 1)
	count(t, sealed, `data-testid="revise-spec-btn"`, 1)
	count(t, sealed, `data-testid="create-panel"`, 1)
	count(t, sealed, `data-testid="revise-panel"`, 1)
	for _, absent := range []string{`id="commit-push-btn"`, `data-testid="uncommitted-indicator"`, `id="branch-menu"`, `data-testid="branch-switcher"`} {
		count(t, sealed, absent, 0)
	}

	plain := render(t, &BoardProjection{Spec: "s", Mode: modeReadOnly, Class: "feature", Status: "closed", Problem: "p", Outcome: "o"})
	for _, absent := range []string{`create-spec-btn`, `revise-spec-btn`, `data-testid="create-panel"`, `data-testid="revise-panel"`} {
		count(t, plain, absent, 0)
	}

	// The Document page shares the bar without the wall's controls (SI-368
	// (18)): rendered without Refresh, an authoring spec keeps the plain
	// branch text.
	var b strings.Builder
	bar := specBarFacts(&BoardProjection{Spec: "s", Mode: modeAuthoring}, testASDView())
	writeASDPosture(&b, &bar, false)
	if strings.Contains(b.String(), "branch-switcher") {
		t.Errorf("a posture group without the wall's Refresh carries the switcher:\n%s", b.String())
	}
}

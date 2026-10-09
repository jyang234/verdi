package workbench

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/designscaffold"
)

// The rail's retirement (spec/wall-strip-and-drawer-v2 ac-6, dc-2; ledger
// SI-368 (5), (6), (9), (20), (27)(c); lane F3c): no wall, in any mode,
// renders the side rail or anything it held, and every item has its home —
// the review mirror's and the read-only walls' explanations in
// .board-notices with their hooks kept, the inbox tray docked below the
// wall frame, the yarn key a hidden source the drawer's Keys tab reads,
// Instantiate on the stub's toolbar (the card says the toolbar may offer
// it), and the add-object dialog's opener in the dialog layer.

// railMarkers is every element the retired rail carried, and the shell
// that sat beside it: none renders on any wall.
func railMarkers() []string {
	return []string{
		// The rail itself, and the rail panels' class.
		`class="board-side"`,
		`scratch-panel`,
		// The on-wall readiness shell beside it.
		`id="asd-shell"`,
		`data-testid="asd-shell"`,
		// The on-demand JSON panels.
		`data-asd-panel`,
		`asd-panel-json`,
		// The scratch panel's button.
		`id="add-sticky-btn"`,
		// The typed-operation forms panel, with its Set problem and Set
		// outcome.
		`id="asd-forms"`,
		`id="asd-set-problem"`,
		`id="asd-set-outcome"`,
		// The four-move guide.
		`data-testid="board-guide"`,
		// The shell's policy guide: its home is the Readiness tab.
		`asd-policy-guide`,
		// Instantiate inside a stub card.
		`class="stub-instantiate"`,
		`data-instantiate=`,
		// A visible yarn key.
		`data-testid="yarn-key">`,
	}
}

// railWall is one wall the rail used to sit beside.
type railWall struct {
	name string
	p    *BoardProjection
}

// railWalls is a wall in every mode and room: a live authoring wall, one
// under its domain refusal, the review mirror (with a trayed comment), the
// sealed accepted feature wall (with its creation fields), and the two
// read-only walls that are not the sealed record. Each carries yarn and
// stubs, so a rail that held the yarn key or Instantiate would show.
func railWalls(t *testing.T) []railWall {
	t.Helper()
	wall := func(mode boardModeKind, status string) *BoardProjection {
		p := scopingRenderProjection(t, mode)
		p.Status = status
		return p
	}
	refused := wall(modeAuthoring, "draft")
	refused.DomainRefusal = "not the namesake branch"
	review := wall(modeReview, "draft")
	review.Tray = []reviewStickyView{{Body: "a comment naming no card"}}
	sealed := wall(modeReadOnly, "accepted-pending-build")
	sealed.CreateFields = []designscaffold.Field{{Name: "Title", Kind: designscaffold.FieldInput}}
	return []railWall{
		{"a live authoring wall", wall(modeAuthoring, "draft")},
		{"an authoring wall under its domain refusal", refused},
		{"the review mirror", review},
		{"the sealed record", sealed},
		{"a read-only wall not yet accepted", wall(modeReadOnly, "draft")},
		{"a read-only wall whose lifecycle is unproven", wall(modeReadOnly, "")},
	}
}

// noticesOf slices the .board-notices wrapper out of a rendered region,
// or "" when the region has none.
func noticesOf(region string) string {
	start := strings.Index(region, `<div class="board-notices">`)
	end := strings.Index(region, `<div class="asd-main">`)
	if start < 0 || end < start {
		return ""
	}
	return region[start:end]
}

// TestWallRail_GoneFromEveryWall: no wall renders the rail, the shell or
// any item the rail held, in its region or anywhere on its page, whatever
// the policy guide's facts (the shell drew the guide when policy was
// missing); the yarn key stays in the region as a hidden source, which
// the drawer's Keys tab copies and the toolbar's Yarn key opens.
func TestWallRail_GoneFromEveryWall(t *testing.T) {
	for _, w := range railWalls(t) {
		t.Run(w.name, func(t *testing.T) {
			region := renderBoardRegion(w.p, &boardGitState{}, policyForbiddenView())
			page, err := renderBoardSpecPage(t.Context(), w.p, &boardGitState{Branch: "design/scoping-fixture"}, policyForbiddenView())
			if err != nil {
				t.Fatalf("renderBoardSpecPage: %v", err)
			}
			for _, marker := range railMarkers() {
				if strings.Contains(region, marker) {
					t.Errorf("the region still renders %s", marker)
				}
				if strings.Contains(string(page), marker) {
					t.Errorf("the page still renders %s", marker)
				}
			}
			if n := strings.Count(region, `<section class="yarn-key" data-testid="yarn-key" hidden>`); n != 1 {
				t.Errorf("the region carries the hidden yarn key source %d times, want once", n)
			}
		})
	}
}

// TestWallRail_EveryItemHasAHome (SI-368 (6), (20)): the review mirror's
// explanation and each read-only wall's explanation sit in .board-notices
// with their hooks kept — the sealed record's with the creation and revise
// notes beside it, on the wall whose bar offers those actions — and the
// review-mode inbox tray docks directly below the wall frame, never
// inside it, keeping role=region "Inbox tray".
func TestWallRail_EveryItemHasAHome(t *testing.T) {
	for _, w := range railWalls(t) {
		t.Run(w.name, func(t *testing.T) {
			region := renderBoardRegion(w.p, &boardGitState{}, testASDView())
			notices := noticesOf(region)
			var want, absent []string
			switch {
			case w.p.Mode == modeReview:
				want = []string{`<section class="mirror-note board-note">`, "mirrors the merge request"}
			case w.p.Mode == modeReadOnly && w.p.Status == "accepted-pending-build":
				want = []string{`<section class="sealed-panel board-note">`, "This spec is accepted", `data-testid="create-panel"`, `data-testid="revise-panel"`}
				absent = []string{`data-testid="readonly-panel"`}
			case w.p.Mode == modeReadOnly && w.p.Status == "draft":
				want = []string{`data-testid="readonly-panel" data-readonly-reason="not-accepted"`}
				absent = []string{`sealed-panel`, `data-testid="create-panel"`, `data-testid="revise-panel"`}
			case w.p.Mode == modeReadOnly:
				want = []string{`data-testid="readonly-panel" data-readonly-reason="unproven"`, "git remote set-head origin"}
				absent = []string{`sealed-panel`, `data-testid="create-panel"`, `data-testid="revise-panel"`}
			default:
				absent = []string{`mirror-note`, `sealed-panel`, `data-testid="readonly-panel"`, `data-testid="create-panel"`, `data-testid="revise-panel"`}
			}
			for _, s := range want {
				if !strings.Contains(notices, s) {
					t.Errorf(".board-notices lacks %s:\n%s", s, notices)
				}
			}
			for _, s := range absent {
				if strings.Contains(region, s) {
					t.Errorf("the region renders %s", s)
				}
			}

			const docked = `<div class="wall-minimap" data-testid="wall-minimap" aria-hidden="true"></div></div></div></div><section class="inbox-tray" role="region" aria-label="Inbox tray">`
			tray := strings.Count(region, `aria-label="Inbox tray"`)
			switch {
			case w.p.Mode == modeReview && (tray != 1 || !strings.Contains(region, docked)):
				t.Errorf("the review mirror's tray is not docked once directly below the wall frame (%d trays)", tray)
			case w.p.Mode != modeReview && tray != 0:
				t.Errorf("a %s wall renders an inbox tray", w.p.Mode)
			}
		})
	}
}

// TestWallRail_InstantiateOnTheStubsToolbar (SI-368 (5); BL-167): on the
// sealed accepted feature wall every stub card says the toolbar may offer
// Instantiate, and a spike stub says it is a spike, the word the toolbar's
// button and its confirmation speak; the card itself carries no
// Instantiate button. No other wall's stub says so.
func TestWallRail_InstantiateOnTheStubsToolbar(t *testing.T) {
	for _, w := range railWalls(t) {
		t.Run(w.name, func(t *testing.T) {
			region := renderBoardRegion(w.p, &boardGitState{}, testASDView())
			offered := w.p.Mode == modeReadOnly && w.p.Status == "accepted-pending-build"
			for _, sv := range w.p.StubViews {
				at := strings.Index(region, `data-testid="stub-card-`+sv.Slug+`"`)
				card := region[strings.LastIndex(region[:at], `<div class="stubcard`):]
				card = card[:strings.Index(card, `>`)]
				if got := strings.Contains(card, `data-can-instantiate="true"`); got != offered {
					t.Errorf("stub %s says the toolbar may offer Instantiate = %v, want %v:\n%s", sv.Slug, got, offered, card)
				}
				if got := strings.Contains(card, `data-spike="true"`); got != sv.Spike {
					t.Errorf("stub %s carries data-spike = %v, want %v", sv.Slug, got, sv.Spike)
				}
			}
			if strings.Contains(region, "Instantiate") {
				t.Error("the region renders an Instantiate control inside a stub card")
			}
		})
	}
}

// TestWallRail_AddObjectOpenerInTheDialogLayer (SI-368 (6)): the add-object
// dialog's opener survives in the dialog layer, hidden — the toolbar's
// Card ▾ opens the dialog through it — exactly where the typed-operation
// forms rendered: a live authoring wall, never under its domain refusal
// or in another mode.
func TestWallRail_AddObjectOpenerInTheDialogLayer(t *testing.T) {
	const opener = `<button type="button" id="asd-add-object" data-asd-op="add-object" hidden>Add object&#8230;</button>`
	for _, w := range railWalls(t) {
		t.Run(w.name, func(t *testing.T) {
			dialogs, _ := renderBoardDialogs(w.p)
			want := 0
			if w.p.Mode == modeAuthoring && w.p.DomainRefusal == "" {
				want = 1
			}
			if n := strings.Count(dialogs, opener); n != want {
				t.Errorf("the dialog layer carries the add-object opener %d times, want %d", n, want)
			}
			if strings.Contains(renderBoardRegion(w.p, &boardGitState{}, testASDView()), `asd-add-object`) {
				t.Error("the region carries the add-object opener")
			}
		})
	}
}

// TestWallRail_EmptyWallNamesTheToolbar: the empty authoring wall's
// invitation names the toolbar's Sticky, never the retired rail.
func TestWallRail_EmptyWallNamesTheToolbar(t *testing.T) {
	empty := &BoardProjection{Spec: "fresh", Mode: modeAuthoring, Class: "story", Problem: "p", Outcome: "o"}
	body := renderBoardRegion(empty, &boardGitState{Branch: "design/fresh"}, testASDView())
	how := body[strings.Index(body, `<p class="board-empty-how">`):]
	how = how[:strings.Index(how, `</p>`)]
	if !strings.Contains(how, `<strong>Sticky</strong>`) || !strings.Contains(how, "toolbar") {
		t.Errorf("the empty wall's invitation does not name the toolbar's Sticky: %s", how)
	}
	if strings.Contains(how, "rail") || strings.Contains(how, "Add sticky") {
		t.Errorf("the empty wall's invitation names the retired rail: %s", how)
	}
}

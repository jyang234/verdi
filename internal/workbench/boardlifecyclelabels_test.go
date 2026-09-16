package workbench

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// R2 of the MVP release amendment (docs/superpowers/plans/2026-08-29-wave-6-
// workbench-presentation.md, 2026-09-13) under merge-signaled acceptance
// AC-7 ("Missing default-branch or ancestry evidence produces an explicit
// unproven result, never an assumed acceptance"): a read-only wall speaks
// WHY it is read-only from the spec's EFFECTIVE lifecycle status — the
// specstate verdict the loader already carries in BoardProjection.Status —
// never from the CSS mode alone. Only a revision proven on the default
// branch (accepted-pending-build, superseded, closed) is the sealed record.
// An unproven lifecycle discloses the missing witness and its remedy and
// asserts neither acceptance nor sealing; a not-yet-accepted revision that
// renders read-only for a branch or divergence reason is named as such.

const (
	sealedStamp   = "read-only · sealed record"
	sealedClaim   = "This spec is accepted"
	unprovenStamp = "read-only · lifecycle unproven"
	notAccepted   = "read-only · not yet accepted"
)

// bodyMustHave/bodyMustLack name the exact markup at stake; the mode
// stamp is quoted alongside so a failure reads as the wall's own claim.
func bodyMustHave(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("board missing %q (stamp: %q)", want, modeStampOf(body))
		}
	}
}

func bodyMustLack(t *testing.T, body string, banned ...string) {
	t.Helper()
	for _, b := range banned {
		if strings.Contains(body, b) {
			t.Errorf("board must not contain %q (stamp: %q)", b, modeStampOf(body))
		}
	}
}

// readOnlyPanelOf slices the rendered read-only rail panel out of a page,
// or returns "" when the page carries none — so remedy assertions can be
// scoped to the panel's OWN copy rather than the whole page.
func readOnlyPanelOf(body string) string {
	start := strings.Index(body, `data-testid="readonly-panel"`)
	if start < 0 {
		return ""
	}
	end := strings.Index(body[start:], "</section>")
	if end < 0 {
		return ""
	}
	return body[start : start+end]
}

// modeStampOf extracts the rendered mode stamp's text, or "" when absent.
func modeStampOf(body string) string {
	const open = `class="board-mode-tag board-mode-tag--`
	i := strings.Index(body, open)
	if i < 0 {
		return ""
	}
	rest := body[i:]
	start := strings.Index(rest, ">")
	end := strings.Index(rest, "</span>")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return rest[start+1 : end]
}

// newUnprovenLifecycleFixture is the baseline report's B-06 shape: the
// CLI's statusless scaffold on its design branch in a repository with NO
// resolvable default branch — no CI_DEFAULT_BRANCH, no origin/HEAD, no
// remote-tracking main/master — so specstate projects Unproven.
func newUnprovenLifecycleFixture(t *testing.T) string {
	t.Helper()
	neutralizeCIEnv(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/adr/0001-outbox-events.md": boardFixtureADR,
			".verdi/.gitignore":                "data/\n",
		},
		Message: "seed unproven-lifecycle fixture",
	}})
	commitFilesOnBranch(t, repo.Dir, "design/"+boardFixtureName, map[string]string{
		".verdi/specs/active/" + boardFixtureName + "/spec.md":     statuslessFixtureSpec,
		".verdi/specs/active/" + boardFixtureName + "/layout.json": boardFixtureLayout,
	})
	return repo.Dir
}

// TestBoard_UnprovenLifecycle_ReadOnlyWithoutAcceptanceClaims is the direct
// regression for B-06: the wall stays read-only, discloses the missing
// witness and the supported local remedy, and never dresses the unproven
// state as the accepted/sealed record — in the stamp, the rail, or the
// machine-readable reason — while, three-valued, never asserting the
// opposite either (missing proof proves no negative). Every editing
// affordance is absent and every write (scratch tier and domain surface
// alike) is refused.
func TestBoard_UnprovenLifecycle_ReadOnlyWithoutAcceptanceClaims(t *testing.T) {
	root := newUnprovenLifecycleFixture(t)
	h := newBoardTestHandler(root)

	rec := getBoard(t, h, boardFixtureName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// Fails closed to read-only and names the reason, in words and as data.
	bodyMustHave(t, body,
		`data-board-mode="readonly"`,
		`data-readonly-reason="unproven"`,
		unprovenStamp,
		`data-testid="readonly-panel"`,
		"cannot claim acceptance or sealing",
		// The posture header's plain word follows the formal state.
		"displayed bytes: unproven",
		// The missing witness (the existing honest notice), whose remedy is
		// the local one: the configured default branch fetched and
		// origin/HEAD pointed at it, then reload.
		`data-testid="board-notice"`,
		"default branch could not be resolved",
		"point origin/HEAD at it",
		"then reload",
	)
	// Never the sealed record — never the proven negative either — and
	// never a CI-environment workaround as the remedy.
	bodyMustLack(t, body, sealedStamp, sealedClaim, `sealed-panel`, `data-readonly-reason="sealed"`,
		"neither accepted nor sealed", "is not accepted", "set CI_DEFAULT_BRANCH", "displayed bytes: proposed")
	// The panel's own remedy is the local one — the configured default
	// branch fetched and origin/HEAD pointed at it — never a CI-environment
	// workaround (adopted R0 forbids one for local adoption).
	panel := readOnlyPanelOf(body)
	if !strings.Contains(panel, "git remote set-head origin") {
		t.Errorf("read-only panel missing the origin/HEAD remedy: %q", panel)
	}
	if strings.Contains(panel, "CI_DEFAULT_BRANCH") {
		t.Errorf("read-only panel offers a CI-environment workaround: %q", panel)
	}
	// No editing affordance of any tier.
	bodyMustLack(t, body,
		`id="add-sticky-btn"`, `id="commit-push-btn"`, `data-testid="asd-forms"`,
		`data-testid="create-panel"`, `class="delete-btn"`, `class="graduate-btn"`, `data-retype`,
	)

	// Mutation boundary: the scratch tier and the domain surface both refuse.
	if rec := postBoardAPI(t, h, boardFixtureName, "sticky", `{"text":"unproven wall","type":"comment"}`); rec.Code != http.StatusForbidden {
		t.Errorf("sticky on an unproven wall = %d, want 403\n%s", rec.Code, rec.Body.String())
	}
	if mrec, _ := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "x", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil); mrec.Code != http.StatusForbidden {
		t.Errorf("mutate_draft on an unproven wall = %d, want 403\n%s", mrec.Code, mrec.Body.String())
	}
}

// TestBoard_ProvenLifecycle_RetainsSealedRecord: the proven default-branch
// states keep their truthful sealed labels — the repair distinguishes
// unproven from sealed, it does not dilute the sealed record.
func TestBoard_ProvenLifecycle_RetainsSealedRecord(t *testing.T) {
	legacyTerminal := func(status string) string {
		return strings.Replace(boardFixtureSpec, "status: draft\n",
			"status: "+status+"\nfrozen: { at: 2026-03-01, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }\n", 1)
	}
	landed := func(t *testing.T, spec string) string {
		t.Helper()
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/specs/active/" + boardFixtureName + "/spec.md": spec,
				".verdi/adr/0001-outbox-events.md":                     boardFixtureADR,
				".verdi/.gitignore":                                    "data/\n",
			},
			Message: "seed landed fixture",
		}})
		setDefaultBranchSymref(t, repo.Dir)
		return repo.Dir
	}
	tests := []struct {
		name      string
		root      func(t *testing.T) string
		wantBytes string
	}{
		{"accepted-pending-build: statusless bytes exact on the default branch", func(t *testing.T) string { return newStatuslessBoardFixture(t, false) }, "displayed bytes: accepted"},
		{"superseded: legacy terminal status landed on the default branch", func(t *testing.T) string { return landed(t, legacyTerminal("superseded")) }, "displayed bytes: superseded"},
		{"closed: legacy terminal status landed on the default branch", func(t *testing.T) string { return landed(t, legacyTerminal("closed")) }, "displayed bytes: closed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := getBoard(t, NewHandler(tc.root(t)), boardFixtureName).Body.String()
			bodyMustHave(t, body, `data-board-mode="readonly"`, `data-readonly-reason="sealed"`, sealedStamp, `sealed-panel`, sealedClaim, tc.wantBytes)
			bodyMustLack(t, body, unprovenStamp, notAccepted, `data-testid="readonly-panel"`, "displayed bytes: proposed")
		})
	}
}

// TestBoard_NotYetAcceptedReadOnly_NeverLabeledSealed: the two reachable
// read-only shapes whose effective state is still Proposed — new content
// served from the default-branch checkout, and bytes diverged from the
// accepted revision on the design branch — are read-only for a branch or
// divergence reason, not because anything was accepted; the generic
// read-only fallback may not call them sealed.
func TestBoard_NotYetAcceptedReadOnly_NeverLabeledSealed(t *testing.T) {
	ctx := context.Background()
	t.Run("proposed content on the default-branch checkout", func(t *testing.T) {
		root := newStatuslessBoardFixture(t, true) // committed on the design branch only
		if err := gitx.Checkout(ctx, root, "main"); err != nil {
			t.Fatal(err)
		}
		writeWorkingTreeSpec(t, root, boardFixtureName, statuslessFixtureSpec) // uncommitted, on main
		body := getBoard(t, NewHandler(root), boardFixtureName).Body.String()
		bodyMustHave(t, body, `data-board-mode="readonly"`, `data-readonly-reason="not-accepted"`, notAccepted, `data-testid="readonly-panel"`, "design/"+boardFixtureName, "displayed bytes: proposed")
		bodyMustLack(t, body, sealedStamp, sealedClaim, `sealed-panel`, unprovenStamp)
	})
	t.Run("diverged from the accepted revision on the design branch", func(t *testing.T) {
		root := newStatuslessBoardFixture(t, false) // landed on main
		if err := gitx.CheckoutNewBranch(ctx, root, "design/"+boardFixtureName); err != nil {
			t.Fatal(err)
		}
		writeWorkingTreeSpec(t, root, boardFixtureName, strings.Replace(statuslessFixtureSpec, "current decline flow", "current decline flow (diverged)", 1))
		body := getBoard(t, NewHandler(root), boardFixtureName).Body.String()
		bodyMustHave(t, body, `data-board-mode="readonly"`, `data-readonly-reason="not-accepted"`, notAccepted, "diverge", "displayed bytes: proposed")
		bodyMustLack(t, body, sealedStamp, sealedClaim, `sealed-panel`, unprovenStamp)
	})
}

// TestWriteASDPosture_DisplayedBytesWordByStateFormal: the posture
// header's plain word is a pure function of the formal state — both
// spellings of proposed map to "proposed", the three proven default-branch
// states keep their own words, and anything unrecognized (including an
// absent state) reads as unproven, never as proposed or accepted.
func TestWriteASDPosture_DisplayedBytesWordByStateFormal(t *testing.T) {
	proj := &BoardProjection{Spec: "s", Title: "S", Mode: modeReadOnly, Cards: []cardView{{ID: "ac-1", Kind: "acceptance-criterion", Text: "x"}}}
	for _, tc := range []struct{ state, want string }{
		{"proposed", "proposed"},
		{"draft", "proposed"},
		{"accepted-pending-build", "accepted"},
		{"closed", "closed"},
		{"superseded", "superseded"},
		{"unproven", "unproven"},
		{"", "unproven"},
		{"bogus", "unproven"},
	} {
		asd := testASDView()
		asd.StateFormal = tc.state
		body := renderBoardRegion(proj, &boardGitState{}, asd)
		if want := `displayed bytes: ` + tc.want + ` <span class="asd-posture-formal">`; !strings.Contains(body, want) {
			t.Errorf("StateFormal %q: posture header missing %q", tc.state, want)
		}
	}
	// And the same function is mode-independent: an authoring wall's
	// proposed bytes read "proposed", not something inferred from the room.
	proj.Mode = modeAuthoring
	asd := testASDView()
	asd.StateFormal = "proposed"
	if body := renderBoardRegion(proj, &boardGitState{Branch: "design/s"}, asd); !strings.Contains(body, "displayed bytes: proposed ") {
		t.Errorf("authoring wall's proposed bytes not read as proposed")
	}
}

// TestBoardDiagram_ReadOnlyStampNeverSealed: the diagram editor is a
// /board tool, and its read-only mode derives from diagram status plus
// branch state — never proof of a sealed or accepted record — so its
// read-only stamp is the generic "read-only · diagram" while the separate
// status badge keeps carrying the diagram's declared status.
func TestBoardDiagram_ReadOnlyStampNeverSealed(t *testing.T) {
	root, _ := newDiagramFixture(t)
	if err := gitx.Checkout(context.Background(), root, "main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	body := getDiagram(t, NewHandler(root), diagramFixtureName, "").Body.String()
	bodyMustHave(t, body, `data-editor-mode="readonly"`, "read-only · diagram", `data-testid="diagram-status-badge"`)
	bodyMustLack(t, body, "sealed record")
}

// TestRenderBoardRegion_ReadOnlyReasonFailsClosed: a read-only projection
// carrying no effective status at all (pure renders, tests) must never be
// dressed as the sealed record — the undeclared case reads as unproven.
func TestRenderBoardRegion_ReadOnlyReasonFailsClosed(t *testing.T) {
	proj := &BoardProjection{Spec: "s", Title: "S", Mode: modeReadOnly, Cards: []cardView{{ID: "ac-1", Kind: "acceptance-criterion", Text: "x"}}}
	body := renderBoardRegion(proj, &boardGitState{}, testASDView())
	bodyMustHave(t, body, `data-readonly-reason="unproven"`, unprovenStamp)
	bodyMustLack(t, body, sealedStamp, sealedClaim, `sealed-panel`)
}

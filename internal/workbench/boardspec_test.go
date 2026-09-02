package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
)

// The board-fixture spec: a draft feature on a design branch — the v1
// board's authoring case. dc-1 carries a declared exempts edge (projects
// as spec-layer yarn with an external reference card); dc-2 is plain
// (fresh yarn draws from it).
const boardFixtureSpec = `---
id: spec/refi-test
kind: spec
class: feature
title: "Refi test flow"
status: draft
owners: [platform-team]
problem: { text: "declined applicants act on stale decline reasons", anchor: "#problem" }
outcome: { text: "declined applicants see a current decline flow", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a declined applicant sees the current reason", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a decline reverses within one day", evidence: [behavioral, attestation], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "notices never name internal model scores", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "excuse this flow from the outbox rule", anchor: "#dc-1",
      links: [ { type: exempts, ref: adr/0001-outbox-events, note: "async by design" } ] }
  - { id: dc-2, text: "reuse the notification channel", anchor: "#dc-2" }
---
# Refi test flow

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## ac-2

Prose.

## co-1

Prose.

## dc-1

Prose.

## dc-2

Prose.
`

const boardFixtureLayout = `{
  "schema": "verdi.boardlayout/v1",
  "positions": { "ac-1": { "x": 40, "y": 60 } }
}
`

const boardFixtureName = "refi-test"

// boardFixtureADR is the ADR dc-1's exempts edge targets — a real,
// peekable corpus artifact (the ref-peek tests resolve it).
const boardFixtureADR = `---
id: adr/0001-outbox-events
kind: adr
title: "Outbox pattern for domain events (board fixture)"
status: accepted
owners: [platform-team]
decided: 2026-03-01
frozen: { at: 2026-03-01, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# Outbox pattern for domain events

## Decision

Domain events leave through the transactional outbox.
`

// boardFixtureADR2 is a second, unreferenced corpus ADR — nothing on the
// wall names it, so it is the pin lifecycle's import candidate.
const boardFixtureADR2 = `---
id: adr/0007-retry-budget
kind: adr
title: "Retry budget for downstream calls (board fixture)"
status: accepted
owners: [platform-team]
decided: 2026-03-02
frozen: { at: 2026-03-02, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# Retry budget for downstream calls

## Decision

Every downstream call spends from a shared retry budget.
`

// newBoardFixture builds a fixture repo with the draft spec authored on a
// design branch (authoring mode's state under merge-signaled acceptance:
// the draft is a NEW path — Proposed/RelationNew — exactly the shape
// `verdi design start`'s scaffold produces; main carries no revision of
// it at all). Final fix wave I6 retired the old diverged-authoring shape
// this fixture used to model (a draft edition over a landed seed
// revision): a landed revision IS the accepted revision under
// merge-signaled acceptance, so working-tree bytes diverging from it are
// a modified accepted revision and render read-only, never an authoring
// wall — see TestLoadBoard_DivergedOnDesignBranch_ReadOnlyWithNotice.
func newBoardFixture(t *testing.T) string {
	t.Helper()
	return buildAuthoringFixture(t, "design/"+boardFixtureName,
		map[string]string{
			".verdi/adr/0001-outbox-events.md": boardFixtureADR,
			".verdi/adr/0007-retry-budget.md":  boardFixtureADR2,
			".verdi/.gitignore":                "data/\n",
		},
		map[string]string{
			".verdi/specs/active/" + boardFixtureName + "/spec.md":     boardFixtureSpec,
			".verdi/specs/active/" + boardFixtureName + "/layout.json": boardFixtureLayout,
		})
}

// statuslessFixtureSpec is boardFixtureSpec with its status: line omitted —
// the only shape the CLI's design-start scaffold emits now (merge-signaled
// acceptance: lifecycle state is derived from Git, never persisted).
var statuslessFixtureSpec = strings.Replace(boardFixtureSpec, "status: draft\n", "", 1)

// newStatuslessBoardFixture builds the merge-signaled acceptance fixture:
// a STATUSLESS spec either committed only on its design branch (proposed —
// not reachable from main) or landed on main itself (exact-on-default).
// The default branch is provable in both shapes (origin/HEAD symref).
func newStatuslessBoardFixture(t *testing.T, onDesignBranch bool) string {
	t.Helper()
	mainFiles := map[string]string{
		".verdi/adr/0001-outbox-events.md": boardFixtureADR,
		".verdi/.gitignore":                "data/\n",
	}
	specFiles := map[string]string{
		".verdi/specs/active/" + boardFixtureName + "/spec.md":     statuslessFixtureSpec,
		".verdi/specs/active/" + boardFixtureName + "/layout.json": boardFixtureLayout,
	}
	if onDesignBranch {
		return buildAuthoringFixture(t, "design/"+boardFixtureName, mainFiles, specFiles)
	}
	for rel, content := range specFiles {
		mainFiles[rel] = content
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: mainFiles, Message: "seed statusless fixture"}})
	setDefaultBranchSymref(t, repo.Dir)
	return repo.Dir
}

// The step-3 acceptance pair (merge-signaled spec acceptance): the SAME
// statusless spec is an authoring board on its unmerged design branch and
// a read-only board once its exact bytes are reachable from the default
// branch — mode keyed by EFFECTIVE state, never by a persisted status:
// field (which the CLI no longer writes at all).
func TestBoardSpec_Statusless_DesignBranch_Authoring(t *testing.T) {
	root := newStatuslessBoardFixture(t, true)
	h := newBoardTestHandler(root)

	rec := getBoard(t, h, boardFixtureName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET statusless board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `data-board-mode="authoring"`) {
		t.Errorf("statusless spec on its design branch is not an authoring board:\n%s", body)
	}

	// The advertised design-start → serve → board-edit flow: a sticky
	// lands (mutable tier) and a spec edit lands (working tree).
	rec = postBoardAPI(t, h, boardFixtureName, "sticky", `{"text":"statusless authoring works","type":"comment"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sticky on statusless authoring board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if mrec, mout := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "a declined applicant sees the current reason [statusless]", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil); mrec.Code != http.StatusOK || mout.Result == nil {
		t.Fatalf("mutate_draft on statusless authoring board = %d, want a clean result\n%s", mrec.Code, mrec.Body.String())
	}
}

func TestBoardSpec_Statusless_ExactOnDefault_ReadOnly(t *testing.T) {
	root := newStatuslessBoardFixture(t, false)
	h := newBoardTestHandler(root)

	rec := getBoard(t, h, boardFixtureName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET statusless board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `data-board-mode="readonly"`) {
		t.Errorf("statusless spec exact on the default branch is not read-only:\n%s", body)
	}
	if mrec, _ := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "x", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil); mrec.Code != http.StatusForbidden {
		t.Fatalf("mutate_draft on exact-on-default statusless board = %d, want 403\n%s", mrec.Code, mrec.Body.String())
	}
}

// writeWorkingTreeSpec drops name's spec.md UNCOMMITTED into root's
// current working tree — the two default-branch-checkout negatives use it
// now that newBoardFixture's draft is committed on the design branch only.
func writeWorkingTreeSpec(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeStateResolver drives loadBoard's mode mapping without git — the
// StateResolver port's package-test double.
type fakeStateResolver struct {
	result specstate.Result
	err    error
}

func (f fakeStateResolver) Resolve(context.Context, string, specstate.Candidate) (specstate.Result, error) {
	return f.result, f.err
}

// TestLoadBoard_ModeByEffectiveState characterizes the full mode map over
// every specstate state (merge-signaled acceptance): PROPOSED on a design
// branch is the only authoring shape; accepted-pending-build, superseded,
// closed, and unproven all fail closed to read-only — and unproven's
// disclosures surface as board notices, never silence.
func TestLoadBoard_ModeByEffectiveState(t *testing.T) {
	root := newBoardFixture(t) // checked out on design/refi-test
	ctx := context.Background()

	unprovenDisclosure := "specstate: no default branch could be resolved for " + root
	tests := []struct {
		name       string
		result     specstate.Result
		wantMode   boardModeKind
		wantStatus string
		wantNotice string
	}{
		{"proposed on design branch is authoring", specstate.Result{State: specstate.Proposed, Relation: specstate.RelationNew}, modeAuthoring, "draft", ""},
		{"accepted-pending-build is read-only", specstate.Result{State: specstate.AcceptedPendingBuild, Relation: specstate.RelationExact}, modeReadOnly, "accepted-pending-build", ""},
		{"superseded is read-only", specstate.Result{State: specstate.Superseded, Relation: specstate.RelationExact}, modeReadOnly, "superseded", ""},
		{"closed is read-only", specstate.Result{State: specstate.Closed, Relation: specstate.RelationExact}, modeReadOnly, "closed", ""},
		{"unproven fails closed and discloses", specstate.Result{State: specstate.Unproven, Relation: specstate.RelationUnproven, Disclosures: []string{unprovenDisclosure}}, modeReadOnly, "unproven", unprovenDisclosure},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &boardSpecServer{root: root, state: fakeStateResolver{result: tc.result}}
			proj, _, _, _, err := s.loadBoard(ctx, boardFixtureName)
			if err != nil {
				t.Fatalf("loadBoard: %v", err)
			}
			if proj.Mode != tc.wantMode {
				t.Errorf("Mode = %q, want %q", proj.Mode, tc.wantMode)
			}
			if proj.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", proj.Status, tc.wantStatus)
			}
			if tc.wantNotice != "" {
				found := false
				for _, n := range proj.Notices {
					if n == tc.wantNotice {
						found = true
					}
				}
				if !found {
					t.Errorf("Notices = %q, want them to carry %q", proj.Notices, tc.wantNotice)
				}
			}
		})
	}

	// Negative: PROPOSED/NEW alone is not enough — on the default branch
	// itself (an uncommitted new spec dropped straight onto main, say) the
	// board stays read-only. The file is written UNCOMMITTED into main's
	// working tree: the fixture's draft is design-branch-only (I6's
	// RelationNew authoring shape), so main's checkout otherwise has
	// nothing at the path for loadBoard to read.
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatal(err)
	}
	writeWorkingTreeSpec(t, root, boardFixtureName, boardFixtureSpec)
	s := &boardSpecServer{root: root, state: fakeStateResolver{result: specstate.Result{State: specstate.Proposed, Relation: specstate.RelationNew}}}
	proj, _, _, _, err := s.loadBoard(ctx, boardFixtureName)
	if err != nil {
		t.Fatalf("loadBoard on main: %v", err)
	}
	if proj.Mode != modeReadOnly {
		t.Errorf("proposed content ON the default branch renders %q, want readonly", proj.Mode)
	}

	// Negative: a resolver failure is an operational error, never a
	// silently guessed mode.
	s = &boardSpecServer{root: root, state: fakeStateResolver{err: fmt.Errorf("boom")}}
	if _, _, _, _, err := s.loadBoard(ctx, boardFixtureName); err == nil {
		t.Fatal("loadBoard with a failing resolver returned no error")
	}
}

// TestLoadBoard_DivergedOnDesignBranch_ReadOnlyWithNotice (final fix wave
// I6): authoring requires State==Proposed AND Relation==RelationNew. A
// frozen legacy accepted spec whose working-tree bytes have been edited
// resolves Proposed/RelationDiverged — it is a MODIFIED ACCEPTED REVISION
// (VL-010 will refuse it), not a new proposal, so even on a design branch
// the wall renders read-only with a notice naming the divergence — never
// an editable authoring wall over bytes no merge can legally accept.
func TestLoadBoard_DivergedOnDesignBranch_ReadOnlyWithNotice(t *testing.T) {
	root := newBoardFixture(t) // checked out on design/refi-test
	ctx := context.Background()

	s := &boardSpecServer{root: root, state: fakeStateResolver{result: specstate.Result{
		State:    specstate.Proposed,
		Relation: specstate.RelationDiverged,
		Baseline: &specstate.Baseline{Path: ".verdi/specs/active/" + boardFixtureName + "/spec.md", Blob: "1111111111111111111111111111111111aaaa"},
	}}}
	proj, _, _, _, err := s.loadBoard(ctx, boardFixtureName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	if proj.Mode != modeReadOnly {
		t.Errorf("Mode = %q, want readonly (diverged bytes are a modified accepted revision, never authorable)", proj.Mode)
	}
	found := false
	for _, n := range proj.Notices {
		if strings.Contains(n, "diverge") && strings.Contains(n, "accepted revision") {
			found = true
		}
	}
	if !found {
		t.Errorf("Notices = %q, want one naming the divergence from the accepted revision", proj.Notices)
	}
}

// The vocabulary half of the step-3 acceptance: the projection's Status is
// the EFFECTIVE status (specstate's ArtifactStatus projection), so a
// statusless spec displays "draft" while proposed and
// "accepted-pending-build" once its exact bytes land on the default branch.
func TestLoadProjection_Statusless_EffectiveStatusVocabulary(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name           string
		onDesignBranch bool
		wantStatus     string
	}{
		{"proposed on design branch displays draft", true, "draft"},
		{"exact on default displays accepted-pending-build", false, "accepted-pending-build"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := newStatuslessBoardFixture(t, tc.onDesignBranch)
			proj, _, err := LoadProjection(ctx, root, boardFixtureName, nil, "", nil)
			if err != nil {
				t.Fatalf("LoadProjection: %v", err)
			}
			if proj.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q (effective state, never the persisted field)", proj.Status, tc.wantStatus)
			}
		})
	}
}

func getBoard(t *testing.T, h http.Handler, name string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/"+name, nil))
	return rec
}

func postBoardAPI(t *testing.T, h http.Handler, name, action, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/board/spec/"+name+"/api/"+action, strings.NewReader(body))
	h.ServeHTTP(rec, req)
	return rec
}

func TestBoardSpecPage_Authoring(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	rec := getBoard(t, h, boardFixtureName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`data-board-mode="authoring"`,
		`data-testid="placard-problem"`, "stale decline reasons",
		`data-testid="placard-outcome"`, "current decline flow",
		`data-testid="card-ac-1"`, `data-object-kind="acceptance-criterion"`,
		`data-testid="card-co-1"`, `data-object-kind="constraint"`,
		`data-testid="card-dc-2"`, `data-object-kind="decision"`,
		`data-edge-type="exempts" data-from="dc-1" data-to="adr/0001-outbox-events" data-layer="spec"`,
		`data-testid="ref-card-adr-0001-outbox-events"`,
		`data-testid="yarn-handle-dc-2"`,
		`data-testid="uncommitted-indicator" hidden`,
		`Commit &amp; push`,
		`Add sticky`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("board page missing %q", want)
		}
	}
	// The stored layout position passes through verbatim.
	if !strings.Contains(body, `data-testid="card-ac-1" data-id="ac-1" data-object-kind="acceptance-criterion" data-anchor="#ac-1" data-evidence="attestation" style="left:40px;top:60px"`) {
		t.Error("stored ac-1 position not rendered verbatim")
	}
}

// Owner directive (R4-I-35): cards never RENDER stacked, in any mode. A
// layout.json holding footprint-colliding positions (saved before the
// uniform-footprint enlargement — the accepted-pending-build regression
// fixture's exact geometry) renders resolved: the canonical-order first
// claimant keeps its stored spot, the collider is nudged — and rendering
// never writes layout.json (only a real drag writes).
func TestBoardSpecPage_CollidingStoredPositionsRenderResolved(t *testing.T) {
	root := newBoardFixture(t)
	layoutPath := filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "layout.json")
	colliding := `{
  "schema": "verdi.boardlayout/v1",
  "positions": { "ac-1": { "x": 40, "y": 20 }, "ac-2": { "x": 220, "y": 20 } }
}
`
	if err := os.WriteFile(layoutPath, []byte(colliding), 0o644); err != nil {
		t.Fatalf("seeding colliding layout: %v", err)
	}
	h := NewHandler(root)

	rec := getBoard(t, h, boardFixtureName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// First claimant: stored verbatim.
	if !strings.Contains(body, `data-testid="card-ac-1" data-id="ac-1" data-object-kind="acceptance-criterion" data-anchor="#ac-1" data-evidence="attestation" style="left:40px;top:20px"`) {
		t.Error("ac-1 (first claimant) not rendered at its stored position")
	}
	// The collider does NOT render at its stored, overlapping position.
	if strings.Contains(body, `data-id="ac-2" data-object-kind="acceptance-criterion" data-anchor="#ac-2" data-evidence="behavioral,attestation" style="left:220px;top:20px"`) {
		t.Error("ac-2 still renders stacked at its stored colliding position")
	}
	// Rendering never wrote the store: the colliding record is intact.
	after, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Fatalf("reading layout.json back: %v", err)
	}
	if string(after) != colliding {
		t.Errorf("rendering rewrote layout.json:\n got %s\nwant %s", after, colliding)
	}
}

func TestBoardSpecPage_Deterministic(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	first := getBoard(t, h, boardFixtureName)
	second := getBoard(t, h, boardFixtureName)
	if first.Body.String() != second.Body.String() {
		t.Fatal("two renders of the same inputs differ")
	}
}

func TestBoardSpecPage_NotFound(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	for _, name := range []string{"no-such-spec", "Bad_Name!"} {
		rec := getBoard(t, h, name)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET board %q = %d, want 404", name, rec.Code)
		}
	}
}

// TestErrBoardNotFound_MatchesThroughWrap proves spec/fail-loud ac-2's
// workbench half: every 404 branch in this package now checks
// errors.Is(err, ErrBoardNotFound), not err == ErrBoardNotFound (four
// sites: boardSpecPageHandler and boardSpecFragmentHandler here,
// boardspecapi.go's write-action handler, boardpin.go's pin-search
// handler). A bare `==` comparison silently degrades to a 500 the day
// any caller %w-wraps the sentinel with its own context (e.g. "loading
// board %q: %w") — this table proves the sentinel is still recognized
// once wrapped, and documents (via the parallel `==` check below) that
// the old comparison would NOT have recognized it, which is exactly the
// honesty gap this AC closes.
func TestErrBoardNotFound_MatchesThroughWrap(t *testing.T) {
	wrapped := fmt.Errorf("loading board %q: %w", "refi-test", ErrBoardNotFound)
	otherErr := errors.New("some unrelated operational failure")

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"bare sentinel", ErrBoardNotFound, true},
		{"%w-wrapped sentinel", wrapped, true},
		{"unrelated error", otherErr, false},
		{"nil error", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errors.Is(tt.err, ErrBoardNotFound); got != tt.want {
				t.Errorf("errors.Is(%v, ErrBoardNotFound) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}

	// The regression this AC guards against: a bare `==` comparison
	// recognizes the unwrapped sentinel but NOT the wrapped one — proving
	// why the four handler sites had to move off `==` and onto
	// errors.Is.
	if wrapped == ErrBoardNotFound { //nolint:errorlint // deliberate: proving the old, now-replaced comparison's failure mode
		t.Fatal("wrapped error unexpectedly == ErrBoardNotFound (sentinel identity should not survive wrapping)")
	}
	if !errors.Is(wrapped, ErrBoardNotFound) {
		t.Fatal("errors.Is failed to see through the %w-wrap — the fix this test guards would be broken")
	}
}

func TestBoardSpec_EditText(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)

	rec, out := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "a declined applicant sees the current reason [edited]", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)
	if rec.Code != http.StatusOK || out.Result == nil {
		t.Fatalf("mutate_draft(edit-ac) = %d\n%s", rec.Code, rec.Body.String())
	}
	if out.Projection == nil || !out.Projection.Dirty {
		t.Error("mutation projection dirty = false, want true (the spec working tree changed)")
	}

	specPath := filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `text: "a declined applicant sees the current reason [edited]"`) {
		t.Error("spec document does not carry the edit")
	}
	// The projection re-renders the edit.
	if !strings.Contains(getBoard(t, h, boardFixtureName).Body.String(), "[edited]") {
		t.Error("board does not re-render the edited text")
	}
}

func TestBoardSpec_EditText_Negative(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)

	// An unknown object refuses through the kernel: a typed VERDICT in the
	// body (§4.3 — classification is body content, not an HTTP status).
	t.Run("unknown object is a typed verdict with zero mutation", func(t *testing.T) {
		before, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
		rec, out := postMutate(t, h, root, boardFixtureName, []map[string]any{
			{"op": "edit-ac", "id": "ac-99", "text": "x", "evidence": []string{"attestation"}, "anchor": "#ac-99"},
		}, nil, nil)
		if rec.Code != http.StatusOK || out.Failure == nil || out.Failure.Classification != "verdict" {
			t.Fatalf("= %d %+v, want 200 with a verdict failure\n%s", rec.Code, out.Failure, rec.Body.String())
		}
		after, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
		if string(before) != string(after) {
			t.Error("a refused operation still mutated the spec")
		}
	})

	// The strict pre-application grammar (design §3.2): unknown fields,
	// duplicate keys, nulls, and trailing data fail BEFORE any application
	// call — HTTP 400, nothing decoded further, zero mutation.
	valid := mutateEnvelope(t, root, boardFixtureName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "x", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)
	for name, body := range map[string]string{
		"unknown envelope field": `{"bogus":1,` + valid[1:],
		"duplicate key":          `{"graduate_annotations":[],"graduate_annotations":[],` + valid[1:],
		"null value":             `{"graduate_annotations":null,` + valid[1:],
		"trailing data":          valid + `{}`,
		"wrong spec name":        strings.Replace(valid, `"spec":"spec/`+boardFixtureName+`"`, `"spec":"spec/other-spec"`, 1),
		"oversized body":         `{"graduate_annotations":["` + strings.Repeat("a", 1<<20) + `"],` + valid[1:],
	} {
		t.Run(name, func(t *testing.T) {
			rec := postBoardAPI(t, h, boardFixtureName, "mutate_draft", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("= %d, want 400\n%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestBoardSpec_WritesRefusedOutsideAuthoring(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)
	// On the default branch the draft is not authorable (05 §Workbench:
	// modes keyed by branch state). The draft is dropped UNCOMMITTED into
	// main's working tree — the fixture's committed draft lives only on
	// the design branch (I6's RelationNew authoring shape) — so this is a
	// real Proposed/new candidate whose only disqualifier is the branch.
	if err := gitx.Checkout(context.Background(), root, "main"); err != nil {
		t.Fatal(err)
	}
	writeWorkingTreeSpec(t, root, boardFixtureName, boardFixtureSpec)
	rec, _ := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "x", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("mutate_draft off design branch = %d, want 403", rec.Code)
	}
	if !strings.Contains(getBoard(t, h, boardFixtureName).Body.String(), `data-board-mode="readonly"`) {
		t.Error("board off design branch is not read-only")
	}
}

func TestBoardSpec_Edge(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)

	// An illegal source refuses through the kernel's validate-before-write
	// as a typed verdict (the closed link vocabulary is the kernel's).
	rec, out := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "add-link", "source": "ac-1", "type": "implements", "ref": "spec/" + boardFixtureName + "#ac-2"},
	}, nil, nil)
	if rec.Code != http.StatusOK || out.Failure == nil || out.Failure.Classification != "verdict" {
		t.Fatalf("illegal edge = %d %+v, want a typed verdict\n%s", rec.Code, out.Failure, rec.Body.String())
	}
	// An unknown link type is refused by the strict operation union at
	// decode — BEFORE any application call (design §3.2's closed grammar).
	rec2, _ := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "add-link", "source": "dc-2", "type": "blesses", "ref": "adr/0001-outbox-events"},
	}, nil, nil)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("unknown edge type = %d, want 400 (pre-application strict decode)\n%s", rec2.Code, rec2.Body.String())
	}

	// Legal: decision → ADR supersedes, one typed transaction.
	rec, out = postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "add-link", "source": "dc-2", "type": "supersedes", "ref": "adr/0001-outbox-events", "note": "drawn on the board"},
	}, nil, nil)
	if rec.Code != http.StatusOK || out.Result == nil {
		t.Fatalf("legal edge = %d\n%s", rec.Code, rec.Body.String())
	}
	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-edge-type="supersedes" data-from="dc-2" data-to="adr/0001-outbox-events" data-layer="spec"`) {
		t.Error("new edge does not project as spec-layer yarn")
	}
	// The chip carries the exact stored tuple for later removal/retype.
	if !strings.Contains(body, `data-stored-ref="adr/0001-outbox-events" data-note="drawn on the board"`) {
		t.Error("spec-layer chip does not carry its stored link tuple")
	}
}

func TestBoardSpec_StickyLifecycle(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)

	// The type is the author's explicit choice at creation (owner UAT
	// round 6, item 2 — amends R4-I-31's question-by-default).
	rec := postBoardAPI(t, h, boardFixtureName, "sticky", `{"text":"open question: partial refunds?","type":"question"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sticky = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Dirty {
		t.Error("a sticky dirtied the spec working tree (mutable zone must not)")
	}

	annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(annotations) != 1 || annotations[0].Type != artifact.AnnotationQuestion {
		t.Fatalf("annotations = %+v, want one question", annotations)
	}
	stickyID := annotations[0].ID

	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-testid="sticky-`+stickyID+`" data-id="`+stickyID+`" data-annotation-type="question"`) {
		t.Error("sticky does not render")
	}

	// Graduation: one typed add-question transaction plus the annotation
	// flip riding the same gesture (graduate_annotations).
	grec, gout := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "add-question", "id": "oq-1", "text": "open question: partial refunds?", "anchor": "#oq-1"},
	}, []string{stickyID}, nil)
	if grec.Code != http.StatusOK || gout.Result == nil {
		t.Fatalf("graduation mutate_draft = %d\n%s", grec.Code, grec.Body.String())
	}
	if gout.Projection == nil || !gout.Projection.Dirty {
		t.Error("graduation did not dirty the spec working tree (it IS a spec edit)")
	}

	specData, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if !strings.Contains(string(specData), `{ id: oq-1, text: "open question: partial refunds?", anchor: "#oq-1" }`) {
		t.Error("spec does not carry the graduated open question")
	}
	if !strings.Contains(string(specData), "\n## oq-1\n") {
		t.Error("spec body has no heading for the graduated object's anchor")
	}

	body = getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-testid="card-oq-1" data-id="oq-1" data-object-kind="open-question"`) {
		t.Error("graduated object does not render as a card")
	}
	if strings.Contains(body, `data-testid="sticky-`+stickyID+`"`) {
		t.Error("graduated sticky still renders")
	}
}

// Owner UAT round 6, item 2: the sticky's type is chosen at creation
// from the closed sticky-creatable enum; nothing defaults silently and
// unknown types fail closed (CLAUDE.md).
func TestBoardSpec_StickyTypes(t *testing.T) {
	creatable := []artifact.AnnotationType{
		artifact.AnnotationComment,
		artifact.AnnotationQuestion,
		artifact.AnnotationDecisionNeeded,
		artifact.AnnotationAgentTask,
	}
	for _, typ := range creatable {
		t.Run("creatable/"+string(typ), func(t *testing.T) {
			root := newBoardFixture(t)
			h := NewHandler(root)
			rec := postBoardAPI(t, h, boardFixtureName, "sticky", `{"text":"note for `+string(typ)+`","type":"`+string(typ)+`"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("sticky type %s = %d\n%s", typ, rec.Code, rec.Body.String())
			}
			annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
			if err != nil {
				t.Fatal(err)
			}
			if len(annotations) != 1 || annotations[0].Type != typ {
				t.Fatalf("annotations = %+v, want one %s", annotations, typ)
			}
			body := getBoard(t, h, boardFixtureName).Body.String()
			if !strings.Contains(body, `data-annotation-type="`+string(typ)+`"`) {
				t.Errorf("sticky does not render with its chosen type %s", typ)
			}
		})
	}

	for name, req := range map[string]string{
		"missing type":              `{"text":"typeless"}`,
		"unknown type fails closed": `{"text":"x","type":"todo"}`,
		"relates is not a sticky":   `{"text":"x","type":"relates"}`,
		"review is not creatable":   `{"text":"x","type":"review"}`,
	} {
		t.Run("negative/"+name, func(t *testing.T) {
			root := newBoardFixture(t)
			h := NewHandler(root)
			rec := postBoardAPI(t, h, boardFixtureName, "sticky", req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s = %d, want 400\n%s", name, rec.Code, rec.Body.String())
			}
			annotations, _ := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
			if len(annotations) != 0 {
				t.Errorf("a refused sticky still wrote a record: %+v", annotations)
			}
		})
	}
}

func TestBoardSpec_StickyGraduate_Negative(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	rec := postBoardAPI(t, h, boardFixtureName, "sticky-graduate", `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","kind":"open-question"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("graduating a missing sticky = %d, want 400", rec.Code)
	}
	rec = postBoardAPI(t, h, boardFixtureName, "sticky-graduate", `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","kind":"story"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("graduating to an unknown kind = %d, want 400", rec.Code)
	}
}

// Owner UAT round 6, item 4: clicking a reference card peeks the
// referenced artifact without leaving the board. The fragment carries
// title, kind, status, rendered body, and the full-page link; an
// unresolvable ref gets a DISCLOSED explanation, never a dead click and
// never a silent nothing.
func TestBoardSpec_RefPeek(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	get := func(query string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/"+boardFixtureName+"/peek"+query, nil))
		return rec
	}

	t.Run("resolvable ref renders the artifact", func(t *testing.T) {
		rec := get("?ref=adr/0001-outbox-events")
		if rec.Code != http.StatusOK {
			t.Fatalf("peek = %d\n%s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, want := range []string{
			"Outbox pattern for domain events (board fixture)", // title
			`class="peek-kind"`, ">adr<", // kind
			`class="peek-status"`, ">accepted<", // status
			"Domain events leave through the transactional outbox", // rendered body
			// The full-page link opens a NEW tab (owner directive: the
			// whole point of the peek is never losing the board).
			`href="/a/adr/0001-outbox-events" target="_blank" rel="noopener"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("peek fragment missing %q\n%s", want, body)
			}
		}
	})

	t.Run("pinned and fragment refs resolve to the same artifact", func(t *testing.T) {
		for _, ref := range []string{
			"adr/0001-outbox-events@78e3161594fb31fdad17f2ea8a96b52f33dbf0f3",
			"spec/" + boardFixtureName + "%23ac-1",
		} {
			rec := get("?ref=" + ref)
			if rec.Code != http.StatusOK {
				t.Fatalf("peek %s = %d", ref, rec.Code)
			}
			if strings.Contains(rec.Body.String(), "ref-peek-error") {
				t.Errorf("peek %s disclosed an error for a resolvable target\n%s", ref, rec.Body.String())
			}
		}
	})

	t.Run("unresolvable refs are disclosed, never silent", func(t *testing.T) {
		for name, ref := range map[string]string{
			"missing artifact": "adr/no-such-adr",
			"non-artifact ref": "jira:LOAN-1482",
		} {
			rec := get("?ref=" + ref)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: peek = %d, want 200 with a disclosed fragment", name, rec.Code)
			}
			body := rec.Body.String()
			if !strings.Contains(body, `data-testid="ref-peek-error"`) {
				t.Errorf("%s: no disclosed error state\n%s", name, body)
			}
		}
	})

	t.Run("negative: no ref, wrong method", func(t *testing.T) {
		if rec := get(""); rec.Code != http.StatusBadRequest {
			t.Errorf("peek without ref = %d, want 400", rec.Code)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/board/spec/"+boardFixtureName+"/peek?ref=adr/0001-outbox-events", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST peek = %d, want 405", rec.Code)
		}
	})

	t.Run("deterministic fragment", func(t *testing.T) {
		a := get("?ref=adr/0001-outbox-events").Body.String()
		b := get("?ref=adr/0001-outbox-events").Body.String()
		if a != b {
			t.Error("two peeks of the same ref differ")
		}
	})
}

// Owner UAT round 6, item 3(a)/(b): a scratch sticky or an untyped
// relates thread dies from the mutable stream — never touching the spec
// document — through the board's own affordance.
func TestBoardSpec_AnnotationDelete(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, boardFixtureName, "sticky", `{"text":"a doomed note","type":"comment"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sticky = %d\n%s", rec.Code, rec.Body.String())
	}
	annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if err != nil || len(annotations) != 1 {
		t.Fatalf("annotations = %+v, err %v", annotations, err)
	}
	id := annotations[0].ID

	rec = postBoardAPI(t, h, boardFixtureName, "annotation-delete", `{"id":"`+id+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("annotation-delete = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Dirty {
		t.Error("deleting a sticky dirtied the spec working tree (mutable zone only)")
	}
	annotations, _ = boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if len(annotations) != 0 {
		t.Fatalf("record still present after delete: %+v", annotations)
	}
	if strings.Contains(getBoard(t, h, boardFixtureName).Body.String(), `data-testid="sticky-`+id+`"`) {
		t.Error("deleted sticky still renders")
	}

	// Negative: an id this board does not present is refused — and the
	// refusal names WHICH annotation and WHICH board (owner bug
	// 2026-07-19: "annotations were missing, unclear where" — the
	// message must carry the where, not just the id).
	rec = postBoardAPI(t, h, boardFixtureName, "annotation-delete", `{"id":"a-01J8Z0K9ZZZZZZZZZZZZZZZZZZ"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("deleting a foreign annotation = %d, want 400", rec.Code)
	}
	for _, want := range []string{"a-01J8Z0K9ZZZZZZZZZZZZZZZZZZ", "spec/" + boardFixtureName} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("delete refusal does not name %q: %s", want, rec.Body.String())
		}
	}
}

// The sticky-position refusal names WHICH sticky and WHICH board too —
// the drag-path twin of the delete refusal above (owner bug 2026-07-19).
func TestBoardSpec_StickyPositionRefusalNamesItsBoard(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, boardFixtureName, "sticky-position", `{"id":"a-01J8Z0K9ZZZZZZZZZZZZZZZZZZ","x":10,"y":20}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("sticky-position on a missing sticky = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"a-01J8Z0K9ZZZZZZZZZZZZZZZZZZ", "spec/" + boardFixtureName} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("position refusal does not name %q: %s", want, rec.Body.String())
		}
	}
}

// Owner UAT round 6, item 3(c): removing a spec-layer typed edge is the
// exact inverse of drawing it — an ordinary spec edit through the
// splice write path.
func TestBoardSpec_EdgeDelete(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)

	// The fixture's dc-1 carries the declared exempts edge; the removal
	// addresses its EXACT stored tuple (ref + note verbatim).
	rec, out := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "remove-link", "source": "dc-1", "type": "exempts", "ref": "adr/0001-outbox-events", "note": "async by design"},
	}, nil, nil)
	if rec.Code != http.StatusOK || out.Result == nil {
		t.Fatalf("edge removal = %d\n%s", rec.Code, rec.Body.String())
	}
	if out.Projection == nil || !out.Projection.Dirty {
		t.Error("removing a declared edge did not dirty the working tree (it IS a spec edit)")
	}
	specData, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if strings.Contains(string(specData), "exempts") {
		t.Errorf("spec still carries the removed edge:\n%s", specData)
	}
	if strings.Contains(getBoard(t, h, boardFixtureName).Body.String(), `data-edge-type="exempts" data-from="dc-1"`) {
		t.Error("removed edge still projects as yarn")
	}

	// Negatives refuse through the kernel as typed verdicts with zero
	// mutation: the tuple is already gone, or the source never carried it.
	for name, op := range map[string]map[string]any{
		"already removed":   {"op": "remove-link", "source": "dc-1", "type": "exempts", "ref": "adr/0001-outbox-events", "note": "async by design"},
		"undeclared source": {"op": "remove-link", "source": "zz-9", "type": "exempts", "ref": "adr/0001-outbox-events"},
	} {
		rec, out := postMutate(t, h, root, boardFixtureName, []map[string]any{op}, nil, nil)
		if rec.Code != http.StatusOK || out.Failure == nil || out.Failure.Classification != "verdict" {
			t.Errorf("%s = %d %+v, want a typed verdict\n%s", name, rec.Code, out.Failure, rec.Body.String())
		}
	}
}

// Owner directive (round 6 UAT follow-up): the relationship's type is
// updatable in place — one atomic splice transaction, ref and note
// surviving verbatim.
func TestBoardSpec_EdgeRetype(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)

	// Retype = remove + add in ONE atomic transaction, the stored ref and
	// note surviving verbatim (task-2 brief adjudication).
	rec, out := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "remove-link", "source": "dc-1", "type": "exempts", "ref": "adr/0001-outbox-events", "note": "async by design"},
		{"op": "add-link", "source": "dc-1", "type": "supersedes", "ref": "adr/0001-outbox-events", "note": "async by design"},
	}, nil, nil)
	if rec.Code != http.StatusOK || out.Result == nil {
		t.Fatalf("edge retype = %d\n%s", rec.Code, rec.Body.String())
	}
	specData, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if !strings.Contains(string(specData), `note: "async by design"`) || !strings.Contains(string(specData), "type: supersedes") {
		t.Errorf("spec does not carry the retyped edge with ref and note verbatim:\n%s", specData)
	}
	if strings.Contains(string(specData), "type: exempts") {
		t.Errorf("spec still carries the old edge type:\n%s", specData)
	}
	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-edge-type="supersedes" data-from="dc-1"`) {
		t.Error("retyped edge does not project")
	}
	if strings.Contains(body, `data-edge-type="exempts" data-from="dc-1"`) {
		t.Error("old edge type still projects")
	}

	// A batch whose second half fails lands NOTHING (the ordered batch is
	// atomic): removing the surviving supersedes edge twice refuses on the
	// second operation, and the spec keeps the supersedes edge.
	rec, out = postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "remove-link", "source": "dc-1", "type": "supersedes", "ref": "adr/0001-outbox-events", "note": "async by design"},
		{"op": "remove-link", "source": "dc-1", "type": "supersedes", "ref": "adr/0001-outbox-events", "note": "async by design"},
	}, nil, nil)
	if rec.Code != http.StatusOK || out.Failure == nil || out.Failure.Classification != "verdict" {
		t.Fatalf("atomic batch refusal = %d %+v, want a typed verdict\n%s", rec.Code, out.Failure, rec.Body.String())
	}
	specData, _ = os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if !strings.Contains(string(specData), "type: supersedes") {
		t.Error("the refused batch removed the edge anyway (batch atomicity broken)")
	}
}

// Deletion and retype affordances exist ONLY in authoring mode: review
// is a mirror, read-only a document (05 §Workbench). The renderer is
// mode-gated, provable directly on the projection render; the e2e suite
// additionally proves the absence on a live read-only board.
func TestBoardSpec_AffordancesAreAuthoringOnly(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	authoringBody := getBoard(t, h, boardFixtureName).Body.String()
	for _, want := range []string{`class="delete-btn"`, `data-retype`} {
		if !strings.Contains(authoringBody, want) {
			t.Errorf("authoring board missing %s", want)
		}
	}

	proj := &BoardProjection{
		Spec: boardFixtureName, Mode: modeReadOnly,
		Cards:    []cardView{{ID: "dc-1", Kind: "decision", Text: "x"}},
		Edges:    []edgeView{{Type: "exempts", From: "dc-1", To: "adr/0001-outbox-events", Layer: "spec"}},
		Stickies: []scratchStickyView{{ID: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Type: "comment", Body: "b"}},
	}
	for _, mode := range []boardModeKind{modeReadOnly, modeReview} {
		proj.Mode = mode
		frozen := renderBoardRegion(proj, &boardGitState{}, testASDView())
		for _, banned := range []string{`class="delete-btn"`, `data-retype`, `class="graduate-btn"`} {
			if strings.Contains(frozen, banned) {
				t.Errorf("%s board renders %s", mode, banned)
			}
		}
	}
}

func TestBoardSpec_RelatesLifecycle(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)

	rec := postBoardAPI(t, h, boardFixtureName, "relates", `{"from":"dc-2","to":"adr/0001-outbox-events"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("relates = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Dirty {
		t.Error("a relates thread dirtied the spec working tree")
	}

	annotations, _ := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if len(annotations) != 1 || annotations[0].Type != artifact.AnnotationRelates {
		t.Fatalf("annotations = %+v, want one relates", annotations)
	}
	threadID := annotations[0].ID

	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-edge-type="relates" data-from="dc-2" data-to="adr/0001-outbox-events" data-layer="annotation" data-annotation-id="`+threadID+`"`) {
		t.Error("relates thread does not render as annotation-layer yarn")
	}

	// Graduation to a typed edge: one add-link transaction, the thread
	// flip riding the same gesture.
	grec, gout := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "add-link", "source": "dc-2", "type": "exempts", "ref": "adr/0001-outbox-events", "note": "confirmed on the board"},
	}, []string{threadID}, nil)
	if grec.Code != http.StatusOK || gout.Result == nil {
		t.Fatalf("relates graduation = %d\n%s", grec.Code, grec.Body.String())
	}
	body = getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-edge-type="exempts" data-from="dc-2" data-to="adr/0001-outbox-events" data-layer="spec"`) {
		t.Error("graduated thread does not project as spec-layer yarn")
	}
	if strings.Contains(body, `data-annotation-id="`+threadID+`"`) {
		t.Error("graduated thread still renders as annotation yarn")
	}

	// Illegal graduation is refused server-side.
	rec = postBoardAPI(t, h, boardFixtureName, "relates", `{"from":"ac-1","to":"ac-2"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("second relates = %d", rec.Code)
	}
	annotations, _ = boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	var acThread string
	for _, a := range annotations {
		if a.ID != threadID {
			acThread = a.ID
		}
	}
	grec, gout = postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "add-link", "source": "ac-1", "type": "implements", "ref": "spec/" + boardFixtureName + "#ac-2"},
	}, []string{acThread}, nil)
	if grec.Code != http.StatusOK || gout.Failure == nil || gout.Failure.Classification != "verdict" {
		t.Fatalf("illegal graduation = %d %+v, want a typed verdict", grec.Code, gout.Failure)
	}
	// The refused transaction flipped NOTHING: the thread is still live.
	annotations, _ = boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	for _, a := range annotations {
		if a.ID == acThread && a.Status != artifact.AnnotationOpen {
			t.Errorf("refused graduation still flipped thread %s to %s", acThread, a.Status)
		}
	}
}

func TestBoardSpec_Position(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	// A drop on open canvas is stored verbatim.
	rec := postBoardAPI(t, h, boardFixtureName, "position", `{"id":"ac-2","x":613,"y":500}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("position = %d\n%s", rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "layout.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, `"ac-2":{"x":613,"y":500}`) {
		t.Errorf("layout.json missing the stored position: %s", got)
	}
	if !strings.Contains(got, `"ac-1":{"x":40,"y":60}`) {
		t.Errorf("layout.json lost the pre-existing stored position: %s", got)
	}
	if !strings.Contains(getBoard(t, h, boardFixtureName).Body.String(), `style="left:613px;top:500px"`) {
		t.Error("stored position does not render")
	}

	// A drop overlapping another card's footprint resolves to the nearest
	// non-overlapping position (collision-free by construction): (613,218)
	// lands on dc-2's generated slot (496,216), so the drop slides out to
	// the right of dc-2's footprint — and ONLY the dragged card's stored
	// position changes.
	rec = postBoardAPI(t, h, boardFixtureName, "position", `{"id":"ac-2","x":613,"y":218}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("position (colliding) = %d\n%s", rec.Code, rec.Body.String())
	}
	data, err = os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "layout.json"))
	if err != nil {
		t.Fatal(err)
	}
	got = string(data)
	if !strings.Contains(got, `"ac-2":{"x":708,"y":218}`) {
		t.Errorf("colliding drop not resolved to the free position right of dc-2: %s", got)
	}
	if !strings.Contains(got, `"ac-1":{"x":40,"y":60}`) {
		t.Errorf("drop resolution touched another card's stored position: %s", got)
	}

	// A non-object key is refused (VL-018: keys must resolve).
	rec = postBoardAPI(t, h, boardFixtureName, "position", `{"id":"zz-9","x":1,"y":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("position for unknown id = %d, want 400", rec.Code)
	}
}

func TestBoardSpec_GitCommitAndSwitch(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)
	ctx := context.Background()

	// Dirty the tree through the board's own write path.
	if rec, out := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "edit-constraint", "id": "co-1", "text": "notices never name internal scores [amended]", "anchor": "#co-1"},
	}, nil, nil); rec.Code != http.StatusOK || out.Result == nil {
		t.Fatalf("dirtying edit = %d\n%s", rec.Code, rec.Body.String())
	}
	dirty, _ := gitx.StatusDirty(ctx, root)
	if !dirty {
		t.Fatal("fixture not dirty after edit")
	}

	// The guard: a dirty tree refuses to switch.
	rec := postBoardAPI(t, h, boardFixtureName, "git-switch", `{"branch":"main"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("git-switch (dirty) = %d, want 409", rec.Code)
	}

	// A message is required.
	rec = postBoardAPI(t, h, boardFixtureName, "git-commit", `{"message":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("git-commit without message = %d, want 400", rec.Code)
	}

	// Commit clears the indicator's signal. (No origin remote here: the
	// commit is still durable; push engages only when origin exists.)
	rec = postBoardAPI(t, h, boardFixtureName, "git-commit", `{"message":"board: amend co-1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("git-commit = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Dirty {
		t.Error("tree still dirty after the board commit")
	}

	// Clean: the switch works.
	rec = postBoardAPI(t, h, boardFixtureName, "git-switch", `{"branch":"main"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("git-switch (clean) = %d\n%s", rec.Code, rec.Body.String())
	}
	branch, _ := gitx.CurrentBranch(ctx, root)
	if branch != "main" {
		t.Fatalf("branch after switch = %q, want main", branch)
	}
}

// fakeFeed is the CommentFeed test double review mode is built against
// (V1-P6 "Stubs": the real forge port is V1-P7's; the wave close adapts
// it over this interface).
type fakeFeed struct {
	feeds map[string][]MRComment
}

func (f fakeFeed) ListMRComments(_ context.Context, specName string) ([]MRComment, bool, error) {
	comments, ok := f.feeds[specName]
	return comments, ok, nil
}

func TestBoardSpec_ReviewMode(t *testing.T) {
	root := newBoardFixture(t)
	feed := fakeFeed{feeds: map[string][]MRComment{
		boardFixtureName: {
			{ID: "1", Author: "alice", Body: "[vd:ac-2] this outcome AC reads implementation-scoped — reword?"},
			{ID: "2", Author: "bob", Body: "overall direction looks right"},
			{ID: "3", Author: "carol", Body: "[vd:zz-99] does this still apply after the split?", Resolved: true},
		},
	}}
	h := NewHandlerWith(root, Deps{CommentFeed: feed, Design: testDesignBridge{}})

	rec := getBoard(t, h, boardFixtureName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET review board = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-board-mode="review"`) {
		t.Fatal("board with an open MR is not in review mode")
	}
	// The resolvable token anchors to its object's card.
	acCard := body[strings.Index(body, `data-testid="card-ac-2"`):]
	acCard = acCard[:strings.Index(acCard, `data-testid="card-co-1"`)]
	if !strings.Contains(acCard, `data-annotation-type="review" data-anchor="ac-2"`) {
		t.Error("anchored comment does not ride its object's card")
	}
	// Token-free and unresolvable-token comments land in the tray.
	if !strings.Contains(body, `aria-label="Inbox tray"`) {
		t.Fatal("no inbox tray")
	}
	tray := body[strings.Index(body, `aria-label="Inbox tray"`):]
	for _, want := range []string{"overall direction looks right", "[vd:zz-99] does this still apply"} {
		if !strings.Contains(tray, want) {
			t.Errorf("inbox tray missing %q", want)
		}
	}
	// Conservation: the whole feed renders.
	if got := strings.Count(body, `data-annotation-type="review"`); got != 3 {
		t.Errorf("review stickies = %d, want 3 (never dropped)", got)
	}
	// A mirror, not an editing surface.
	for _, absent := range []string{"Commit &amp; push", "Add sticky", "yarn-handle", "graduate-btn"} {
		if strings.Contains(body, absent) {
			t.Errorf("review mode still renders %q", absent)
		}
	}
	// And no write goes through: the review mirror refuses the typed
	// mutation at the adapter (CO-1's review-mode refusal is board
	// knowledge; the kernel cannot see the open MR).
	recW, _ := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "x", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)
	if recW.Code != http.StatusForbidden {
		t.Fatalf("write in review mode = %d, want 403", recW.Code)
	}
}

func TestCommentToken(t *testing.T) {
	tests := []struct {
		body, want string
	}{
		{"[vd:ac-2] reword this", "ac-2"},
		{"no token here", ""},
		{"mid-body [vd:ac-2] token does not anchor", ""},
		{"[vd:zz-99] unresolvable is still a token", "zz-99"},
		{"[vd:] empty", ""},
	}
	for _, tc := range tests {
		if got := commentToken(tc.body); got != tc.want {
			t.Errorf("commentToken(%q) = %q, want %q", tc.body, got, tc.want)
		}
	}
}

func TestLegalEdgeTypes(t *testing.T) {
	tests := []struct {
		source, target string
		want           int
	}{
		{"decision", "adr", 2},
		{"decision", "decision", 2},
		{"decision", "spec-fragment", 2},
		{"acceptance-criterion", "acceptance-criterion", 0},
		{"constraint", "adr", 0},
		{"open-question", "decision", 0},
	}
	for _, tc := range tests {
		if got := len(legalEdgeTypes(tc.source, tc.target)); got != tc.want {
			t.Errorf("legalEdgeTypes(%s, %s) = %d types, want %d", tc.source, tc.target, got, tc.want)
		}
	}
}

// TestProtoYarnTargetKind pins dc-5's type-directed yarn table — the one
// rule the picker (routeProtoYarn) and the server (checkProtoYarnLegal)
// share — on both poles: the two proto-sticky types name their single
// legal target kind, every other sticky type is untyped (no rule, so the
// untyped relates vocabulary stays open).
func TestProtoYarnTargetKind(t *testing.T) {
	tests := []struct {
		stickyType string
		want       string
		wantTyped  bool
	}{
		{"story", "acceptance-criterion", true},
		{"spike", "open-question", true},
		{"comment", "", false},
		{"question", "", false},
		{"decision-needed", "", false},
		{"relates", "", false},
		{"", "", false},
		{"Story", "", false}, // enum values are exact; unknown fails closed
	}
	for _, tc := range tests {
		got, typed := protoYarnTargetKind(tc.stickyType)
		if got != tc.want || typed != tc.wantTyped {
			t.Errorf("protoYarnTargetKind(%q) = (%q, %v), want (%q, %v)", tc.stickyType, got, typed, tc.want, tc.wantTyped)
		}
	}
}

// erroringFeed is a CommentFeed whose call always fails — the
// configured-but-unreachable forge from the failure side (I-2).
type erroringFeed struct{}

// errFeedUnreachable is the failure erroringFeed reports; the tests below
// re-render it through the shared seam to derive the notice they expect,
// so no test re-authors the disclosure vocabulary either
// (spec/disclosure-seam-v2 ac-1).
var errFeedUnreachable = errors.New("forge unreachable: dial tcp 10.0.0.1:443: connect: connection refused")

func (erroringFeed) ListMRComments(context.Context, string) ([]MRComment, bool, error) {
	return nil, false, errFeedUnreachable
}

// TestBoard_FeedError_DegradesNotBlocks proves I-2: a feed error on an
// authoring board renders 200 with the board content intact PLUS a
// disclosed notice — never a 500, never a blocked page. The feed is a
// review-mode-only input; authoring must always render (04 §Semantics'
// degradation posture).
func TestBoard_FeedError_DegradesNotBlocks(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandlerWith(root, Deps{CommentFeed: erroringFeed{}})

	rec := getBoard(t, h, boardFixtureName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET board with a failing feed = %d, want 200 (never block on the feed)\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// Board content is intact — authoring mode rendered fully.
	if !strings.Contains(body, `data-board-mode="authoring"`) {
		t.Error("authoring board did not render authoring mode on a feed error")
	}
	if !strings.Contains(body, `data-testid="card-ac-1"`) {
		t.Error("authoring board content missing on a feed error (should be fully rendered)")
	}
	// The failure is disclosed, never silent — and disclosed through the
	// shared internal/disclosure seam, never a locally authored string
	// (spec/disclosure-seam-v2 ac-1).
	wantNotice := disclosure.Render(disclosure.ReviewUnavailableTransport(errFeedUnreachable))
	if !strings.Contains(body, `data-testid="board-notice"`) || !strings.Contains(body, wantNotice) {
		t.Errorf("feed error not disclosed as a board notice through the shared seam (%q):\n%s", wantNotice, body)
	}
	// The fragment surface degrades identically (post-mutation re-render).
	frag := httptest.NewRecorder()
	h.ServeHTTP(frag, httptest.NewRequest(http.MethodGet, "/board/spec/"+boardFixtureName+"/fragment", nil))
	if frag.Code != http.StatusOK || !strings.Contains(frag.Body.String(), wantNotice) {
		t.Errorf("fragment on feed error = %d, want 200 with the disclosure notice", frag.Code)
	}
}

// TestBoard_ConfiguredButUnavailable_Disclosed proves I-1(b) state 3: a
// forge configured but with no live feed (Deps.ReviewUnavailable set)
// discloses on the board chrome rather than rendering as silently
// not-under-review.
func TestBoard_ConfiguredButUnavailable_Disclosed(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandlerWith(root, Deps{ReviewUnavailable: `forge "gitlab" is configured but no credentials are available to reach it; review state cannot be shown`})

	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-testid="board-notice"`) || !strings.Contains(body, "review state cannot be shown") {
		t.Errorf("configured-but-unavailable forge not disclosed on the board:\n%s", body)
	}
}

// TestBoard_NoForge_Silent proves I-1(b) state 1: with no feed and no
// ReviewUnavailable, the board says nothing about review — an unconfigured
// integration legitimately stays silent (no review-specific disclosure).
func TestBoard_NoForge_Silent(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandlerWith(root, Deps{})

	body := getBoard(t, h, boardFixtureName).Body.String()
	for _, absent := range []string{"could not be consulted", "review state cannot be shown"} {
		if strings.Contains(body, absent) {
			t.Errorf("unconfigured forge should be silent, but board contains %q", absent)
		}
	}
}

// TestBoard_DefaultBranchUnresolved_HonestStoryAndRemedy (fix round 2,
// finding 3 — supersedes the old assumed-"main" disclosure): a repo with
// NO resolvable default branch (no CI_DEFAULT_BRANCH, no origin/HEAD, no
// D6-6 remote-tracking fallback) renders ONE consistent story — the spec's
// effective state is unproven, the board fails closed to read-only, and
// the chrome discloses what was tried plus the remedy. The old
// contradictory second story ("assuming main" while everything renders
// read-only anyway) must be gone.
func TestBoard_DefaultBranchUnresolved_HonestStoryAndRemedy(t *testing.T) {
	// "No CI_DEFAULT_BRANCH" is half this test's premise, so establish it
	// rather than inherit it: a runner that exports CI_DEFAULT_BRANCH
	// would otherwise make the default branch resolvable and dissolve the
	// case under test.
	neutralizeCIEnv(t)
	// The pre-symref fixture shape on purpose: draft committed on main,
	// design branch cut, origin/HEAD never configured.
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + boardFixtureName + "/spec.md": boardFixtureSpec,
			".verdi/adr/0001-outbox-events.md":                     boardFixtureADR,
			".verdi/.gitignore":                                    "data/\n",
		},
		Message: "seed board fixture",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/"+boardFixtureName); err != nil {
		t.Fatalf("checkout design branch: %v", err)
	}
	h := NewHandler(repo.Dir)

	body := getBoard(t, h, boardFixtureName).Body.String()
	// The honest story: what was tried (the D6-6 chain) and the remedy.
	for _, want := range []string{
		`data-testid="board-notice"`,
		"default branch could not be resolved",
		"CI_DEFAULT_BRANCH",
		"origin/HEAD",
		"git remote set-head origin",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("unresolved-default-branch notice missing %q:\n%s", want, body)
		}
	}
	// The contradictory assumed-main claim is gone from this path.
	if strings.Contains(body, `assuming "main"`) {
		t.Errorf("board still claims an assumed \"main\" default beside the unproven story:\n%s", body)
	}
	// Unproven fails closed to read-only, and the missing witness is
	// disclosed in the chrome, never silently rendered as either mode.
	if !strings.Contains(body, `data-board-mode="readonly"`) {
		t.Errorf("board with an unprovable default branch is not read-only:\n%s", body)
	}
	if !strings.Contains(body, "no default branch could be resolved") {
		t.Errorf("unproven state's disclosure missing from the board chrome:\n%s", body)
	}
}

// TestBoardSpec_ActiveZoneLegacySuperseded_BadgeAndDisclosure (fix round
// 2, finding 2): an ACTIVE-zone spec whose landed bytes carry a legacy
// explicit `status: superseded` projects Superseded WITH a compatibility
// disclosure (the merged projector's legacy-terminal rows) — the board
// header wears the terminal badge, the wall is the sealed read-only
// record, and the compatibility disclosure surfaces as a board notice.
func TestBoardSpec_ActiveZoneLegacySuperseded_BadgeAndDisclosure(t *testing.T) {
	legacySuperseded := strings.Replace(boardFixtureSpec, "status: draft\n",
		"status: superseded\nfrozen: { at: 2026-03-01, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }\n", 1)
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + boardFixtureName + "/spec.md": legacySuperseded,
			".verdi/adr/0001-outbox-events.md":                     boardFixtureADR,
			".verdi/.gitignore":                                    "data/\n",
		},
		Message: "seed legacy-superseded fixture",
	}})
	setDefaultBranchSymref(t, repo.Dir)
	h := NewHandler(repo.Dir)

	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-board-mode="readonly"`) {
		t.Errorf("legacy-superseded active-zone board is not read-only:\n%s", body)
	}
	if !strings.Contains(body, `data-testid="board-status-badge"`) || !strings.Contains(body, `badge-superseded`) {
		t.Errorf("board head missing the superseded status badge:\n%s", body)
	}
	if !strings.Contains(body, "compatibility reading") {
		t.Errorf("legacy-terminal compatibility disclosure not surfaced in the board chrome:\n%s", body)
	}
}

// TestBoard_ConcurrentMutations_BothLand proves M-2: two racing board
// mutations against the same spec both land (no lost update). Without the
// per-server writeMu the second read-modify-write of spec.md could
// clobber the first (last writer wins).
// TestBoard_ConcurrentMutations_NeverLoseAnUpdate is DC-5's migrated
// witness: two concurrent typed mutations against the SAME base digest
// serialize through the kernel — exactly one lands, the other receives
// the structured stale refusal naming the current digest, and no update
// is ever silently merged or lost.
func TestBoard_ConcurrentMutations_NeverLoseAnUpdate(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)

	// Both envelopes read the SAME base (built before either posts).
	envelopes := []string{
		mutateEnvelope(t, root, boardFixtureName, []map[string]any{
			{"op": "edit-ac", "id": "ac-1", "text": "a declined applicant sees the current reason [edit-A]", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
		}, nil, nil),
		mutateEnvelope(t, root, boardFixtureName, []map[string]any{
			{"op": "edit-ac", "id": "ac-2", "text": "a decline reverses within one day [edit-B]", "evidence": []string{"behavioral", "attestation"}, "anchor": "#ac-2"},
		}, nil, nil),
	}
	results := make([]mutateOutcome, len(envelopes))
	codes := make([]int, len(envelopes))
	var wg sync.WaitGroup
	for i, envelope := range envelopes {
		wg.Add(1)
		go func(i int, envelope string) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/board/spec/"+boardFixtureName+"/api/mutate_draft", strings.NewReader(envelope))
			h.ServeHTTP(rec, req)
			codes[i] = rec.Code
			_ = json.Unmarshal(rec.Body.Bytes(), &results[i])
		}(i, envelope)
	}
	wg.Wait()

	landed, stale := 0, 0
	for i := range results {
		if codes[i] != http.StatusOK {
			t.Fatalf("concurrent mutate %d = %d\n", i, codes[i])
		}
		if results[i].Result != nil {
			landed++
		}
		if results[i].Stale != nil {
			stale++
			if !strings.Contains(string(results[i].Stale), "current_digest") {
				t.Errorf("stale refusal carries no current digest: %s", results[i].Stale)
			}
		}
	}
	if landed != 1 || stale != 1 {
		t.Fatalf("landed=%d stale=%d, want exactly one of each (DC-5: refuse, never merge)", landed, stale)
	}

	specData, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	spec := string(specData)
	got := 0
	for _, want := range []string{"[edit-A]", "[edit-B]"} {
		if strings.Contains(spec, want) {
			got++
		}
	}
	if got != 1 {
		t.Errorf("spec carries %d of the two edits, want exactly the winner:\n%s", got, spec)
	}
}

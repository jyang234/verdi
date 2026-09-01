package workbench

// Review-fix coverage (Wave 6 Task 2, round 1) for the browser design
// adapter's honesty seams:
//
//   - I-1: a proposed draft served from a checkout whose branch is NOT
//     the kernel's design/<spec-name> branch must not present the live
//     domain-authoring surface (typed forms, card editors, spec-edge
//     removal) — the kernel (draftmutation.AuthorizeState) refuses every
//     such write with state-forbidden, so offering the surface is
//     dishonest. The scratch tier (stickies, threads, positions) stays
//     live: those writes land in the mutable zone and the kernel does not
//     govern them.
//   - I-2: the canonical-checkout memo must never poison itself with a
//     transient (request-scoped) failure for the server's lifetime.
//   - I-3: a remove-object transaction must not land when declared links
//     the wall does NOT render (the six non-yarn link types, or the
//     document-level links: block) still name the removed fragment — the
//     legacy object-trash enumerated ALL declared links (VL-003).

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- I-2: the canonical-checkout memo -----------------------------------

// TestCachedCanonicalCheckout_RecoversAfterCanceledContext pins the fix
// for the poisoned sync.Once: the first resolution runs under a browser
// request's context — if that request is aborted, the failure must NOT be
// memoized for the server's lifetime. Success only is cached; the third
// call proves the cache (a canceled context succeeds because no git runs).
func TestCachedCanonicalCheckout_RecoversAfterCanceledContext(t *testing.T) {
	root := newBoardFixture(t)
	s := &boardSpecServer{root: root}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.cachedCanonicalCheckout(canceled, boardFixtureName); err == nil {
		t.Fatal("first resolution under a canceled context succeeded; the fixture cannot exercise the retry path")
	}

	got, err := s.cachedCanonicalCheckout(context.Background(), boardFixtureName)
	if err != nil {
		t.Fatalf("second resolution with a live context failed — the first failure was cached: %v", err)
	}
	if got == "" {
		t.Fatal("second resolution returned an empty checkout")
	}

	// The success is cached: a canceled context now succeeds because the
	// memo answers without running git.
	cached, err := s.cachedCanonicalCheckout(canceled, boardFixtureName)
	if err != nil {
		t.Fatalf("third resolution (cache hit) failed: %v", err)
	}
	if cached != got {
		t.Fatalf("cache returned %q, want the resolved %q", cached, got)
	}
}

// --- I-1: wrong-branch draft posture ------------------------------------

// newWrongBranchFixture commits the standard draft spec on a branch that
// is NOT the kernel's design/<spec-name> branch: the state projects
// Proposed/RelationNew (authoring's scratch tier), but every domain
// mutation is state-forbidden.
func newWrongBranchFixture(t *testing.T) string {
	t.Helper()
	return buildAuthoringFixture(t, "design/another-lane",
		map[string]string{".verdi/.gitignore": "data/\n"},
		map[string]string{
			".verdi/specs/active/" + boardFixtureName + "/spec.md": boardFixtureSpec,
		})
}

func TestBoardSpec_WrongBranchDraft_RefusesDomainSurface(t *testing.T) {
	root := newWrongBranchFixture(t)
	s := &boardSpecServer{root: root, design: testDesignBridge{}}

	proj, _, _, _, err := s.loadBoard(context.Background(), boardFixtureName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	// The scratch tier stays live (annotation writes are not the
	// kernel's): the wall is still the authoring surface for stickies.
	if proj.Mode != modeAuthoring {
		t.Fatalf("Mode = %q, want %q (the scratch tier stays live)", proj.Mode, modeAuthoring)
	}
	// The domain surface is refused, and the refusal names the exact
	// branch the kernel requires.
	if proj.DomainRefusal == "" {
		t.Fatal("DomainRefusal is empty on a wrong-branch draft: the wall would offer spec edits the kernel refuses (state-forbidden)")
	}
	if !strings.Contains(proj.DomainRefusal, `design/`+boardFixtureName) {
		t.Errorf("DomainRefusal does not name the required branch design/%s: %q", boardFixtureName, proj.DomainRefusal)
	}

	// The rendered wall: explanation visible, no typed-forms panel, no
	// spec-edge removal affordances — asserted through the real page
	// handler (the same render the browser receives).
	h := newBoardTestHandler(root)
	rec := getBoard(t, h, boardFixtureName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET wrong-branch board = %d\n%s", rec.Code, rec.Body.String())
	}
	html := rec.Body.String()
	if !strings.Contains(html, `data-testid="asd-domain-refusal"`) {
		t.Error("the wrong-branch wall renders no domain-refusal explanation")
	}
	if strings.Contains(html, `id="asd-forms"`) {
		t.Error("the wrong-branch wall still renders the typed-operation forms panel")
	}
	if strings.Contains(html, `data-retype`) || strings.Contains(html, `data-delete="edge"`) {
		t.Error("the wrong-branch wall still renders spec-edge removal/retype affordances")
	}

	// The namesake-branch fixture keeps the full surface (control).
	root2 := newBoardFixture(t)
	s2 := &boardSpecServer{root: root2, design: testDesignBridge{}}
	proj2, _, _, _, err := s2.loadBoard(context.Background(), boardFixtureName)
	if err != nil {
		t.Fatalf("loadBoard (namesake branch): %v", err)
	}
	if proj2.Mode != modeAuthoring || proj2.DomainRefusal != "" {
		t.Fatalf("namesake-branch draft: Mode=%q DomainRefusal=%q, want authoring with no refusal", proj2.Mode, proj2.DomainRefusal)
	}
}

func TestMutateDraft_WrongBranch_RefusedBeforeKernel(t *testing.T) {
	root := newWrongBranchFixture(t)
	h := newBoardTestHandler(root)
	rec, _ := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "x", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("mutate_draft on a wrong-branch draft = %d, want 403\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "design/"+boardFixtureName) {
		t.Errorf("the refusal does not name the required branch design/%s:\n%s", boardFixtureName, rec.Body.String())
	}
}

// --- I-3(a): un-rendered link types guard --------------------------------

// inboundLinkFixtureSpec: dc-1 depends-on oq-1. The artifact validator
// (internal/artifact/common.go: fragment targets are closed to the five
// yarn types) means every VALID inbound fragment link is chip-rendered —
// but the mutate_draft surface takes batches, not gestures: a batch that
// removes the fragment without covering the stored link (a direct API
// caller, or any client drift from the rendered chips) would land a
// dangling ref (VL-003). The guard enumerates the decoded frontmatter
// itself — the legacy object-trash enumeration — so honesty never
// depends on what happened to be rendered.
const inboundLinkFixtureSpec = `---
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
open_questions:
  - { id: oq-1, text: "which reasons can be shown verbatim?", anchor: "#oq-1" }
decisions:
  - { id: dc-1, text: "depend on the open legal question", anchor: "#dc-1",
      links: [ { type: depends-on, ref: "spec/refi-test#oq-1", note: "legal context" } ] }
---
# Refi test flow

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## oq-1

Prose.

## dc-1

Prose.
`

func newInboundLinkFixture(t *testing.T) string {
	t.Helper()
	return buildAuthoringFixture(t, "design/"+boardFixtureName,
		map[string]string{".verdi/.gitignore": "data/\n"},
		map[string]string{
			".verdi/specs/active/" + boardFixtureName + "/spec.md": inboundLinkFixtureSpec,
		})
}

func TestMutateDraft_RemoveObject_RefusesUncoveredInboundLink(t *testing.T) {
	root := newInboundLinkFixture(t)
	h := newBoardTestHandler(root)

	// A batch that removes the fragment WITHOUT covering the stored
	// inbound link must be refused with ZERO mutation, naming the
	// uncovered link (the legacy enumeration, server-side).
	rec, _ := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "remove-question", "id": "oq-1"},
	}, nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("remove-question with an uncovered depends-on link = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"dc-1", "depends-on", "spec/refi-test#oq-1"} {
		if !strings.Contains(body, want) {
			t.Errorf("refusal does not disclose %q:\n%s", want, body)
		}
	}
	// Zero mutation: the link and the question both survive.
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "depends-on") || !strings.Contains(string(data), "id: oq-1") {
		t.Fatalf("a refused removal mutated the spec:\n%s", data)
	}

	// The covering batch lands: remove the exact stored tuple in the same
	// transaction and the removal is clean, leaving no dangling ref.
	rec2, out := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "remove-link", "source": "dc-1", "type": "depends-on", "ref": "spec/refi-test#oq-1", "note": "legal context"},
		{"op": "remove-question", "id": "oq-1"},
	}, nil, nil)
	if rec2.Code != http.StatusOK || out.Result == nil {
		t.Fatalf("covered removal batch = %d, want a clean result\n%s", rec2.Code, rec2.Body.String())
	}
	after, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "depends-on") || strings.Contains(string(after), "id: oq-1") {
		t.Fatalf("the covered batch left link or object behind:\n%s", after)
	}
}

// docLinkFixtureSpec: the document-level links: block names ac-1. The
// board can never edit that block, so removing ac-1 must be refused (the
// legacy object-trash refusal, preserved through the typed path).
const docLinkFixtureSpec = `---
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
links:
  - { type: depends-on, ref: "spec/refi-test#ac-1" }
---
# Refi test flow

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.
`

func TestMutateDraft_RemoveObject_RefusesDocumentHeldFragment(t *testing.T) {
	root := buildAuthoringFixture(t, "design/"+boardFixtureName,
		map[string]string{".verdi/.gitignore": "data/\n"},
		map[string]string{
			".verdi/specs/active/" + boardFixtureName + "/spec.md": docLinkFixtureSpec,
		})
	h := newBoardTestHandler(root)
	rec, _ := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "remove-ac", "id": "ac-1"},
	}, nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("remove-ac held by the document links: block = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "links:") {
		t.Errorf("refusal does not name the links: block:\n%s", rec.Body.String())
	}
}

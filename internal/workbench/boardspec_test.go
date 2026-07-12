package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
	"github.com/OWNER/verdi/internal/fixturegit"
	"github.com/OWNER/verdi/internal/gitx"
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
frozen: { at: 2026-03-01, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
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
frozen: { at: 2026-03-02, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# Retry budget for downstream calls

## Decision

Every downstream call spends from a shared retry budget.
`

// newBoardFixture builds a fixture repo with the draft spec, checked out
// on a design branch (authoring mode's branch state).
func newBoardFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + boardFixtureName + "/spec.md":     boardFixtureSpec,
			".verdi/specs/active/" + boardFixtureName + "/layout.json": boardFixtureLayout,
			".verdi/adr/0001-outbox-events.md":                         boardFixtureADR,
			".verdi/adr/0007-retry-budget.md":                          boardFixtureADR2,
			".verdi/.gitignore":                                        "data/\n",
		},
		Message: "seed board fixture",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/"+boardFixtureName); err != nil {
		t.Fatalf("checkout design branch: %v", err)
	}
	return repo.Dir
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
	if !strings.Contains(body, `data-testid="card-ac-1" data-id="ac-1" data-object-kind="acceptance-criterion" style="left:40px;top:60px"`) {
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
	if !strings.Contains(body, `data-testid="card-ac-1" data-id="ac-1" data-object-kind="acceptance-criterion" style="left:40px;top:20px"`) {
		t.Error("ac-1 (first claimant) not rendered at its stored position")
	}
	// The collider does NOT render at its stored, overlapping position.
	if strings.Contains(body, `data-id="ac-2" data-object-kind="acceptance-criterion" style="left:220px;top:20px"`) {
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

func TestBoardSpec_EditText(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, boardFixtureName, "edit-text", `{"id":"ac-1","text":"a declined applicant sees the current reason [edited]"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit-text = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if !resp.Dirty {
		t.Error("edit-text response dirty = false, want true (the spec working tree changed)")
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
	h := NewHandler(root)
	tests := []struct {
		name, body string
		wantCode   int
	}{
		{"unknown object", `{"id":"ac-99","text":"x"}`, http.StatusBadRequest},
		{"unknown field fails closed", `{"id":"ac-1","text":"x","bogus":1}`, http.StatusBadRequest},
		{"empty text", `{"id":"ac-1","text":""}`, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := postBoardAPI(t, h, boardFixtureName, "edit-text", tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("= %d, want %d\n%s", rec.Code, tc.wantCode, rec.Body.String())
			}
		})
	}
}

func TestBoardSpec_WritesRefusedOutsideAuthoring(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	// On the default branch the draft is not authorable (05 §Workbench:
	// modes keyed by branch state).
	if err := gitx.Checkout(context.Background(), root, "main"); err != nil {
		t.Fatal(err)
	}
	rec := postBoardAPI(t, h, boardFixtureName, "edit-text", `{"id":"ac-1","text":"x"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("edit-text off design branch = %d, want 403", rec.Code)
	}
	if !strings.Contains(getBoard(t, h, boardFixtureName).Body.String(), `data-board-mode="readonly"`) {
		t.Error("board off design branch is not read-only")
	}
}

func TestBoardSpec_Edge(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	// Illegal pair: an AC source offers no typed edge.
	rec := postBoardAPI(t, h, boardFixtureName, "edge", `{"from":"ac-1","to":"ac-2","type":"implements"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("illegal edge = %d, want 400", rec.Code)
	}
	// Unknown type fails closed even on a legal pair.
	rec = postBoardAPI(t, h, boardFixtureName, "edge", `{"from":"dc-2","to":"adr/0001-outbox-events","type":"blesses"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown edge type = %d, want 400", rec.Code)
	}

	// Legal: decision → ADR supersedes (the first yarn on dc-2 — the S7
	// absent-links splice case, exercised through the full HTTP path).
	rec = postBoardAPI(t, h, boardFixtureName, "edge", `{"from":"dc-2","to":"adr/0001-outbox-events","type":"supersedes","note":"drawn on the board"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("legal edge = %d\n%s", rec.Code, rec.Body.String())
	}
	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-edge-type="supersedes" data-from="dc-2" data-to="adr/0001-outbox-events" data-layer="spec"`) {
		t.Error("new edge does not project as spec-layer yarn")
	}
}

func TestBoardSpec_StickyLifecycle(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

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

	// Graduation: the sticky becomes a declared open-question object via
	// an ordinary edit; the record flips to graduated.
	rec = postBoardAPI(t, h, boardFixtureName, "sticky-graduate", `{"id":"`+stickyID+`","kind":"open-question"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sticky-graduate = %d\n%s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Dirty {
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
			"adr/0001-outbox-events@c5e360a9ee5e9eb6089e54b772fa16959ada4662",
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

	// Negative: an id this board does not present is refused.
	rec = postBoardAPI(t, h, boardFixtureName, "annotation-delete", `{"id":"a-01J8Z0K9ZZZZZZZZZZZZZZZZZZ"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("deleting a foreign annotation = %d, want 400", rec.Code)
	}
}

// Owner UAT round 6, item 3(c): removing a spec-layer typed edge is the
// exact inverse of drawing it — an ordinary spec edit through the
// splice write path.
func TestBoardSpec_EdgeDelete(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	// The fixture's dc-1 carries the declared exempts edge.
	rec := postBoardAPI(t, h, boardFixtureName, "edge-delete", `{"from":"dc-1","to":"adr/0001-outbox-events","type":"exempts"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("edge-delete = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Dirty {
		t.Error("removing a declared edge did not dirty the working tree (it IS a spec edit)")
	}
	specData, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if strings.Contains(string(specData), "exempts") {
		t.Errorf("spec still carries the removed edge:\n%s", specData)
	}
	if strings.Contains(getBoard(t, h, boardFixtureName).Body.String(), `data-edge-type="exempts" data-from="dc-1"`) {
		t.Error("removed edge still projects as yarn")
	}

	// Negatives: unknown edge, undeclared source, document-level source.
	for name, body := range map[string]string{
		"already removed":       `{"from":"dc-1","to":"adr/0001-outbox-events","type":"exempts"}`,
		"undeclared source":     `{"from":"zz-9","to":"adr/0001-outbox-events","type":"exempts"}`,
		"document-level source": `{"from":"spec","to":"adr/0001-outbox-events","type":"implements"}`,
	} {
		rec := postBoardAPI(t, h, boardFixtureName, "edge-delete", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400\n%s", name, rec.Code, rec.Body.String())
		}
	}
}

// Owner directive (round 6 UAT follow-up): the relationship's type is
// updatable in place — one atomic splice transaction, ref and note
// surviving verbatim.
func TestBoardSpec_EdgeRetype(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, boardFixtureName, "edge-retype", `{"from":"dc-1","to":"adr/0001-outbox-events","type":"exempts","newType":"supersedes"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("edge-retype = %d\n%s", rec.Code, rec.Body.String())
	}
	specData, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if !strings.Contains(string(specData), `links: [ { type: supersedes, ref: adr/0001-outbox-events, note: "async by design" } ]`) {
		t.Errorf("spec does not carry the retyped edge with ref and note verbatim:\n%s", specData)
	}
	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-edge-type="supersedes" data-from="dc-1"`) {
		t.Error("retyped edge does not project")
	}
	if strings.Contains(body, `data-edge-type="exempts" data-from="dc-1"`) {
		t.Error("old edge type still projects")
	}

	// Negatives: an illegal new type, and a type the edge does not carry.
	for name, req := range map[string]string{
		"illegal new type":   `{"from":"dc-1","to":"adr/0001-outbox-events","type":"supersedes","newType":"implements"}`,
		"wrong current type": `{"from":"dc-1","to":"adr/0001-outbox-events","type":"exempts","newType":"supersedes"}`,
	} {
		rec := postBoardAPI(t, h, boardFixtureName, "edge-retype", req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400\n%s", name, rec.Code, rec.Body.String())
		}
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
		frozen := renderBoardRegion(proj, &boardGitState{})
		for _, banned := range []string{`class="delete-btn"`, `data-retype`, `class="graduate-btn"`} {
			if strings.Contains(frozen, banned) {
				t.Errorf("%s board renders %s", mode, banned)
			}
		}
	}
}

func TestBoardSpec_RelatesLifecycle(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

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

	// Graduation to a typed edge via the picker's pair.
	rec = postBoardAPI(t, h, boardFixtureName, "relates-graduate", `{"id":"`+threadID+`","type":"exempts","note":"confirmed on the board"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("relates-graduate = %d\n%s", rec.Code, rec.Body.String())
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
	rec = postBoardAPI(t, h, boardFixtureName, "relates-graduate", `{"id":"`+acThread+`","type":"implements"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("illegal relates-graduate = %d, want 400", rec.Code)
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
	h := NewHandler(root)
	ctx := context.Background()

	// Dirty the tree through the board's own write path.
	postBoardAPI(t, h, boardFixtureName, "edit-text", `{"id":"co-1","text":"notices never name internal scores [amended]"}`)
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
	h := NewHandlerWith(root, Deps{CommentFeed: feed})

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
	// And no write goes through.
	recW := postBoardAPI(t, h, boardFixtureName, "edit-text", `{"id":"ac-1","text":"x"}`)
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

// erroringFeed is a CommentFeed whose call always fails — the
// configured-but-unreachable forge from the failure side (I-2).
type erroringFeed struct{}

func (erroringFeed) ListMRComments(context.Context, string) ([]MRComment, bool, error) {
	return nil, false, errors.New("forge unreachable: dial tcp 10.0.0.1:443: connect: connection refused")
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
	// The failure is disclosed, never silent.
	if !strings.Contains(body, `data-testid="board-notice"`) || !strings.Contains(body, "review feed unavailable") {
		t.Errorf("feed error not disclosed as a board notice:\n%s", body)
	}
	// The fragment surface degrades identically (post-mutation re-render).
	frag := httptest.NewRecorder()
	h.ServeHTTP(frag, httptest.NewRequest(http.MethodGet, "/board/spec/"+boardFixtureName+"/fragment", nil))
	if frag.Code != http.StatusOK || !strings.Contains(frag.Body.String(), "review feed unavailable") {
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
	for _, absent := range []string{"review feed unavailable", "review state cannot be shown"} {
		if strings.Contains(body, absent) {
			t.Errorf("unconfigured forge should be silent, but board contains %q", absent)
		}
	}
}

// TestBoard_DefaultBranchAssumed_Disclosed proves M-4: a repo with no
// origin/HEAD configured (the fixture's state) discloses the assumed "main"
// default rather than keying authoring mode off it silently.
func TestBoard_DefaultBranchAssumed_Disclosed(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-testid="board-notice"`) || !strings.Contains(body, "default branch could not be resolved") {
		t.Errorf("assumed default branch not disclosed on the board:\n%s", body)
	}
}

// TestBoard_ConcurrentMutations_BothLand proves M-2: two racing board
// mutations against the same spec both land (no lost update). Without the
// per-server writeMu the second read-modify-write of spec.md could
// clobber the first (last writer wins).
func TestBoard_ConcurrentMutations_BothLand(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	var wg sync.WaitGroup
	edits := []struct{ id, text string }{
		{"ac-1", "a declined applicant sees the current reason [edit-A]"},
		{"ac-2", "a decline reverses within one day [edit-B]"},
	}
	for _, e := range edits {
		wg.Add(1)
		go func(id, text string) {
			defer wg.Done()
			body := `{"id":"` + id + `","text":"` + text + `"}`
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/board/spec/"+boardFixtureName+"/api/edit-text", strings.NewReader(body))
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("edit-text %s = %d, want 200: %s", id, rec.Code, rec.Body.String())
			}
		}(e.id, e.text)
	}
	wg.Wait()

	specData, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	spec := string(specData)
	for _, e := range edits {
		if !strings.Contains(spec, e.text) {
			t.Errorf("edit %q was lost (last-writer-wins) — both concurrent mutations must land:\n%s", e.text, spec)
		}
	}
}

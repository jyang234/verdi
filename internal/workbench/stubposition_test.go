package workbench

// Round 5.5's dc-6 amendment: stub cards become draggable, storing their
// position under a "stub:<slug>" layout.json key exactly like an object
// card's stored position (02 §Record schemas, VL-018). These tests cover
// the "position" action's stub-aware write path: accepting a stub id,
// treating other stub cards as drop obstacles, refusing an unknown stub
// in plain language, and reload-determinism (a fresh projection reproduces
// the stored spot).

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// stubDragSpec is a draft feature-class wall (authoring mode) already
// carrying two declared stubs — legal on a draft spec (Stub.Validate
// requires no particular status; only accepted/closed/superseded statuses
// require `frozen`), and the exact shape the board renders as scoping
// cards regardless of the wall's own lifecycle stage.
const stubDragSpec = `---
id: spec/stub-drag
kind: spec
class: feature
title: "Stub drag wall"
status: draft
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "ac one", evidence: [attestation], anchor: "#ac-1" }
open_questions:
  - { id: oq-1, text: "oq one", anchor: "#oq-1" }
stubs:
  - { slug: borrower-update-api, acceptance_criteria: [ac-1] }
  - { slug: retry-strategy-spike, spike: true, resolves: [oq-1] }
---
# Stub drag wall

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## oq-1

Prose.
`

const stubDragName = "stub-drag"

func newStubDragFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + stubDragName + "/spec.md": stubDragSpec,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed stub drag fixture",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/"+stubDragName); err != nil {
		t.Fatalf("checkout design branch: %v", err)
	}
	return repo.Dir
}

func stubDragLayoutPath(root string) string {
	return filepath.Join(root, ".verdi", "specs", "active", stubDragName, "layout.json")
}

// TestBoardSpec_Position_Stub_Happy proves the position action accepts a
// "stub:<slug>" id naming a declared stub, writes ONLY that key, and the
// stored position renders verbatim and survives a fresh reload (the
// determinism a real drag depends on).
func TestBoardSpec_Position_Stub_Happy(t *testing.T) {
	root := newStubDragFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, stubDragName, "position", `{"id":"stub:borrower-update-api","x":700,"y":300}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("position(stub) = %d\n%s", rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(stubDragLayoutPath(root))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, `"stub:borrower-update-api":{"x":700,"y":300}`) {
		t.Errorf("layout.json missing the stored stub position: %s", got)
	}

	if !strings.Contains(getBoard(t, h, stubDragName).Body.String(), `data-testid="stub-card-borrower-update-api" data-stub="borrower-update-api" style="left:700px;top:300px"`) {
		body := getBoard(t, h, stubDragName).Body.String()
		t.Errorf("stored stub position does not render verbatim:\n%s", body)
	}

	// Reload-determinism: a fresh projection reproduces the exact stored
	// spot (the same property already proven for object cards).
	proj, _, _, err := (&boardSpecServer{root: root}).loadBoard(context.Background(), stubDragName)
	if err != nil {
		t.Fatal(err)
	}
	var foundX, foundY float64
	var found bool
	for _, sv := range proj.StubViews {
		if sv.Slug == "borrower-update-api" {
			foundX, foundY, found = sv.X, sv.Y, true
		}
	}
	if !found || foundX != 700 || foundY != 300 {
		t.Fatalf("reloaded projection stub position = (%v,%v, found=%v), want (700,300, true)", foundX, foundY, found)
	}
}

// TestBoardSpec_Position_Stub_CollidesWithOtherStub proves a stub drop
// overlapping ANOTHER stub's stored footprint resolves to the nearest
// free position — the same collision-free-by-construction, only-the-
// dragged-card guarantee object drags already have, extended to stub
// obstacles.
func TestBoardSpec_Position_Stub_CollidesWithOtherStub(t *testing.T) {
	root := newStubDragFixture(t)
	h := NewHandler(root)

	// Park the spike stub at a known spot first.
	rec := postBoardAPI(t, h, stubDragName, "position", `{"id":"stub:retry-strategy-spike","x":400,"y":400}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("position(spike stub) = %d\n%s", rec.Code, rec.Body.String())
	}

	// Now drop the plain stub squarely on top of it.
	rec = postBoardAPI(t, h, stubDragName, "position", `{"id":"stub:borrower-update-api","x":400,"y":400}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("position(colliding stub) = %d\n%s", rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(stubDragLayoutPath(root))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, `"stub:borrower-update-api":{"x":400,"y":400}`) {
		t.Errorf("colliding stub drop was not resolved off the other stub's footprint: %s", got)
	}
	// Only the dragged card's stored position changed — the obstacle's
	// own stored position is untouched.
	if !strings.Contains(got, `"stub:retry-strategy-spike":{"x":400,"y":400}`) {
		t.Errorf("drop resolution touched the other stub's stored position: %s", got)
	}
}

// TestBoardSpec_Position_Stub_Unknown proves an unknown stub slug is
// refused in plain language (VL-018's own resolution requirement, echoed
// server-side rather than merely relying on the lint gate).
func TestBoardSpec_Position_Stub_Unknown(t *testing.T) {
	root := newStubDragFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, stubDragName, "position", `{"id":"stub:no-such-stub","x":1,"y":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("position(unknown stub) = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "stub:no-such-stub") {
		t.Errorf("refusal does not name the unknown stub: %s", rec.Body.String())
	}
	data, err := os.ReadFile(stubDragLayoutPath(root))
	if err == nil && strings.Contains(string(data), "no-such-stub") {
		t.Errorf("the refused write still touched layout.json: %s", data)
	}
}

// TestBoardSpec_Position_ObjectAvoidsStubObstacle proves the obstacle set
// the position action builds for an OBJECT drag now includes stub
// footprints too — dragging ac-1 onto a stub's stored spot resolves away
// from it, not through it.
func TestBoardSpec_Position_ObjectAvoidsStubObstacle(t *testing.T) {
	root := newStubDragFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, stubDragName, "position", `{"id":"stub:borrower-update-api","x":500,"y":500}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("position(stub) = %d\n%s", rec.Code, rec.Body.String())
	}
	rec = postBoardAPI(t, h, stubDragName, "position", `{"id":"ac-1","x":500,"y":500}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("position(ac-1 onto stub) = %d\n%s", rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(stubDragLayoutPath(root))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, `"ac-1":{"x":500,"y":500}`) {
		t.Errorf("ac-1 was allowed to land on the stub's stored footprint: %s", got)
	}
	if !strings.Contains(got, `"stub:borrower-update-api":{"x":500,"y":500}`) {
		t.Errorf("the stub's own stored position was disturbed by ac-1's drop resolution: %s", got)
	}
}

package workbench

// The import/pin surface (02 §Record schemas round-5.2: type pin;
// 05 §The scratch tier: pinned references) and the trash gesture's
// server half (owner directive round 7: dragging a wall element to the
// trash removes it and disconnects its yarn). Everything here is
// authoring-mode-only, behind the same 403 gate every board write
// already sits behind.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
	"github.com/OWNER/verdi/internal/fixturegit"
	"github.com/OWNER/verdi/internal/gitx"
)

func getPinSearch(t *testing.T, h http.Handler, name, query string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/"+name+"/pinsearch"+query, nil))
	return rec
}

func readAnnotations(t *testing.T, root string) []*artifact.Annotation {
	t.Helper()
	as, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBoardSpec_PinLifecycle(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	// Import: pin an artifact nothing on the wall names yet.
	rec := postBoardAPI(t, h, boardFixtureName, "pin", `{"ref":"adr/0007-retry-budget"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("pin = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Dirty {
		t.Error("a pin dirtied the spec working tree (pins are mutable-zone records)")
	}

	as := readAnnotations(t, root)
	if len(as) != 1 || as[0].Type != artifact.AnnotationPin {
		t.Fatalf("annotations = %+v, want one pin", as)
	}
	pin := as[0]
	if pin.Target == nil || !strings.HasPrefix(pin.Target.Ref, "adr/0007-retry-budget@") {
		t.Fatalf("pin target = %+v, want adr/0007-retry-budget pinned at HEAD", pin.Target)
	}
	if pin.Board == nil || pin.Board.Story != boardFixtureName {
		t.Fatalf("pin board = %+v, want this board", pin.Board)
	}
	// Deterministic landing: the bottom of the references lane — below
	// the edge-derived adr/0001 card at the lane's first slot (y 40,
	// RefCardHeight 72, gap 24).
	if pin.Board.X != 952 || pin.Board.Y != 136 {
		t.Errorf("pin landed at (%v, %v), want the references lane bottom (952, 136)", pin.Board.X, pin.Board.Y)
	}

	// The pinned ref wears the same reference-card paper, deduped: one
	// card, carrying the pin marking and its stored position.
	body := getBoard(t, h, boardFixtureName).Body.String()
	if strings.Count(body, `data-ref="adr/0007-retry-budget"`) != 1 {
		t.Fatalf("want exactly one card for the pinned ref:\n%s", body)
	}
	if !strings.Contains(body, `data-pin-id="`+pin.ID+`"`) {
		t.Error("pinned card does not carry its pin id")
	}
	if !strings.Contains(body, `style="left:952px;top:136px"`) {
		t.Error("pinned card does not render at the pin record's stored position")
	}

	// Pins drag like stickies: the position write rewrites the record.
	rec = postBoardAPI(t, h, boardFixtureName, "sticky-position", `{"id":"`+pin.ID+`","x":300,"y":500}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sticky-position on a pin = %d\n%s", rec.Code, rec.Body.String())
	}
	as = readAnnotations(t, root)
	if as[0].Board.X != 300 || as[0].Board.Y != 500 {
		t.Errorf("pin position after drag = (%v, %v), want (300, 500)", as[0].Board.X, as[0].Board.Y)
	}

	// Graduation IS drawing a typed edge to the pinned target (02): the
	// record flips to graduated, the card stays (the edge projects it now)
	// and files into the references lane — the pin no longer holds it.
	rec = postBoardAPI(t, h, boardFixtureName, "edge", `{"from":"dc-2","to":"adr/0007-retry-budget","type":"exempts"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("edge = %d\n%s", rec.Code, rec.Body.String())
	}
	as = readAnnotations(t, root)
	if len(as) != 1 || as[0].Status != artifact.AnnotationGraduated {
		t.Fatalf("after graduation annotations = %+v, want the one pin graduated", as)
	}
	body = getBoard(t, h, boardFixtureName).Body.String()
	if strings.Count(body, `data-ref="adr/0007-retry-budget"`) != 1 {
		t.Error("the card did not survive graduation (the edge projects it)")
	}
	if strings.Contains(body, `data-pin-id=`) {
		t.Error("a graduated pin still marks its card as pinned")
	}
}

func TestBoardSpec_Pin_Negative(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	for name, body := range map[string]string{
		"missing ref":          `{}`,
		"unresolvable ref":     `{"ref":"adr/no-such-adr"}`,
		"malformed ref":        `{"ref":"not a ref"}`,
		"the board's own spec": `{"ref":"spec/refi-test"}`,
		"a fragment":           `{"ref":"spec/refi-test#ac-1"}`,
		"already on the wall":  `{"ref":"adr/0001-outbox-events"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := postBoardAPI(t, h, boardFixtureName, "pin", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("pin(%s) = %d, want 400\n%s", name, rec.Code, rec.Body.String())
			}
		})
	}
	if as := readAnnotations(t, root); len(as) != 0 {
		t.Errorf("a refused pin still wrote a record: %+v", as)
	}

	// Double pin: the first lands, the second fails closed (one card per
	// ref, ever).
	if rec := postBoardAPI(t, h, boardFixtureName, "pin", `{"ref":"adr/0007-retry-budget"}`); rec.Code != http.StatusOK {
		t.Fatalf("first pin = %d", rec.Code)
	}
	if rec := postBoardAPI(t, h, boardFixtureName, "pin", `{"ref":"adr/0007-retry-budget"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("second pin of the same ref = %d, want 400", rec.Code)
	}
}

func TestBoardSpec_PinSearch(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	rec := getPinSearch(t, h, boardFixtureName, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("pinsearch = %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-ref="adr/0007-retry-budget"`) {
		t.Error("pinsearch omits an eligible corpus artifact")
	}
	if !strings.Contains(body, "Retry budget for downstream calls") || !strings.Contains(body, ">adr<") {
		t.Error("pinsearch results do not show kind and title")
	}
	// The board's own spec and refs already on the wall are excluded.
	if strings.Contains(body, `data-ref="spec/refi-test"`) {
		t.Error("pinsearch offers the board's own spec")
	}
	if strings.Contains(body, `data-ref="adr/0001-outbox-events"`) {
		t.Error("pinsearch offers a ref already on the wall")
	}

	// Deterministic: same query, same fragment.
	if again := getPinSearch(t, h, boardFixtureName, "").Body.String(); again != body {
		t.Error("two identical pinsearch renders differ")
	}

	// A query narrows; a no-hit query discloses instead of voiding.
	rec = getPinSearch(t, h, boardFixtureName, "?q=retry+budget")
	if !strings.Contains(rec.Body.String(), "adr/0007-retry-budget") {
		t.Error("query does not find the retry ADR")
	}
	rec = getPinSearch(t, h, boardFixtureName, "?q=zzzznothing")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "matches") {
		t.Errorf("no-hit query should disclose emptiness, got %d: %s", rec.Code, rec.Body.String())
	}

	// Authoring-only, like every board write surface.
	if err := gitx.Checkout(context.Background(), root, "main"); err != nil {
		t.Fatal(err)
	}
	// The spec is not on main; use the readonly path: a spec present on
	// main renders read-only. Reuse the design spec by checking the 403
	// gate through the API instead: pinsearch on the branchless checkout.
	rec = getPinSearch(t, h, boardFixtureName, "")
	if rec.Code == http.StatusOK {
		t.Errorf("pinsearch outside authoring = %d, want a refusal", rec.Code)
	}
}

// trashFixtureSpec wires the object-trash case: dc-2 supersedes dc-1's
// fragment, so trashing dc-1 must also splice dc-2's link out (VL-003:
// no dangling fragment refs).
const trashFixtureSpec = `---
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
decisions:
  - { id: dc-1, text: "excuse this flow from the outbox rule", anchor: "#dc-1",
      links: [ { type: exempts, ref: adr/0001-outbox-events, note: "async by design" } ] }
  - { id: dc-2, text: "reuse the notification channel", anchor: "#dc-2",
      links: [ { type: supersedes, ref: "spec/refi-test#dc-1" } ] }
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

## dc-1

Prose.

## dc-2

Prose.
`

const trashFixtureLayout = `{
  "schema": "verdi.boardlayout/v1",
  "positions": { "ac-1": { "x": 40, "y": 60 } }
}
`

func newTrashFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + boardFixtureName + "/spec.md":     trashFixtureSpec,
			".verdi/specs/active/" + boardFixtureName + "/layout.json": trashFixtureLayout,
			".verdi/adr/0001-outbox-events.md":                         boardFixtureADR,
			".verdi/.gitignore":                                        "data/\n",
		},
		Message: "seed trash fixture",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/"+boardFixtureName); err != nil {
		t.Fatalf("checkout design branch: %v", err)
	}
	return repo.Dir
}

func TestBoardSpec_RefTrash(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	// Give the adr/0001 card a scratch thread too: trash must take the
	// typed edge AND the thread in one act.
	rec := postBoardAPI(t, h, boardFixtureName, "relates", `{"from":"ac-1","to":"adr/0001-outbox-events"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("relates = %d\n%s", rec.Code, rec.Body.String())
	}
	if as := readAnnotations(t, root); len(as) != 1 {
		t.Fatalf("annotations = %+v, want the one thread", as)
	}

	rec = postBoardAPI(t, h, boardFixtureName, "ref-trash", `{"ref":"adr/0001-outbox-events"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("ref-trash = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Dirty {
		t.Error("removing a declared edge did not dirty the spec tree")
	}

	specData, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if strings.Contains(string(specData), "exempts") {
		t.Errorf("the exempts edge survived the trash:\n%s", specData)
	}
	if as := readAnnotations(t, root); len(as) != 0 {
		t.Errorf("the ref's relates thread survived the trash: %+v", as)
	}
	body := getBoard(t, h, boardFixtureName).Body.String()
	if strings.Contains(body, `data-ref="adr/0001-outbox-events"`) {
		t.Error("the trashed reference card still renders")
	}
}

func TestBoardSpec_RefTrash_PinWithThreads(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	if rec := postBoardAPI(t, h, boardFixtureName, "pin", `{"ref":"adr/0007-retry-budget"}`); rec.Code != http.StatusOK {
		t.Fatalf("pin = %d", rec.Code)
	}
	if rec := postBoardAPI(t, h, boardFixtureName, "relates", `{"from":"ac-1","to":"adr/0007-retry-budget"}`); rec.Code != http.StatusOK {
		t.Fatalf("relates = %d", rec.Code)
	}
	if as := readAnnotations(t, root); len(as) != 2 {
		t.Fatalf("annotations = %+v, want pin + thread", as)
	}

	// A pin with no typed edges dies without ceremony — taking its own
	// relates threads with it (02 §Record schemas), spec untouched.
	rec := postBoardAPI(t, h, boardFixtureName, "ref-trash", `{"ref":"adr/0007-retry-budget"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("ref-trash = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Dirty {
		t.Error("trashing a pure pin dirtied the spec tree")
	}
	if as := readAnnotations(t, root); len(as) != 0 {
		t.Errorf("the pin or its threads survived: %+v", as)
	}
}

func TestBoardSpec_RefTrash_Negative(t *testing.T) {
	// A document-level edge (frontmatter links:) is not board-editable:
	// trashing its reference card is refused, with the reason named.
	storySpec := `---
id: spec/refi-story
kind: spec
class: story
title: "Refi story"
status: draft
owners: [platform-team]
story: jira:LOAN-9
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/accepted-pending-build#ac-1" }
---
# Refi story

## Problem

Prose.

## Outcome

Prose.
`
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/refi-story/spec.md": storySpec,
			".verdi/.gitignore":                      "data/\n",
		},
		Message: "seed story fixture",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/refi-story"); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(repo.Dir)

	rec := postBoardAPI(t, h, "refi-story", "ref-trash", `{"ref":"spec/accepted-pending-build#ac-1"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ref-trash on a document-level edge = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "document") {
		t.Errorf("the refusal does not name the document-level edge: %s", rec.Body.String())
	}

	// Unknown refs fail closed.
	root := newBoardFixture(t)
	h2 := NewHandler(root)
	rec = postBoardAPI(t, h2, boardFixtureName, "ref-trash", `{"ref":"adr/not-on-this-wall"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ref-trash on an absent ref = %d, want 400", rec.Code)
	}
}

func TestBoardSpec_ObjectTrash(t *testing.T) {
	root := newTrashFixture(t)
	h := NewHandler(root)

	// A relates thread touching the object dies with it (the owner's
	// verbatim ask: removal disconnects any existing relationship yarn).
	if rec := postBoardAPI(t, h, boardFixtureName, "relates", `{"from":"ac-1","to":"dc-1"}`); rec.Code != http.StatusOK {
		t.Fatalf("relates = %d", rec.Code)
	}

	rec := postBoardAPI(t, h, boardFixtureName, "object-trash", `{"id":"dc-1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("object-trash = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Dirty {
		t.Error("object removal did not dirty the spec tree")
	}

	specData, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	spec := string(specData)
	if strings.Contains(spec, "{ id: dc-1,") {
		t.Errorf("dc-1's frontmatter entry survived:\n%s", spec)
	}
	if strings.Contains(spec, "spec/refi-test#dc-1") {
		t.Errorf("dc-2's link to the removed object survived (VL-003 would trip):\n%s", spec)
	}
	// Prose is never silently destroyed: the body section stays.
	if !strings.Contains(spec, "\n## dc-1\n") {
		t.Error("dc-1's body prose was deleted")
	}
	// The scratch thread died with the card.
	if as := readAnnotations(t, root); len(as) != 0 {
		t.Errorf("the object's relates thread survived: %+v", as)
	}
	body := getBoard(t, h, boardFixtureName).Body.String()
	if strings.Contains(body, `data-testid="card-dc-1"`) {
		t.Error("the trashed object card still renders")
	}
}

func TestBoardSpec_ObjectTrash_PrunesLayout(t *testing.T) {
	root := newTrashFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, boardFixtureName, "object-trash", `{"id":"ac-1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("object-trash = %d\n%s", rec.Code, rec.Body.String())
	}
	layout, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "layout.json"))
	if strings.Contains(string(layout), "ac-1") {
		t.Errorf("layout.json still holds the removed object's key (VL-018):\n%s", layout)
	}
}

func TestBoardSpec_ObjectTrash_Negative(t *testing.T) {
	root := newTrashFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, boardFixtureName, "object-trash", `{"id":"oq-9"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("object-trash on an undeclared id = %d, want 400", rec.Code)
	}

	// Removing the LAST acceptance criterion would leave an invalid
	// feature spec: validate-before-write refuses, and nothing is written.
	before, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if rec := postBoardAPI(t, h, boardFixtureName, "object-trash", `{"id":"ac-2"}`); rec.Code != http.StatusOK {
		t.Fatalf("object-trash ac-2 = %d", rec.Code)
	}
	rec = postBoardAPI(t, h, boardFixtureName, "object-trash", `{"id":"ac-1"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("removing the last AC = %d, want 400 (feature needs one)", rec.Code)
	}
	after, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if !strings.Contains(string(after), "{ id: ac-1,") {
		t.Error("the refused removal still changed the spec")
	}
	_ = before
}

func TestBoardSpec_PinActionsAreAuthoringOnly(t *testing.T) {
	root := newBoardFixture(t)
	// On the default branch the same draft renders read-only (05
	// §Workbench: authoring is keyed to a design branch) — the mode every
	// new write affordance must 403 in.
	if err := gitx.Checkout(context.Background(), root, "main"); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(root)
	for action, body := range map[string]string{
		"pin":          `{"ref":"adr/0007-retry-budget"}`,
		"ref-trash":    `{"ref":"adr/0001-outbox-events"}`,
		"object-trash": `{"id":"ac-1"}`,
	} {
		rec := postBoardAPI(t, h, boardFixtureName, action, body)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s outside authoring = %d, want 403", action, rec.Code)
		}
	}
	if rec := getPinSearch(t, h, boardFixtureName, ""); rec.Code != http.StatusForbidden {
		t.Errorf("pinsearch outside authoring = %d, want 403", rec.Code)
	}
}

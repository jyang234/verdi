package workbench

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/gitx/readcensus"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/wtmanager"
)

const marksWallName = "marks-wall"

// marksWallSpec is a draft feature whose readiness names a card of each
// kind the marks project (SI-350 (1)): ac-2 by success/coverage/ac-2 (no
// stub lists it), oq-1 by its unclaimed shape/question/oq-1, and the
// digit-led stub 2fa-x by review/blocker/stub-unreconciled/s-2fa-x, whose
// last segment journey rewrites (SI-360 (1)).
const marksWallSpec = `---
id: spec/marks-wall
kind: spec
class: feature
title: "Marks wall"
status: draft
owners: [platform-team]
problem: { text: "a stale notice stands", anchor: "#problem" }
outcome: { text: "every channel retracts it", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a stale notice is retracted", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a retraction is audited", evidence: [attestation], anchor: "#ac-2" }
open_questions:
  - { id: oq-1, text: "which channel confirms first?", anchor: "#oq-1" }
stubs:
  - { slug: 2fa-x, acceptance_criteria: [ac-1] }
---
# Marks wall

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## ac-2

Prose.

## oq-1

Prose.
`

// newMarksWallFixture serves the marks wall from its design branch, with
// origin/main (discovered through origin/HEAD) as the default branch, a
// store manifest the readiness loader opens, and a second local branch,
// design/marks-other, carrying the same wall for a /b/ mount.
func newMarksWallFixture(t *testing.T) string {
	t.Helper()
	root := buildAuthoringFixture(t, "design/"+marksWallName,
		map[string]string{".verdi/.gitignore": "data/\n", ".verdi/verdi.yaml": "schema: verdi.layout/v1\n"},
		map[string]string{".verdi/specs/active/" + marksWallName + "/spec.md": marksWallSpec})
	gitOut(t, root, "update-ref", "refs/remotes/origin/main", "refs/heads/main")
	gitOut(t, root, "branch", "design/marks-other", "HEAD")
	return root
}

// marksWire is the part of the wall's snapshot the marks ride on.
type marksWire struct {
	Revision string     `json:"revision"`
	HTML     string     `json:"html"`
	Marks    *wallMarks `json:"marks"`
}

// pollWall GETs one wall snapshot under ctx, sending etag as If-None-Match
// when set, and returns the recorder and the decoded body (zero on 304).
func pollWall(t *testing.T, ctx context.Context, h http.Handler, path, etag string) (*httptest.ResponseRecorder, marksWire) {
	t.Helper()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, path+"/snapshot", nil)
	if etag != "" {
		req.Header.Set("If-None-Match", `"`+etag+`"`)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var body marksWire
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decoding %s snapshot: %v\n%s", path, err, rec.Body.String())
		}
		if rec.Header().Get("ETag") != `"`+body.Revision+`"` {
			t.Fatalf("%s: the ETag %s does not carry the body's revision %s", path, rec.Header().Get("ETag"), body.Revision)
		}
	}
	return rec, body
}

// pageRevision is the revision a wall page embeds for its client's first
// conditional refresh.
func pageRevision(t *testing.T, h http.Handler, path string) (string, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d\n%s", path, rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	const marker = "window.__BOARDV2__ = "
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("%s embeds no page state", path)
	}
	end := strings.Index(body[start:], ";\n</script>")
	var state struct {
		Asd struct {
			Revision string `json:"revision"`
		} `json:"asd"`
	}
	if end < 0 || json.Unmarshal([]byte(body[start+len(marker):start+end]), &state) != nil || state.Asd.Revision == "" {
		t.Fatalf("%s embeds no revision", path)
	}
	return state.Asd.Revision, body
}

// TestWallMarks_ServedPoll (SI-360 (2), (4); BL-165 (4)): on a wall served
// from the serving root, the 2 s poll serves the composed refresh. Its
// body carries the marks the readiness derivation names — an object card
// by success/coverage ("no stub") and by shape/question ("unresolved"),
// and a digit-led stub by journey's slug rule — exactly the marks a
// derivation over a load alone gives; its revision is the ETag and the
// page's embedded revision; the client's next poll with it is a 304; and
// each poll is one application projection (Wave 6 §5.3).
func TestWallMarks_ServedPoll(t *testing.T) {
	root := newMarksWallFixture(t)
	s := refreshServer(root)
	h := NewHandlerWith(root, Deps{Design: s.design, ReadinessLoader: s.readinessLoader})
	path := "/board/spec/" + marksWallName
	acc := originAccepted(t, root)

	census := &readcensus.Census{}
	rec, snap := pollWall(t, gitx.WithObserver(context.Background(), census), h, path, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
	}
	checkRefreshBudget(t, "the served composed poll, cold", census.Budget(acc))
	if snap.Marks == nil || snap.Marks.Unavailable != "" {
		t.Fatalf("the served poll's marks = %+v, want readable marks", snap.Marks)
	}
	for card, want := range map[string]wallMark{
		"ac-2": {Concern: "success/coverage/ac-2", Chip: markChipNoStub},
		"oq-1": {Concern: "shape/question/oq-1", Chip: markChipUnresolved},
	} {
		if !containsMark(snap.Marks.Objects[card], want) {
			t.Errorf("card %s marks = %+v, want %+v among them", card, snap.Marks.Objects[card], want)
		}
	}
	if got, want := snap.Marks.Stubs["2fa-x"], (wallMark{Concern: "review/blocker/stub-unreconciled/s-2fa-x", Chip: markChipUnresolved}); got != want {
		t.Errorf("stub 2fa-x mark = %+v, want %+v", got, want)
	}

	readiness, err := s.readinessLoader.Load(context.Background(), "spec/"+marksWallName)
	if err != nil {
		t.Fatal(err)
	}
	proj, _, asd, err := s.loadASD(context.Background(), marksWallName)
	if err != nil {
		t.Fatal(err)
	}
	alone := deriveWallMarks(wallMarksInputFor(marksWallName, proj, asd, &readiness, nil))
	if !reflect.DeepEqual(snap.Marks, &alone) {
		t.Fatalf("the served marks differ from a derivation over a load alone:\n got: %+v\nwant: %+v", snap.Marks, alone)
	}

	if page, _ := pageRevision(t, h, path); page != snap.Revision {
		t.Fatalf("the page embeds revision %s, the poll serves %s: the client's first poll could never be a 304", page, snap.Revision)
	}
	warm := &readcensus.Census{}
	if rec, _ := pollWall(t, gitx.WithObserver(context.Background(), warm), h, path, snap.Revision); rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Fatalf("the client's next poll with the body's revision = %d (%d bytes), want an empty 304", rec.Code, rec.Body.Len())
	}
	checkRefreshBudget(t, "the served composed poll, warm", warm.Budget(acc))
}

func containsMark(marks []wallMark, want wallMark) bool {
	for _, m := range marks {
		if m == want {
			return true
		}
	}
	return false
}

// swapLoader is a test-only ReadinessLoader whose snapshot a test swaps
// between polls, and which counts its loads.
type swapLoader struct {
	mu    sync.Mutex
	snap  readinesspilot.Snapshot
	err   error
	loads []string
}

func (l *swapLoader) Load(_ context.Context, ref string) (readinesspilot.Snapshot, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.loads = append(l.loads, ref)
	return l.snap, l.err
}

func (l *swapLoader) set(snap readinesspilot.Snapshot, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.snap, l.err = snap, err
}

// TestWallMarks_ReadinessAloneMovesThePoll (SI-360 (2); Wave 6 §5.1): with
// the wall's own projection unchanged, a change to its readiness alone —
// one that moves a mark, one that moves no mark, and a load that starts
// failing — changes the composed revision, so the client's next poll,
// sent with the revision the last body carried, is a 200 with the new
// facts; and an unchanged readiness answers that poll 304.
func TestWallMarks_ReadinessAloneMovesThePoll(t *testing.T) {
	root := newMarksWallFixture(t)
	head := gitOut(t, root, "rev-parse", "HEAD")
	base := readinesspilot.Snapshot{TargetRef: "spec/" + marksWallName, Branch: "design/" + marksWallName, Head: head,
		Attention: []readinesspilot.Concern{{ID: "success/coverage/ac-2", Object: "ac-2"}}}
	moved := base
	moved.Attention = []readinesspilot.Concern{{ID: "shape/question/oq-1", Object: "oq-1"}}
	unmarked := base
	unmarked.StaleNotice = "Derived at HEAD " + head + " for this request."

	loader := &swapLoader{snap: base}
	h := NewHandlerWith(root, Deps{Design: readinessGapCapsBridge(), ReadinessLoader: loader})
	path := "/board/spec/" + marksWallName
	_, first := pollWall(t, context.Background(), h, path, "")
	if want := (wallMarks{Objects: map[string][]wallMark{"ac-2": {{Concern: "success/coverage/ac-2", Chip: markChipNoStub}}}}); !reflect.DeepEqual(first.Marks, &want) {
		t.Fatalf("first marks = %+v, want %+v", first.Marks, want)
	}
	if rec, _ := pollWall(t, context.Background(), h, path, first.Revision); rec.Code != http.StatusNotModified {
		t.Fatalf("an unchanged readiness answered the client's next poll %d, want 304", rec.Code)
	}

	last := first
	for _, step := range []struct {
		name string
		snap readinesspilot.Snapshot
		err  error
	}{
		{name: "a readiness change that moves a mark", snap: moved},
		{name: "a readiness change that moves no mark", snap: unmarked},
		{name: "a readiness load that fails", err: errBoom},
	} {
		loader.set(step.snap, step.err)
		rec, next := pollWall(t, context.Background(), h, path, last.Revision)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: the client's next poll = %d, want 200 with the new readiness", step.name, rec.Code)
		}
		if next.Revision == last.Revision || next.HTML != first.HTML {
			t.Fatalf("%s: revision %s -> %s, region changed %v: want a new token over an unchanged region", step.name, last.Revision, next.Revision, next.HTML != first.HTML)
		}
		if rec, _ := pollWall(t, context.Background(), h, path, next.Revision); rec.Code != http.StatusNotModified {
			t.Fatalf("%s: the poll after it = %d, want 304", step.name, rec.Code)
		}
		last = next
	}
	if want := unavailableMarks("the readiness load failed: " + errBoom.Error()); !reflect.DeepEqual(last.Marks, &want) {
		t.Fatalf("a failed load's marks = %+v, want %+v", last.Marks, want)
	}
}

// TestWallMarks_PageCarriesNoMarkup (SI-360 (4); brief item 5): the marks
// and the unavailable notice are render data only. A wall page and its
// snapshot whose marks are readable, and the same wall's whose marks are
// unavailable, render the same bytes but for the page's embedded revision.
func TestWallMarks_PageCarriesNoMarkup(t *testing.T) {
	root := newMarksWallFixture(t)
	path := "/board/spec/" + marksWallName
	marked := refreshServer(root)
	withMarks := NewHandlerWith(root, Deps{Design: marked.design, ReadinessLoader: marked.readinessLoader})
	unwired := NewHandlerWith(root, Deps{Design: marked.design})

	markedRev, markedPage := pageRevision(t, withMarks, path)
	plainRev, plainPage := pageRevision(t, unwired, path)
	if markedRev == plainRev {
		t.Fatal("the composed page's revision does not cover its readiness")
	}
	if strings.ReplaceAll(markedPage, markedRev, plainRev) != plainPage {
		t.Fatal("the page's bytes differ beyond its embedded revision: the marks reached the markup")
	}
	_, markedSnap := pollWall(t, context.Background(), withMarks, path, "")
	_, plainSnap := pollWall(t, context.Background(), unwired, path, "")
	if markedSnap.HTML != plainSnap.HTML {
		t.Fatal("the snapshot's region differs with the marks: the marks reached the markup")
	}
	if markedSnap.Marks == nil || markedSnap.Marks.Unavailable != "" || plainSnap.Marks == nil || plainSnap.Marks.Unavailable != marksUnwired {
		t.Fatalf("marks: composed %+v, unwired %+v", markedSnap.Marks, plainSnap.Marks)
	}
	if plainSnap.Revision != plainRev {
		t.Fatalf("the unwired wall's poll serves %s, its page embeds %s", plainSnap.Revision, plainRev)
	}
}

// TestWallMarks_BranchWallLoadsNoReadiness (SI-360 (3); BL-165 (4)): a /b/
// wall whose branch is not the serving root's loads no readiness and
// serves the unavailable notice's reason, in one read session with one
// accepted-HEAD resolution, at most one enumeration and no write; its
// page embeds the poll's revision, and the poll after it is a 304. A /b/
// address for the serving root's own branch is the serving wall, which
// composes its readiness.
func TestWallMarks_BranchWallLoadsNoReadiness(t *testing.T) {
	root := newMarksWallFixture(t)
	loader := &swapLoader{}
	h := NewHandlerWith(root, Deps{Design: readinessGapCapsBridge(), ReadinessLoader: loader})
	if _, err := wtmanager.EnsureWorktree(context.Background(), root, "design/marks-other"); err != nil {
		t.Fatalf("cutting the /b/ wall's worktree: %v", err)
	}
	path := "/b/design%2Fmarks-other/board/spec/" + marksWallName
	acc := originAccepted(t, root)

	for _, round := range []string{"cold", "warm"} {
		census := &readcensus.Census{}
		rec, snap := pollWall(t, gitx.WithObserver(context.Background(), census), h, path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s GET %s/snapshot = %d\n%s", round, path, rec.Code, rec.Body.String())
		}
		checkRefreshBudget(t, "the /b/ wall's poll, "+round, census.Budget(acc))
		if want := unavailableMarks(marksBranchWall("design/marks-other")); snap.Marks == nil || !reflect.DeepEqual(*snap.Marks, want) {
			t.Fatalf("the /b/ wall's marks = %+v, want %+v", snap.Marks, want)
		}
		if page, _ := pageRevision(t, h, path); page != snap.Revision {
			t.Fatalf("the /b/ page embeds %s, its poll serves %s", page, snap.Revision)
		}
		if rec, _ := pollWall(t, context.Background(), h, path, snap.Revision); rec.Code != http.StatusNotModified {
			t.Fatalf("the /b/ wall's next poll = %d, want 304", rec.Code)
		}
	}
	if len(loader.loads) != 0 {
		t.Fatalf("the /b/ wall loaded readiness for %q", loader.loads)
	}

	serving := "/b/" + strings.ReplaceAll("design/"+marksWallName, "/", "%2F") + "/board/spec/" + marksWallName
	if rec, _ := pollWall(t, context.Background(), h, serving, ""); rec.Code != http.StatusOK {
		t.Fatalf("GET %s/snapshot = %d", serving, rec.Code)
	}
	if want := []string{"spec/" + marksWallName}; !reflect.DeepEqual(loader.loads, want) {
		t.Fatalf("the serving root's own branch at /b/ loaded %q, want %q", loader.loads, want)
	}
}

// TestSealedASDView_MarksUnavailable (SI-352 (1)): a remote-only branch's
// sealed render carries the unavailable notice's reason, and no mark.
func TestSealedASDView_MarksUnavailable(t *testing.T) {
	v := sealedASDView("design/x", "origin/design/x", &BoardProjection{})
	if want := unavailableMarks(marksSealed("origin/design/x")); v.Marks == nil || !reflect.DeepEqual(*v.Marks, want) {
		t.Fatalf("sealed marks = %+v, want %+v", v.Marks, want)
	}
}

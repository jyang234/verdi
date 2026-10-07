package workbench

import (
	"context"
	"encoding/json"
	stdhtml "html"
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
	// The served region draws those facts (lane M-ui): each named card's
	// mark with its own word, and no unavailable notice.
	requireMarkMarkup(t, "the served poll's region", snap.HTML,
		[]string{servedMarkChip("ac-2", markChipNoStub), servedMarkChip("oq-1", markChipUnresolved), servedMarkChip("stub-2fa-x", markChipUnresolved), `data-testid="readiness-dot-ac-2"`},
		[]string{`data-testid="wall-marks-unavailable"`})

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

	page, body := pageRevision(t, h, path)
	if page != snap.Revision {
		t.Fatalf("the page embeds revision %s, the poll serves %s: the client's first poll could never be a 304", page, snap.Revision)
	}
	// The page's region is the poll's: it draws the same marks.
	requireMarkMarkup(t, "the page's region", body,
		[]string{servedMarkChip("ac-2", markChipNoStub), servedMarkChip("oq-1", markChipUnresolved), servedMarkChip("stub-2fa-x", markChipUnresolved)},
		[]string{`data-testid="wall-marks-unavailable"`})
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

// servedMarkChip is the start of one paper's served mark markup through
// its first chip's word (wallmarksrender.go).
func servedMarkChip(owner, word string) string {
	return `<span class="readiness-mark" data-testid="readiness-mark-` + owner + `"><span class="readiness-chip" data-mark="` + word + `"`
}

// requireMarkMarkup asserts a served region carries each wanted mark
// substring and none of the absent ones.
func requireMarkMarkup(t *testing.T, label, html string, want, absent []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(html, w) {
			t.Errorf("%s lacks %s", label, w)
		}
	}
	for _, a := range absent {
		if strings.Contains(html, a) {
			t.Errorf("%s carries %s", label, a)
		}
	}
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
// facts drawn in its region (lane M-ui: a moved mark moves the region, a
// change that moves no mark moves the token over the region the same
// marks drew before, and a failed load draws the notice); and an
// unchanged readiness answers that poll 304.
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

	ac2Mark, oq1Mark, notice := servedMarkChip("ac-2", markChipNoStub), servedMarkChip("oq-1", markChipUnresolved), `data-testid="wall-marks-unavailable"`
	requireMarkMarkup(t, "the first body's region", first.HTML, []string{ac2Mark}, []string{oq1Mark, notice})

	last := first
	for _, step := range []struct {
		name        string
		snap        readinesspilot.Snapshot
		err         error
		want, never string // the region's markup after the step
		sameAsFirst bool   // the region is the first body's: the same marks drawn again
	}{
		{name: "a readiness change that moves a mark", snap: moved, want: oq1Mark, never: ac2Mark},
		{name: "a readiness change that moves no mark", snap: unmarked, want: ac2Mark, never: oq1Mark, sameAsFirst: true},
		{name: "a readiness load that fails", err: errBoom, want: notice, never: `class="readiness-mark"`},
	} {
		loader.set(step.snap, step.err)
		rec, next := pollWall(t, context.Background(), h, path, last.Revision)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: the client's next poll = %d, want 200 with the new readiness", step.name, rec.Code)
		}
		if next.Revision == last.Revision || next.Revision == first.Revision {
			t.Fatalf("%s: revision %s -> %s: want a new token", step.name, last.Revision, next.Revision)
		}
		requireMarkMarkup(t, step.name+": the region", next.HTML, []string{step.want}, []string{step.never})
		if (next.HTML == first.HTML) != step.sameAsFirst {
			t.Fatalf("%s: the region is the first body's: %v, want %v", step.name, next.HTML == first.HTML, step.sameAsFirst)
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

// TestWallMarks_UnwiredWallDrawsTheNotice (SI-362 (4); lane M-ui, which
// retired M-go's placeholder TestWallMarks_PageCarriesNoMarkup): a server
// with no readiness loader wired serves the wall with the one unavailable
// notice naming that reason, and no mark, on its page and in its poll
// alike; a mutation's response, which composes no marks, carries neither.
func TestWallMarks_UnwiredWallDrawsTheNotice(t *testing.T) {
	root := newMarksWallFixture(t)
	path := "/board/spec/" + marksWallName
	unwired := NewHandlerWith(root, Deps{Design: readinessGapCapsBridge()})

	rev, body := pageRevision(t, unwired, path)
	notice := `<div class="board-notice wall-marks-notice" data-testid="wall-marks-unavailable" role="status">The readiness marks are unavailable: ` + marksUnwired + `.</div>`
	requireMarkMarkup(t, "the unwired page", body, []string{notice}, []string{`class="readiness-mark"`, `class="readiness-dot"`})
	if strings.Count(body, `data-testid="wall-marks-unavailable"`) != 1 {
		t.Fatalf("the unwired page draws %d notices", strings.Count(body, `data-testid="wall-marks-unavailable"`))
	}
	_, snap := pollWall(t, context.Background(), unwired, path, "")
	requireMarkMarkup(t, "the unwired poll's region", snap.HTML, []string{notice}, []string{`class="readiness-mark"`, `class="readiness-dot"`})
	if snap.Marks == nil || snap.Marks.Unavailable != marksUnwired || snap.Revision != rev {
		t.Fatalf("the unwired poll's marks = %+v at %s, the page embeds %s", snap.Marks, snap.Revision, rev)
	}

	// The fragment — the region a mutation swaps in — carries the same
	// instance-fixed notice, loaded from no readiness (SI-364 (3)), so it
	// is the poll's region byte for byte.
	rec := httptest.NewRecorder()
	unwired.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path+"/fragment", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET fragment = %d\n%s", rec.Code, rec.Body.String())
	}
	fragment := rec.Body.String()
	requireMarkMarkup(t, "the fragment", fragment, []string{notice}, []string{`class="readiness-mark"`, `class="readiness-dot"`})
	if fragment != snap.HTML {
		t.Fatal("the unwired fragment is not the poll's region")
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
		// The region draws the one notice with that reason, and no mark
		// (SI-350 (2)), on the poll and the page alike.
		notice := `data-testid="wall-marks-unavailable" role="status">The readiness marks are unavailable: ` + stdhtml.EscapeString(marksBranchWall("design/marks-other")) + `.</div>`
		requireMarkMarkup(t, round+": the /b/ poll's region", snap.HTML, []string{notice}, []string{`class="readiness-mark"`, `class="readiness-dot"`})
		page, body := pageRevision(t, h, path)
		if page != snap.Revision {
			t.Fatalf("the /b/ page embeds %s, its poll serves %s", page, snap.Revision)
		}
		requireMarkMarkup(t, round+": the /b/ page", body, []string{notice}, []string{`class="readiness-mark"`, `class="readiness-dot"`})
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

// saveAndPoll posts one typed edit of ac-1 at mount (a wall address)
// against the wall checked out at wallRoot, and returns the mutation
// response's fresh projection: its revision and region.
func saveAndPoll(t *testing.T, h http.Handler, mount, wallRoot string) (revision, html string) {
	t.Helper()
	body := mutateEnvelope(t, wallRoot, marksWallName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "a stale notice is retracted, today", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, mount+"/api/mutate_draft", strings.NewReader(body)))
	var out struct {
		Result     json.RawMessage     `json:"result"`
		Projection *mutationProjection `json:"projection"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.Result == nil || out.Projection == nil {
		t.Fatalf("POST %s mutate_draft = %d, want a landed save with its fresh projection\n%s", mount, rec.Code, rec.Body.String())
	}
	return out.Projection.Revision, out.Projection.HTML
}

// TestWallMarks_FixedUnavailableSaveMatchesThePoll (SI-364 (3); SI-362
// (1)): on a wall whose marks are fixed for the server instance — no
// loader wired, or a /b/ wall whose branch is not the serving root's — a
// save's fresh projection and the fragment carry the poll's one
// unavailable notice, computed without a readiness load, so the save's
// revision is the token the next poll answers 304 to, and its region is
// the poll's. The sealed render has no save and no poll route; its
// fragment and page carry the same notice.
func TestWallMarks_FixedUnavailableSaveMatchesThePoll(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason string
		serve  func(t *testing.T, root string) (h http.Handler, mount, wallRoot string, loader *swapLoader)
	}{
		{
			name:   "unwired",
			reason: marksUnwired,
			serve: func(_ *testing.T, root string) (http.Handler, string, string, *swapLoader) {
				return NewHandlerWith(root, Deps{Design: testDesignBridge{}}), "/board/spec/" + marksWallName, root, nil
			},
		},
		{
			name:   "a /b/ branch wall",
			reason: marksBranchWall("design/" + marksWallName),
			serve: func(t *testing.T, root string) (http.Handler, string, string, *swapLoader) {
				gitOut(t, root, "checkout", "--quiet", "design/marks-other")
				wt, err := wtmanager.EnsureWorktree(context.Background(), root, "design/"+marksWallName)
				if err != nil {
					t.Fatalf("cutting the /b/ wall's worktree: %v", err)
				}
				loader := &swapLoader{}
				h := NewHandlerWith(root, Deps{Design: testDesignBridge{}, ReadinessLoader: loader})
				return h, "/b/design%2F" + marksWallName + "/board/spec/" + marksWallName, wt, loader
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, mount, wallRoot, loader := tc.serve(t, newMarksWallFixture(t))
			want := unavailableMarks(tc.reason)
			saved, savedHTML := saveAndPoll(t, h, mount, wallRoot)
			if !strings.Contains(savedHTML, `data-testid="wall-marks-unavailable"`) || !strings.Contains(savedHTML, stdhtml.EscapeString(tc.reason)) {
				t.Fatalf("the save's region lacks the notice %q", tc.reason)
			}
			rec, _ := pollWall(t, context.Background(), h, mount, saved)
			if rec.Code != http.StatusNotModified {
				t.Fatalf("the poll after a save, sent the save's revision, = %d, want 304: the save and the poll disagree on the token", rec.Code)
			}
			_, snap := pollWall(t, context.Background(), h, mount, "")
			if snap.Revision != saved || snap.HTML != savedHTML || snap.Marks == nil || !reflect.DeepEqual(*snap.Marks, want) {
				t.Fatalf("the poll = (%s, marks %+v), the save = %s: want one token over one region", snap.Revision, snap.Marks, saved)
			}
			frag := httptest.NewRecorder()
			h.ServeHTTP(frag, httptest.NewRequestWithContext(t.Context(), http.MethodGet, mount+"/fragment", nil))
			if frag.Code != http.StatusOK || frag.Body.String() != snap.HTML {
				t.Fatalf("the fragment (%d) differs from the poll's region", frag.Code)
			}
			if loader != nil && len(loader.loads) != 0 {
				t.Fatalf("the /b/ wall loaded readiness for %q", loader.loads)
			}
		})
	}

	t.Run("sealed", func(t *testing.T) {
		root := newBranchBoardFixture(t)
		h := NewHandler(root)
		mount := "/b/design%2Fremote-only/board/spec/remote-spec"
		page := bGet(t, h, mount)
		frag := bGet(t, h, mount+"/fragment")
		if page.Code != http.StatusOK || frag.Code != http.StatusOK {
			t.Fatalf("sealed page = %d, fragment = %d", page.Code, frag.Code)
		}
		reason := marksSealed("origin/design/remote-only")
		if !strings.Contains(frag.Body.String(), stdhtml.EscapeString(reason)) || strings.Count(frag.Body.String(), `data-testid="wall-marks-unavailable"`) != 1 {
			t.Fatal("the sealed fragment lacks the one sealed notice")
		}
		if !strings.Contains(page.Body.String(), frag.Body.String()) {
			t.Fatal("the sealed page's region is not the fragment's")
		}
	})
}

package workbench

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// TestDeriveWallPill (SI-368 (2), (24)(b)): the pill's facts from a
// composed refresh's readiness are the current step and that step's own
// unresolved count — never the whole Focus next list across every step —
// or the marks' input reason when that readiness cannot be read; a stub
// collision, which only the marks cannot draw, leaves the pill readable.
// tabSnapshot's unresolved concerns sit in three steps: two in Define the
// work, one in Define success, two in Get approval, none in Check
// constraints.
func TestDeriveWallPill(t *testing.T) {
	head := "4f1c9d2ab7e0"
	readable := tabSnapshot(head)
	in := func(snap *readinesspilot.Snapshot, err error, stubs ...string) wallMarksInput {
		return wallMarksInput{Ref: "spec/" + marksWallName, Branch: "design/" + marksWallName, Head: head,
			ObjectIDs: []string{"ac-1", "ac-2", "oq-1"}, StubSlugs: stubs, Readiness: snap, LoadErr: err}
	}
	proven := readable
	proven.CurrentFocus, proven.Attention = "", nil
	later := readable
	later.CurrentFocus = readinesspilot.AreaReview
	empty := readable
	empty.CurrentFocus = readinesspilot.AreaContext
	moved := readable
	moved.Head = "0000000"
	for _, tc := range []struct {
		name string
		in   wallMarksInput
		want wallPill
	}{
		{"the current step's unresolved concerns alone", in(&readable, nil, "2fa-x"), wallPill{Step: 1, Unresolved: 2}},
		{"a later current step counts its own", in(&later, nil), wallPill{Step: 4, Unresolved: 2}},
		{"an empty current step is 0, never the total", in(&empty, nil), wallPill{Step: 3, Unresolved: 0}},
		{"every step proven", in(&proven, nil), wallPill{}},
		{"a failed load", in(nil, errBoom), wallPill{Unavailable: "the readiness load failed: " + errBoom.Error()}},
		{"nothing loaded", in(nil, nil), wallPill{Unavailable: marksUnwired}},
		{"a snapshot of another HEAD", in(&moved, nil), wallPill{Unavailable: deriveWallMarks(in(&moved, nil)).Unavailable}},
		{"a stub collision the marks cannot draw", in(&readable, nil, "2fa-x", "s-2fa-x"), wallPill{Step: 1, Unresolved: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveWallPill(tc.in); got != tc.want {
				t.Fatalf("pill = %+v, want %+v", got, tc.want)
			}
		})
	}
	if marks := deriveWallMarks(in(&readable, nil, "2fa-x", "s-2fa-x")); marks.Unavailable == "" {
		t.Fatal("the collision fixture leaves the marks readable: the case above would be vacuous")
	}
	if got := deriveWallPill(in(&moved, nil)); got.Unavailable == "" || !strings.Contains(got.Unavailable, "0000000") {
		t.Fatalf("the moved-HEAD pill = %+v, want the marks' identity reason", got)
	}
}

// TestWallPill_RevisionCoversThePill (SI-368 (2); SI-362 (1)): the pill
// enters the composed revision as the marks do — the same readiness,
// failure and marks with another pill digest to another token.
func TestWallPill_RevisionCoversThePill(t *testing.T) {
	snap := tabSnapshot("4f1c9d2ab7e0")
	marks := wallMarks{}
	a, err := readinessRevision(&snap, nil, &marks, &wallPill{Step: 1, Unresolved: 5})
	if err != nil {
		t.Fatal(err)
	}
	b, err := readinessRevision(&snap, nil, &marks, &wallPill{Step: 1, Unresolved: 4})
	if err != nil {
		t.Fatal(err)
	}
	again, err := readinessRevision(&snap, nil, &marks, &wallPill{Step: 1, Unresolved: 5})
	if err != nil {
		t.Fatal(err)
	}
	if a == b || a != again {
		t.Fatalf("revisions %s, %s, %s: want the pill alone to move the token, deterministically", a, b, again)
	}
}

// pillWire is the part of a snapshot or mutation projection the pill
// rides on.
type pillWire struct {
	Revision string    `json:"revision"`
	Pill     *wallPill `json:"pill"`
}

// TestWallPill_ServedOnTheComposedPollAlone (SI-368 (2); SI-362 (2)): on
// a wall served from the serving root, the composed poll carries the
// pill derived from the readiness it already loaded — no extra load —
// and the page's composed view carries the same; a save's fresh
// projection, which composes no readiness, carries none, so the pill
// stays as the last poll left it.
func TestWallPill_ServedOnTheComposedPollAlone(t *testing.T) {
	root := newMarksWallFixture(t)
	head := gitOut(t, root, "rev-parse", "HEAD")
	snap := tabSnapshot(head)
	loader := &swapLoader{snap: snap}
	h := NewHandlerWith(root, Deps{Design: testDesignBridge{}, ReadinessLoader: loader})
	path := "/board/spec/" + marksWallName

	rec := tabGet(t, t.Context(), h, path+"/snapshot")
	var poll pillWire
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &poll) != nil {
		t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
	}
	// Define the work holds two of tabSnapshot's five unresolved concerns
	// (SI-368 (24)(b)).
	if want := (wallPill{Step: 1, Unresolved: 2}); poll.Pill == nil || *poll.Pill != want {
		t.Fatalf("the poll's pill = %+v, want %+v", poll.Pill, want)
	}
	if len(loader.loads) != 1 {
		t.Fatalf("the composed poll loaded readiness %d times, want once", len(loader.loads))
	}

	s := &boardSpecServer{root: root, design: testDesignBridge{}, readinessLoader: loader}
	wall, err := s.composeWall(context.Background(), marksWallName, true)
	if err != nil {
		t.Fatal(err)
	}
	if wall.asd.Pill == nil || *wall.asd.Pill != *poll.Pill {
		t.Fatalf("the page's composed view carries pill %+v, the poll %+v", wall.asd.Pill, poll.Pill)
	}

	body := mutateEnvelope(t, root, marksWallName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "a stale notice is retracted, now", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)
	save := httptest.NewRecorder()
	h.ServeHTTP(save, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path+"/api/mutate_draft", strings.NewReader(body)))
	var out struct {
		Projection json.RawMessage `json:"projection"`
	}
	if save.Code != http.StatusOK || json.Unmarshal(save.Body.Bytes(), &out) != nil || out.Projection == nil {
		t.Fatalf("POST mutate_draft = %d\n%s", save.Code, save.Body.String())
	}
	if strings.Contains(string(out.Projection), `"pill"`) {
		t.Fatalf("a serving-root save's fresh projection carries a pill: %s", out.Projection)
	}
}

// TestWallPill_FixedWallsCarryTheMarksReason (SI-364 (3); SI-362 (1)):
// where the marks are fixed for the server instance — no loader wired,
// or a /b/ wall whose branch is not the serving root's — the poll and a
// save's fresh projection carry one pill, the marks' reason, under one
// token, with no readiness load; the sealed render's view carries its
// own reason.
func TestWallPill_FixedWallsCarryTheMarksReason(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason string
		serve  func(t *testing.T, root string) (http.Handler, string, string, *swapLoader)
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
				return NewHandlerWith(root, Deps{Design: testDesignBridge{}, ReadinessLoader: loader}), "/b/design%2F" + marksWallName + "/board/spec/" + marksWallName, wt, loader
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, mount, wallRoot, loader := tc.serve(t, newMarksWallFixture(t))
			want := wallPill{Unavailable: tc.reason}
			body := mutateEnvelope(t, wallRoot, marksWallName, []map[string]any{
				{"op": "edit-ac", "id": "ac-1", "text": "a stale notice is retracted, today", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
			}, nil, nil)
			save := httptest.NewRecorder()
			h.ServeHTTP(save, httptest.NewRequestWithContext(t.Context(), http.MethodPost, mount+"/api/mutate_draft", strings.NewReader(body)))
			var out struct {
				Projection *pillWire `json:"projection"`
			}
			if save.Code != http.StatusOK || json.Unmarshal(save.Body.Bytes(), &out) != nil || out.Projection == nil {
				t.Fatalf("POST mutate_draft = %d\n%s", save.Code, save.Body.String())
			}
			if out.Projection.Pill == nil || *out.Projection.Pill != want {
				t.Fatalf("the save's pill = %+v, want %+v", out.Projection.Pill, want)
			}
			rec := tabGet(t, t.Context(), h, mount+"/snapshot")
			var poll pillWire
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &poll) != nil {
				t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
			}
			if poll.Pill == nil || *poll.Pill != want || poll.Revision != out.Projection.Revision {
				t.Fatalf("the poll = (%s, pill %+v), the save = (%s, pill %+v): want one token and one pill", poll.Revision, poll.Pill, out.Projection.Revision, out.Projection.Pill)
			}
			if loader != nil && len(loader.loads) != 0 {
				t.Fatalf("the fixed wall loaded readiness for %q", loader.loads)
			}
		})
	}
	t.Run("sealed", func(t *testing.T) {
		v := sealedASDView("design/x", "origin/design/x", &BoardProjection{})
		if want := (wallPill{Unavailable: marksSealed("origin/design/x")}); v.Pill == nil || *v.Pill != want {
			t.Fatalf("the sealed view's pill = %+v, want %+v", v.Pill, want)
		}
	})
}

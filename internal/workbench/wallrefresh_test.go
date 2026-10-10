package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/gitx/readcensus"
	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/specstate"
)

// refreshWall is one wall the composed-refresh witness serves: its root,
// its spec, and the accepted HEAD a census counts against.
type refreshWall struct {
	root, name string
	acc        readcensus.Accepted
}

// originAccepted names root's origin/main for a census, read without one.
func originAccepted(t *testing.T, root string) readcensus.Accepted {
	t.Helper()
	tip := strings.TrimSpace(gitOut(t, root, "rev-parse", "--verify", "origin/main"))
	return readcensus.Accepted{Spellings: []string{"origin/main", "refs/remotes/origin/main"}, IDs: []string{tip}}
}

// refreshWalls are a proposed wall on its design branch (the claim-wall
// fixture with a store manifest) and an accepted feature wall whose bytes
// are origin/main's, served from a feature branch — each with origin/main
// as the default branch, discovered through origin/HEAD.
func refreshWalls(t *testing.T) []refreshWall {
	t.Helper()
	proposed := newClaimWallFixture(t)
	addClaimWallManifest(t, proposed)
	gitOut(t, proposed, "update-ref", "refs/remotes/origin/main", "refs/heads/main")

	neutralizeCIEnv(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{Message: "adopt store with one accepted feature", Files: map[string]string{
		".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
		".verdi/specs/active/" + documentWallName + "/spec.md": documentWallSpec,
	}}})
	gitOut(t, repo.Dir, "update-ref", "refs/remotes/origin/main", repo.Head)
	gitOut(t, repo.Dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	gitOut(t, repo.Dir, "checkout", "--quiet", "-b", "feature/work")

	return []refreshWall{
		{root: proposed, name: claimWallName, acc: originAccepted(t, proposed)},
		{root: repo.Dir, name: documentWallName, acc: originAccepted(t, repo.Dir)},
	}
}

// refreshServer is the wall's server as `verdi serve` wires it: the design
// bridge and the production readiness loader.
func refreshServer(root string) *boardSpecServer {
	return &boardSpecServer{
		root:            root,
		design:          readinessGapCapsBridge(),
		readinessLoader: readinessload.Loader{Root: root, Opts: readinessload.Options{BoardHref: BranchBoardHref}},
	}
}

func marshalJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestProjectWallRefresh_ComposedBudget is the wall's composition seam's
// structural witness (Wave 6 §5.3; ledger SI-168, SI-356, SI-360 (2)): a
// refresh that composes the spec's readiness load with the wall's
// snapshot — what the marks need — is one application projection: one
// read session, one accepted-HEAD resolution (one discovery, one explicit
// resolution, no read naming the ref again), at most one accepted-tree
// enumeration, and no write, on the first refresh in a fresh repository
// and on the next. Its bytes are the parts' own: the snapshot is the one
// an unscoped load renders but for the marks and the revision covering
// them, and the readiness the one a load alone derives.
func TestProjectWallRefresh_ComposedBudget(t *testing.T) {
	for _, wall := range refreshWalls(t) {
		t.Run(wall.name, func(t *testing.T) {
			s := refreshServer(wall.root)
			for _, round := range []string{"first refresh", "next refresh"} {
				census := &readcensus.Census{}
				refresh, err := s.projectWallRefresh(gitx.WithObserver(context.Background(), census), wall.name, true)
				if err != nil {
					t.Fatalf("%s: %v", round, err)
				}
				if refresh.readiness == nil || refresh.readinessErr != nil {
					t.Fatalf("%s composed no readiness: %v", round, refresh.readinessErr)
				}
				checkRefreshBudget(t, round, census.Budget(wall.acc))
			}

			refresh, err := s.projectWallRefresh(context.Background(), wall.name, true)
			if err != nil {
				t.Fatal(err)
			}
			alone, err := s.loadSnapshot(context.Background(), wall.name)
			if err != nil {
				t.Fatal(err)
			}
			if refresh.snap.Marks == nil || refresh.snap.Revision == alone.Revision {
				t.Fatalf("the composed snapshot carries marks %+v and revision %s (an unscoped load's: %s)", refresh.snap.Marks, refresh.snap.Revision, alone.Revision)
			}
			// The marks reach the region's markup (lane M-ui): the composed
			// snapshot is an unscoped load rendered with the refresh's marks
			// and pill (SI-368 (2)), but for the revision covering them.
			proj, git, asd, err := s.loadASD(context.Background(), wall.name)
			if err != nil {
				t.Fatal(err)
			}
			if refresh.snap.Pill == nil || refresh.snap.Pill.Unavailable != "" {
				t.Fatalf("the composed snapshot carries pill %+v, want readable facts", refresh.snap.Pill)
			}
			asd.Marks, asd.Pill = refresh.snap.Marks, refresh.snap.Pill
			marked := newASDSnapshot(proj, git, asd)
			bare := *refresh.snap
			bare.Revision = marked.Revision
			if got, want := marshalJSON(t, bare), marshalJSON(t, marked); got != want {
				t.Fatalf("the refresh's snapshot differs from an unscoped load's rendered with its marks, beyond the revision:\n got: %s\nwant: %s", got, want)
			}
			if m := refresh.snap.Marks; (m.Unavailable != "" || len(m.Objects)+len(m.Stubs) > 0) && marked.HTML == alone.HTML {
				t.Fatalf("marks %+v never reached the region", m)
			}
			readiness, err := s.readinessLoader.Load(context.Background(), "spec/"+wall.name)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := marshalJSON(t, refresh.readiness), marshalJSON(t, readiness); got != want {
				t.Fatalf("the refresh's readiness differs from a load alone's:\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// TestProjectWallRefresh_SnapshotRouteServesTheComposedRefresh: a plain
// refresh is exactly the snapshot an unscoped load renders, and the
// /snapshot route serves the composed refresh (SI-360 (2)): the bytes of
// the composed snapshot, its revision token as the ETag, in one
// application projection.
func TestProjectWallRefresh_SnapshotRouteServesTheComposedRefresh(t *testing.T) {
	for _, wall := range refreshWalls(t) {
		t.Run(wall.name, func(t *testing.T) {
			s := refreshServer(wall.root)
			plain, err := s.projectWallRefresh(context.Background(), wall.name, false)
			if err != nil {
				t.Fatal(err)
			}
			if plain.readiness != nil || plain.readinessErr != nil || plain.snap.Marks != nil {
				t.Fatal("a plain refresh composed readiness")
			}
			alone, err := s.loadSnapshot(context.Background(), wall.name)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := marshalJSON(t, plain.snap), marshalJSON(t, alone); got != want {
				t.Fatalf("a plain refresh's snapshot differs from an unscoped load's:\n got: %s\nwant: %s", got, want)
			}
			composed, err := s.projectWallRefresh(context.Background(), wall.name, true)
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			for _, rt := range boardSpecRoutes() {
				mux.HandleFunc(rt.suffix, rt.handler(s))
			}
			census := &readcensus.Census{}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequestWithContext(gitx.WithObserver(context.Background(), census), http.MethodGet, "/board/spec/"+wall.name+"/snapshot", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
			}
			want := httptest.NewRecorder()
			writeJSON(want, http.StatusOK, composed.snap)
			if rec.Body.String() != want.Body.String() || rec.Header().Get("ETag") != `"`+composed.snap.Revision+`"` {
				t.Fatalf("the served snapshot differs from the composed refresh's:\n got: %s\nwant: %s", rec.Body.String(), want.Body.String())
			}
			checkRefreshBudget(t, "the served poll", census.Budget(wall.acc))
		})
	}
}

// TestProjectWallRefresh_NoLoaderNoReadiness: with no loader wired, a
// composed refresh is the snapshot alone.
func TestProjectWallRefresh_NoLoaderNoReadiness(t *testing.T) {
	wall := refreshWalls(t)[0]
	s := &boardSpecServer{root: wall.root, design: readinessGapCapsBridge()}
	refresh, err := s.projectWallRefresh(context.Background(), wall.name, true)
	if err != nil || refresh.snap == nil || refresh.readiness != nil || refresh.readinessErr != nil {
		t.Fatalf("refresh = (%+v, %v), want the snapshot alone", refresh, err)
	}
	if want := unavailableMarks(marksUnwired); refresh.snap.Marks == nil || refresh.snap.Marks.Unavailable != want.Unavailable || refresh.snap.Marks.Objects != nil || refresh.snap.Marks.Stubs != nil {
		t.Fatalf("an unwired wall's marks = %+v, want the notice %+v", refresh.snap.Marks, want)
	}
	if _, err := s.projectWallRefresh(context.Background(), "no-such-wall", true); err == nil {
		t.Fatal("a refresh of a wall that does not exist succeeded")
	}
}

// TestProjectWallRefresh_ReadinessFailureIsCarried: a loader failure is
// the refresh's readinessErr, never the snapshot's failure.
func TestProjectWallRefresh_ReadinessFailureIsCarried(t *testing.T) {
	wall := refreshWalls(t)[0]
	s := &boardSpecServer{root: wall.root, design: readinessGapCapsBridge(), readinessLoader: erroringReadinessLoader{err: errBoom}}
	refresh, err := s.projectWallRefresh(context.Background(), wall.name, true)
	if err != nil || refresh.snap == nil || refresh.readiness != nil || refresh.readinessErr != errBoom {
		t.Fatalf("refresh = (%+v, %v), want the snapshot and the loader's error", refresh, err)
	}
}

var errBoom = errors.New("boom")

// checkRefreshBudget asserts one refresh's §5.3 budget.
func checkRefreshBudget(t *testing.T, label string, b readcensus.Budget) {
	t.Helper()
	t.Logf("%s: processes %d, sessions %d, batches %d, chains %d, explicit %d, operand %d, enumerations %d (relisted %d), writes %d",
		label, b.Processes, b.Sessions, b.Batches, b.Chains, b.Explicit, len(b.Operand), len(b.Enumerations), len(b.Relisted), len(b.Writes))
	if b.Sessions != 1 {
		t.Errorf("%s opened %d read sessions, want one application projection", label, b.Sessions)
	}
	if chains, explicit, operand := b.Resolutions(); chains != 1 || explicit != 1 || operand != 0 {
		t.Errorf("%s resolved the accepted HEAD %d times by discovery, %d explicitly and %d as an operand, want 1, 1 and 0:\n  %s",
			label, chains, explicit, operand, strings.Join(b.Operand, "\n  "))
	}
	if len(b.Enumerations) > 1 {
		t.Errorf("%s enumerated the accepted tree %d times:\n  %s", label, len(b.Enumerations), strings.Join(b.Enumerations, "\n  "))
	}
	if len(b.Writes) != 0 {
		t.Errorf("%s wrote:\n  %s", label, strings.Join(b.Writes, "\n  "))
	}
}

// countingPostureReader counts the posture's rev-parses.
type countingPostureReader struct {
	gitPostureReader
	revs []string
}

func (c *countingPostureReader) RevParse(ctx context.Context, dir, rev string) (string, error) {
	c.revs = append(c.revs, rev)
	return c.gitPostureReader.RevParse(ctx, dir, rev)
}

// TestResolvePostureHeads_PinnedAcceptedHead (ledger SI-356): a checkout
// whose git state came from a pinned request states the pin's accepted
// HEAD without resolving the ref again; unpinned, the posture resolves it
// itself; an unresolved default branch states why.
func TestResolvePostureHeads_PinnedAcceptedHead(t *testing.T) {
	wall := refreshWalls(t)[1]
	s := &boardSpecServer{root: wall.root}
	tip := wall.acc.IDs[0]
	for _, tc := range []struct {
		name     string
		pinned   bool
		wantRevs []string
	}{
		{name: "pinned", pinned: true, wantRevs: []string{"HEAD"}},
		{name: "unpinned", wantRevs: []string{"HEAD", "origin/main"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.pinned {
				ctx = specstate.WithAcceptedHead(ctx, wall.root)
			}
			git, _, err := s.gitState(ctx)
			if err != nil {
				t.Fatal(err)
			}
			rd := &countingPostureReader{}
			heads := resolvePostureHeads(ctx, wall.root, git, rd)
			if heads.accepted != tip || heads.acceptedWhy != "" {
				t.Fatalf("accepted HEAD = (%q, %q), want %s", heads.accepted, heads.acceptedWhy, tip)
			}
			if strings.Join(rd.revs, " ") != strings.Join(tc.wantRevs, " ") {
				t.Fatalf("posture rev-parses = %q, want %q", rd.revs, tc.wantRevs)
			}
		})
	}
	unresolved := resolvePostureHeads(context.Background(), wall.root, &boardGitState{}, &countingPostureReader{})
	if unresolved.accepted != "" || unresolved.acceptedWhy != defaultBranchUnresolved {
		t.Fatalf("an unresolved default branch's heads = %+v", unresolved)
	}
}

// TestProjectWallRefresh_RevisionCoversReadiness (Wave 6 §5.1; review
// P2R-4; SI-360 (2)): a composed refresh's revision covers the readiness
// it composes — changing only the readiness changes the token, over an
// unchanged region when the change moves no mark, and a loader failure
// draws the unavailable notice in the region (lane M-ui) — while a
// plain refresh's token is an unscoped load's revision, byte for byte,
// whatever the loader would say, and so is a composed refresh's that
// loaded none (its marks are then a constant of the server, which an
// unscoped load carries too: SI-364 (3)).
func TestProjectWallRefresh_RevisionCoversReadiness(t *testing.T) {
	wall := refreshWalls(t)[0]
	bare := &boardSpecServer{root: wall.root, design: readinessGapCapsBridge()}
	_, _, view, err := bare.loadASD(context.Background(), wall.name)
	if err != nil {
		t.Fatal(err)
	}
	// Two readable snapshots of this wall naming no card: the same (empty)
	// marks, so the same region, under two tokens (lane M-ui: the marks
	// reach the region, so only a change that moves no mark leaves it).
	snapA := readinesspilot.Snapshot{TargetRef: "spec/" + wall.name, Branch: view.Branch, Head: view.WorktreeHead}
	snapB := snapA
	snapB.StaleNotice = "Derived at HEAD " + view.WorktreeHead + " for this request."
	refreshWith := func(loader ReadinessLoader, compose bool) wallRefresh {
		t.Helper()
		s := &boardSpecServer{root: wall.root, design: readinessGapCapsBridge(), readinessLoader: loader}
		refresh, err := s.projectWallRefresh(context.Background(), wall.name, compose)
		if err != nil {
			t.Fatal(err)
		}
		return refresh
	}
	// alone is a plain load on a serving-root wall with a loader wired: no
	// readiness, no marks. unwired is the same load where no loader is
	// wired, which carries that instance's fixed notice (SI-364 (3)).
	alone, err := (&boardSpecServer{root: wall.root, design: readinessGapCapsBridge(), readinessLoader: fixedSnapshotLoader{snap: snapA}}).loadSnapshot(context.Background(), wall.name)
	if err != nil {
		t.Fatal(err)
	}
	if alone.Marks != nil {
		t.Fatalf("a plain load on a serving-root wall carries marks %+v", alone.Marks)
	}
	unwired, err := bare.loadSnapshot(context.Background(), wall.name)
	if err != nil {
		t.Fatal(err)
	}
	a := refreshWith(fixedSnapshotLoader{snap: snapA}, true)
	b := refreshWith(fixedSnapshotLoader{snap: snapB}, true)
	failed := refreshWith(erroringReadinessLoader{err: errBoom}, true)
	if a.snap.Marks == nil || a.snap.Marks.Unavailable != "" || len(a.snap.Marks.Objects)+len(a.snap.Marks.Stubs) != 0 {
		t.Fatalf("the readable snapshot's marks = %+v, want readable and empty", a.snap.Marks)
	}
	if a.snap.HTML != b.snap.HTML || a.snap.HTML != alone.HTML {
		t.Fatal("the region moved with a readiness change that moved no mark")
	}
	if failed.snap.HTML == alone.HTML || !strings.Contains(failed.snap.HTML, `data-testid="wall-marks-unavailable"`) {
		t.Fatal("a failed load's region carries no unavailable notice")
	}
	if a.snap.Revision == b.snap.Revision || a.snap.Revision == failed.snap.Revision || b.snap.Revision == failed.snap.Revision {
		t.Fatalf("a readiness change left the composed token unchanged: %s, %s, %s", a.snap.Revision, b.snap.Revision, failed.snap.Revision)
	}
	if a.snap.Revision == alone.Revision {
		t.Fatal("the composed token is the wall's alone")
	}
	if again := refreshWith(fixedSnapshotLoader{snap: snapA}, true); again.snap.Revision != a.snap.Revision {
		t.Fatalf("the same readiness derived two composed tokens: %s, %s", a.snap.Revision, again.snap.Revision)
	}
	for _, loader := range []ReadinessLoader{fixedSnapshotLoader{snap: snapA}, fixedSnapshotLoader{snap: snapB}, erroringReadinessLoader{err: errBoom}} {
		if plain := refreshWith(loader, false); plain.snap.Revision != alone.Revision {
			t.Fatalf("a plain refresh's token %s is not an unscoped load's revision %s", plain.snap.Revision, alone.Revision)
		}
	}
	if plain := refreshWith(nil, false); plain.snap.Revision != unwired.Revision {
		t.Fatalf("an unwired plain refresh's token %s is not an unwired load's %s", plain.snap.Revision, unwired.Revision)
	}
	// A composed refresh with no loader digests no readiness (SI-362 (1)):
	// its marks are the instance's fixed notice, which a plain load — the
	// mutation response's and the fragment's — carries too (SI-364 (3)),
	// so the poll and a save share one region and one token; that region
	// is the serving-root wall's plain one plus the one notice.
	none := refreshWith(nil, true)
	if none.asd.readinessRevision != "" {
		t.Fatalf("a composed refresh with no loader digested readiness %q", none.asd.readinessRevision)
	}
	if none.snap.Revision != unwired.Revision || none.snap.HTML != unwired.HTML {
		t.Fatalf("a composed refresh with no loader carries token %s, an unwired plain load (a save's) %s: want one token over one region", none.snap.Revision, unwired.Revision)
	}
	if !strings.Contains(none.snap.HTML, `data-testid="wall-marks-unavailable"`) || strings.Replace(none.snap.HTML, noticeOf(marksUnwired), "", 1) != alone.HTML {
		t.Fatal("the unwired region is not the plain serving-root region plus the one notice")
	}
}

// noticeOf is the unavailable notice's markup for one reason, inside the
// board's notices wrapper when the wall has no other notice.
func noticeOf(reason string) string {
	return `<div class="board-notices"><div class="board-notice wall-marks-notice" data-testid="wall-marks-unavailable" role="status">The readiness marks are unavailable: ` + reason + `.</div></div>`
}

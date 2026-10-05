package pollwitness

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/gitx/readcensus"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/workbench"
)

// budgetPath is one projection the structural witness counts. open
// builds its server once for a repository — one handler, or one loader,
// serving every request the test makes, as in production — and returns
// project, which runs the projection once under ctx (carrying the census)
// and returns the validator a conditional refresh sends next.
type budgetPath struct {
	name string
	open func(root string) projectFunc
}

type projectFunc func(t *testing.T, ctx context.Context, spec, etag string) string

// budgetPaths are the paths lane P2 makes hold Wave 6 §5.3 (ledger
// SI-356): the wall's plain poll, the Document page's poll (which composes
// the readiness load in production, BL-158), and the readiness load alone.
// The wall composed with the readiness load is the workbench's own seam,
// witnessed in that package (TestProjectWallRefresh_ComposedBudget).
func budgetPaths() []budgetPath {
	poll := func(route string) func(string) projectFunc {
		return func(root string) projectFunc {
			h := workbench.NewHandlerWith(root, workbench.Deps{ReadinessLoader: newLoader(root)})
			return func(t *testing.T, ctx context.Context, spec, etag string) string {
				t.Helper()
				req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/board/spec/"+spec+route, nil)
				want := http.StatusOK
				if etag != "" {
					req.Header.Set("If-None-Match", etag)
					want = http.StatusNotModified
				}
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != want {
					t.Fatalf("GET %s = %d, want %d\n%s", route, rec.Code, want, rec.Body.String())
				}
				return rec.Header().Get("ETag")
			}
		}
	}
	return []budgetPath{
		{name: "wall-poll", open: poll("/snapshot")},
		{name: "document-poll", open: poll("/document/snapshot")},
		{name: "readiness-load", open: func(root string) projectFunc {
			loader := newLoader(root)
			return func(t *testing.T, ctx context.Context, spec, _ string) string {
				t.Helper()
				if _, err := loader.Load(ctx, "spec/"+spec); err != nil {
					t.Fatalf("readiness load: %v", err)
				}
				return ""
			}
		}},
	}
}

// acceptedOf names root's accepted HEAD for a census: the ref specstate
// resolves, its full refname, and the ids it resolves to — read without a
// census, before any projection is counted. A default branch that does
// not resolve (the e2e fixture has no origin remote) names nothing, and
// explicit is then 0: a projection's one resolution is its one discovery,
// which finds no branch.
func acceptedOf(t *testing.T, root string) (acc readcensus.Accepted, explicit int) {
	t.Helper()
	branch, ok := specstate.ResolveDefaultBranch(context.Background(), root)
	if !ok {
		return readcensus.Accepted{}, 0
	}
	full := "refs/heads/" + branch.Ref
	if strings.HasPrefix(branch.Ref, "origin/") {
		full = "refs/remotes/" + branch.Ref
	}
	tip := strings.TrimSpace(gitOut(t, root, "rev-parse", "--verify", branch.Ref))
	commit := strings.TrimSpace(gitOut(t, root, "rev-parse", "--verify", branch.Ref+"^{commit}"))
	return readcensus.Accepted{Spellings: []string{branch.Ref, full}, IDs: []string{tip, commit}}, 1
}

// checkBudget asserts one projection's budget (Wave 6 §5.3; ledger
// SI-168, SI-356): one read session (one application projection), one
// accepted-HEAD resolution — one default-branch discovery, explicit
// resolutions of the ref it finds (one, or none when it finds none), no
// read naming the ref again — at most one enumeration of the accepted
// tree, and no write.
func checkBudget(t *testing.T, label string, b readcensus.Budget, wantExplicit int) {
	t.Helper()
	t.Logf("%s: processes %d, sessions %d, batches %d, chains %d, explicit %d, operand %d, enumerations %d (relisted %d), writes %d",
		label, b.Processes, b.Sessions, b.Batches, b.Chains, b.Explicit, len(b.Operand), len(b.Enumerations), len(b.Relisted), len(b.Writes))
	if b.Sessions != 1 {
		t.Errorf("%s opened %d read sessions, want one application projection in one session", label, b.Sessions)
	}
	if chains, explicit, operand := b.Resolutions(); chains != 1 || explicit != wantExplicit || operand != 0 {
		t.Errorf("%s resolved the accepted HEAD %d times by discovery, %d explicitly and %d as an operand, want 1, %d and 0; operand uses:\n  %s",
			label, chains, explicit, operand, wantExplicit, strings.Join(b.Operand, "\n  "))
	}
	if len(b.Enumerations) > 1 {
		t.Errorf("%s enumerated the accepted tree %d times, want at most once:\n  %s", label, len(b.Enumerations), strings.Join(b.Enumerations, "\n  "))
	}
	if len(b.Writes) != 0 {
		t.Errorf("%s ran writes:\n  %s", label, strings.Join(b.Writes, "\n  "))
	}
}

// TestPollWitness_ProjectionBudget is lane P2's structural witness (Wave 6
// §5.3; ledger SI-168, SI-356; BL-158): over one projection of each path,
// in a fresh repository (the first spec's projection runs with every cache
// empty) and then as the conditional refresh that follows, counted at
// gitx's exec seam, every projection is one read session with one
// accepted-HEAD resolution, at most one accepted-tree enumeration and no
// write. The bytes those projections derive are TestPollWitness's.
func TestPollWitness_ProjectionBudget(t *testing.T) {
	for _, fx := range witnessFixtures() {
		for _, path := range budgetPaths() {
			t.Run(fx.name+"/"+path.name, func(t *testing.T) {
				repo := fx.build(t)
				acc, explicit := acceptedOf(t, repo.Dir)
				project := path.open(repo.Dir)
				for _, spec := range fx.specs {
					first := &readcensus.Census{}
					etag := project(t, gitx.WithObserver(context.Background(), first), spec, "")
					checkBudget(t, spec+" first projection", first.Budget(acc), explicit)
					refresh := &readcensus.Census{}
					project(t, gitx.WithObserver(context.Background(), refresh), spec, etag)
					checkBudget(t, spec+" refresh", refresh.Budget(acc), explicit)
				}
			})
		}
	}
}

// TestPollWitness_EachRefreshResolvesItsOwnHead (ledger SI-356;
// readiness-recovery-v2 co-2): the accepted HEAD a refresh reads at is
// resolved by that refresh. After origin/main advances between two polls,
// the second — served by the same handler, or the same loader — resolves
// once more and reads at the new commit, never at an id a previous request
// resolved.
func TestPollWitness_EachRefreshResolvesItsOwnHead(t *testing.T) {
	for _, path := range budgetPaths() {
		t.Run(path.name, func(t *testing.T) {
			repo := buildRealShapedStore(t)
			before, _ := acceptedOf(t, repo.Dir)
			project := path.open(repo.Dir)
			first := &readcensus.Census{}
			project(t, gitx.WithObserver(context.Background(), first), realShapedStory, "")
			checkBudget(t, "the first poll", first.Budget(before), 1)

			tree := strings.TrimSpace(gitOut(t, repo.Dir, "rev-parse", "refs/remotes/origin/main^{tree}"))
			cmd := []string{"commit-tree", tree, "-p", "refs/remotes/origin/main", "-m", "origin/main advances"}
			next := strings.TrimSpace(gitOutEnv(t, repo.Dir, commitEnv(), cmd...))
			git(t, repo.Dir, nil, "update-ref", "refs/remotes/origin/main", next)
			after, _ := acceptedOf(t, repo.Dir)

			second := &readcensus.Census{}
			project(t, gitx.WithObserver(context.Background(), second), realShapedStory, "")
			checkBudget(t, "the poll after origin/main moved", second.Budget(after), 1)
			if !namesID(second.Events(), next) {
				t.Fatalf("the poll after origin/main moved to %s never read at it: it reused a resolution another request made", next)
			}
		})
	}
}

// namesID reports whether any event names id: in an argv, or as an
// object name sent to a batch process.
func namesID(events []readcensus.Event, id string) bool {
	for _, e := range events {
		for _, a := range e.Args {
			if strings.Contains(a, id) {
				return true
			}
		}
	}
	return false
}

// gitOutEnv is gitOut with env added to the process environment.
func gitOutEnv(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

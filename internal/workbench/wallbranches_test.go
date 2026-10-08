package workbench

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

// The branch menu's list (spec/wall-strip-and-drawer-v2 ac-4; ledger
// SI-368 (7)): the body-level #branch-menu popup fills from the
// snapshot's git.branches, which the wall's one git-state read already
// resolves on every refresh — no git read of the story's own (dc-3).

// localBranchReads counts the recorded git calls that list the local
// branches (gitx.LocalBranches: for-each-ref over refs/heads).
func localBranchReads(r *wallGitRecorder) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, args := range r.calls {
		if slices.Contains(args, "for-each-ref") && slices.Contains(args, "refs/heads") {
			n++
		}
	}
	return n
}

// fetchBoardSnapshot GETs /board/spec/{name}/snapshot under ctx and
// strict-decodes it.
func fetchBoardSnapshot(t *testing.T, ctx context.Context, h http.Handler, name string) asdSnapshot {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/board/spec/"+name+"/snapshot", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
	}
	var snap asdSnapshot
	if err := artifact.DecodeStrictJSON(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("strict-decoding the snapshot: %v", err)
	}
	return snap
}

// TestWallSnapshot_CarriesTheBranchList: the authoring wall's snapshot
// carries every local branch, read once per refresh by the wall's own git
// state; a branch cut between two refreshes is on the next one, whose
// revision moves, so a conditional poll cannot hide it.
func TestWallSnapshot_CarriesTheBranchList(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)
	want := strings.Fields(gitOut(t, root, "for-each-ref", "--format=%(refname:short)", "refs/heads"))

	rec := &wallGitRecorder{}
	before := fetchBoardSnapshot(t, gitx.WithObserver(t.Context(), rec), h, boardFixtureName)
	if !strings.Contains(before.HTML, `data-board-mode="authoring"`) {
		t.Fatal("the fixture is not an authoring wall")
	}
	if before.Git == nil || !reflect.DeepEqual(before.Git.Branches, want) {
		t.Fatalf("snapshot branches = %v, want every local branch %v", before.Git, want)
	}
	if n := localBranchReads(rec); n != 1 {
		t.Errorf("one snapshot listed the local branches %d times, want exactly once (the wall's own git state, no read of the menu's own)", n)
	}

	gitOut(t, root, "branch", "design/cut-between-polls")
	after := fetchBoardSnapshot(t, t.Context(), h, boardFixtureName)
	if !slices.Contains(after.Git.Branches, "design/cut-between-polls") {
		t.Errorf("the next refresh's branches = %v, want the branch cut since", after.Git.Branches)
	}
	if after.Revision == before.Revision {
		t.Error("a new branch leaves the revision unchanged: a conditional poll would answer 304 and the menu would miss it")
	}
}

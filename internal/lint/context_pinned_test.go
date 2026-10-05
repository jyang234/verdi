package lint

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/gitx/readcensus"
	"github.com/jyang234/verdi/internal/specstate"
)

// TestBuildContext_PinnedRequestDiffsAtThePinnedCommit (ledger SI-356): in
// a request that pinned its accepted HEAD, BuildContext states the same
// Context — the default branch's name, the diff base — and reads the diff
// base at the pinned commit, never by the ref; unpinned, it reads by the
// ref as before. An unresolvable default branch leaves both unknown.
func TestBuildContext_PinnedRequestDiffsAtThePinnedCommit(t *testing.T) {
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "GITHUB_BASE_REF"} {
		t.Setenv(key, "")
	}
	repo := buildBranchRepo(t)
	fabricateRemoteRef(t, repo.Dir, "main", repo.Head)
	setSymbolicRef(t, repo.Dir, "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	acc := readcensus.Accepted{Spellings: []string{"origin/main", "refs/remotes/origin/main"}, IDs: []string{repo.Head}}

	unpinnedCensus := &readcensus.Census{}
	want := BuildContext(gitx.WithObserver(context.Background(), unpinnedCensus), repo.Dir)
	if want.DefaultBranch != "main" || want.DiffBase != repo.Head {
		t.Fatalf("unpinned Context = %+v, want main with diff base %s", want, repo.Head)
	}
	if len(unpinnedCensus.Budget(acc).Operand) != 1 {
		t.Fatalf("unpinned BuildContext should read the diff base by the ref: %+v", unpinnedCensus.Events())
	}

	pinned := specstate.WithAcceptedHead(context.Background(), repo.Dir)
	census := &readcensus.Census{}
	if got := BuildContext(gitx.WithObserver(pinned, census), repo.Dir); got != want {
		t.Fatalf("pinned Context = %+v, unpinned %+v", got, want)
	}
	if b := census.Budget(acc); b.Chains != 0 || b.Explicit != 0 || len(b.Operand) != 0 {
		t.Fatalf("the pinned BuildContext resolved the ref again: %+v", b)
	}

	none := buildBranchRepo(t)
	if got := BuildContext(specstate.WithAcceptedHead(context.Background(), none.Dir), none.Dir); got.DefaultBranch != "" || got.DiffBase != "" {
		t.Fatalf("an unresolvable default branch's Context = %+v, want both unknown", got)
	}
}

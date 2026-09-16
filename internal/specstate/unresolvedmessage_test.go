package specstate

import (
	"context"
	"strings"
	"testing"
)

// TestUnresolvedDefaultBranchMessage_Resolved proves the "" early return:
// a repo whose default branch resolves (CI_DEFAULT_BRANCH set) carries no
// message — the caller's own Unproven verdict, if any, has some other
// cause.
func TestUnresolvedDefaultBranchMessage_Resolved(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildBranchRepo(t)
	if got := UnresolvedDefaultBranchMessage(context.Background(), repo.Dir); got != "" {
		t.Fatalf("UnresolvedDefaultBranchMessage(resolved) = %q, want empty", got)
	}
}

// TestUnresolvedDefaultBranchMessage_Unresolved proves the diagnostic names
// every source the resolution chain tries plus the remedy — moved
// verbatim from cmd/verdi/buildstart.go's own former private copy (dc-7,
// spec/uat-round-1, I-130), now the one shared wording every consumer
// (build start, design start, --from-stub) reuses.
func TestUnresolvedDefaultBranchMessage_Unresolved(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	repo := buildBranchRepo(t)
	got := UnresolvedDefaultBranchMessage(context.Background(), repo.Dir)
	for _, want := range []string{"CI_DEFAULT_BRANCH", "git remote HEAD", "origin/main", "origin/master", "git remote set-head origin"} {
		if !strings.Contains(got, want) {
			t.Fatalf("UnresolvedDefaultBranchMessage(unresolved) = %q, want it to name %q", got, want)
		}
	}
}

package branchbase

import (
	"context"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
)

// Collision returns the ref under which branch already exists in the tree
// a fresh ritual branch is cut from — the local branch, or a
// remote-tracking branch of the remote base resolves from (origin, for an
// origin/<default> base) — or "" when it exists under neither (ledger
// SI-333). A base that is a local branch or the disclosed HEAD fallback
// resolves from no remote, so only the local branch is asked.
//
// It is the one collision rule build start and constitution propose both
// refuse on before their cut (UAT-031; ledger SI-333, SI-367 (3)), so a new
// branch is never cut beside an existing one the next push would turn into
// a divergence.
func Collision(ctx context.Context, root, branch string, base Resolution) (string, error) {
	local, err := gitx.HasLocalBranch(ctx, root, branch)
	if err != nil {
		return "", err
	}
	if local {
		return "refs/heads/" + branch, nil
	}
	remote, ok := strings.CutSuffix(base.Ref, "/"+base.BranchName)
	if base.Kind != ResolvedDefault || base.BranchName == "" || !ok || remote == "" {
		return "", nil
	}
	tracking, err := gitx.HasRemoteTrackingBranch(ctx, root, remote, branch)
	if err != nil {
		return "", err
	}
	if tracking {
		return "refs/remotes/" + remote + "/" + branch, nil
	}
	return "", nil
}

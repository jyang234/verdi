package gitx

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// IsAncestor reports whether ancestor is commit itself or a real ancestor
// of commit in dir's git history (`git merge-base --is-ancestor`) — the
// primitive 03 §The fold's "current" record selection needs: "the latest
// record per (kind, producer) whose commit is an ancestor of C". A commit
// is its own ancestor for this purpose, matching git's own semantics and
// the fold's evident intent (a record produced at the exact commit being
// evaluated must be in scope).
//
// `git merge-base --is-ancestor` exits 0 when true, 1 when false (a real,
// resolvable commit that just isn't an ancestor), and any other non-zero
// code (typically 128) when either commit does not resolve — that last
// case is a real error, not a false answer, so callers can tell "no" from
// "I can't tell".
func IsAncestor(ctx context.Context, dir, ancestor, commit string) (bool, error) {
	if _, err := run(ctx, dir, "merge-base", "--is-ancestor", ancestor, commit); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("gitx: IsAncestor(%q, %q): %w", ancestor, commit, err)
	}
	return true, nil
}

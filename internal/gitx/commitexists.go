package gitx

import (
	"context"
	"errors"
	"os/exec"
)

// CommitExists reports whether commit names a real commit object reachable
// in dir's git history — VL-009's "frozen artifacts carry a valid frozen
// stamp" (the shape check lives in internal/artifact; realness against
// actual history is git's job) and VL-003's "pins name real commits". It
// deliberately checks the "^{commit}" peeled form so a sha that happens to
// name a blob or tree, rather than a commit, correctly reports false rather
// than a false positive. A non-existent commit is not an error — it is the
// expected, common false case — but a dir that is not a git repository at
// all still surfaces as an error, so callers can tell "not a repo" from
// "not found".
func CommitExists(ctx context.Context, dir, commit string) (bool, error) {
	if _, err := run(ctx, dir, "rev-parse", "--verify", "-q", commit+"^{commit}"); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// A non-zero exit from a real git invocation: distinguish "not a
			// repo at all" (still an error) from "repo exists, commit does
			// not" (false, not an error).
			if _, repoErr := run(ctx, dir, "rev-parse", "--git-dir"); repoErr == nil {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

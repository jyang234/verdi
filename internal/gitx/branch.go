package gitx

import (
	"context"
	"fmt"
	"strings"
)

// CurrentBranch returns the short name of the branch HEAD currently points
// at (`git symbolic-ref --short HEAD`), run inside dir. It fails in a
// detached-HEAD checkout (the common case in CI, which checks out a bare
// commit) — callers that need a ref name in CI should prefer a
// forge-provided environment variable (e.g. GitLab's CI_COMMIT_REF_NAME)
// and fall back to CurrentBranch only for local, non-CI use (added
// alongside RevParse/HashObject/LsFiles/Show for phase 5's `sync`, which
// needs the current ref to compute its store-layout slug — 01 §notes).
func CurrentBranch(ctx context.Context, dir string) (string, error) {
	out, err := run(ctx, dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("gitx: CurrentBranch: %w (detached HEAD? prefer a CI-provided ref env var)", err)
	}
	return strings.TrimSpace(string(out)), nil
}

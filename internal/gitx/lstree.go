package gitx

import (
	"context"
	"fmt"
	"strings"
)

// LsTree recursively lists path's tracked files as they existed at ref —
// `git ls-tree -r --name-only <ref> -- <path>` — returning repo-relative
// slash paths in git's own deterministic tree order. Needed by
// spec/ref-index's default-branch walk (dc-4), which must enumerate
// `.verdi/specs/active/` and `.verdi/specs/archive/` at the resolved
// default-branch ref rather than the working tree (co-1: "index
// computation reads refs and never switches a checkout").
//
// A path that does not exist at ref is not an error: `git ls-tree` simply
// returns no entries (verified against real git — see lstree_test.go),
// which is exactly the distinction spec/ref-index ac-4 needs between "this
// ref has no spec.md yet" (empty result, no error — a disclosed entry, not
// a dropped one) and "this ref does not resolve at all" (a real error,
// below). ref itself failing to resolve — a bogus ref name, or one deleted
// out from under a caller — IS an error, never a silently empty result.
func LsTree(ctx context.Context, dir, ref, path string) ([]string, error) {
	out, err := run(ctx, dir, "ls-tree", "-r", "--name-only", ref, "--", path)
	if err != nil {
		return nil, fmt.Errorf("gitx: LsTree(%s:%s): %w", ref, path, err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

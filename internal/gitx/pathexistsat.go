package gitx

import (
	"context"
	"fmt"
	"strings"
)

// PathExistsAt reports whether path is present in commit's tree —
// `git ls-tree --name-only <commit> -- <path>`, whose output is the path
// itself when a tracked file exists there at commit and empty when it does
// not. path is repo-relative (forward slashes).
//
// The point of this predicate over a bare gitx.Show is the clean three-way it
// draws that `git show <commit>:<path>` cannot. `git show` collapses "path
// genuinely absent at a resolvable commit" and "git could not answer at all"
// (an unresolvable commit, a broken repo, no git binary) into one
// indistinguishable non-zero exit — so a caller that treats every Show error
// as "absent" silently guesses "not present" on an operational failure.
// `git ls-tree` instead treats an absent path at a RESOLVABLE commit as empty
// output / exit 0 — never an error (the same distinction LsTree already
// relies on) — and reserves a non-zero exit for the commit itself failing to
// resolve or an operational git failure. A caller deciding a
// frozen-immutability question can therefore proceed on a proven absence yet
// refuse on an operational failure, never guessing about presence on an
// error.
func PathExistsAt(ctx context.Context, dir, commit, path string) (bool, error) {
	out, err := run(ctx, dir, "ls-tree", "--name-only", commit, "--", path)
	if err != nil {
		return false, fmt.Errorf("gitx: PathExistsAt(%s:%s): %w", commit, path, err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

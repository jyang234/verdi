package gitx

import (
	"context"
	"fmt"
	"strings"
)

// CommonDir resolves `git rev-parse --git-common-dir`, run with dir as
// git's working directory: the repository's SHARED administrative
// directory ($GIT_COMMON_DIR) — the same directory for the main worktree
// and every worktree linked against it, and the directory under which
// `worktrees/<id>/` administrative entries live (spec/execution-workspace
// §Workspace naming's registry-reconciliation algorithm, step 1).
// Read-only: this performs no mutation of any kind, exactly one `git
// rev-parse` call.
//
// Git prints a path RELATIVE to dir for the ordinary case (most often
// ".git", when dir names the main worktree) and an absolute path when dir
// is itself a linked worktree. CommonDir returns exactly what git prints,
// trimmed of trailing whitespace; a caller that needs an absolute path is
// responsible for resolving a relative result against dir itself (git's own
// documented behavior for `--git-common-dir`: "usually the .git directory
// of the current repository, but if the repository is a linked working
// tree, is the .git directory of the repository the linked working tree is
// attached to").
func CommonDir(ctx context.Context, dir string) (string, error) {
	out, err := run(ctx, dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("gitx: CommonDir: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

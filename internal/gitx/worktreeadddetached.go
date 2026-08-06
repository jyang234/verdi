package gitx

import (
	"context"
	"fmt"
)

// WorktreeAddDetached cuts a new git worktree at path, DETACHED at the
// exact commit sha — `git worktree add --detach <path> <sha>` against dir
// (spec/execution-workspace §Exact workspace materialization, shape (a):
// "an exact commit SHA materialized as a DETACHED worktree — a new gitx
// wrapper over `git worktree add --detach <path> <sha>`; no such primitive
// exists today").
//
// --detach is ALWAYS passed, so this call never mints a local branch (the
// spec's "materialization never mints a local branch" invariant,
// preserving worktree-manager ac-2's hard-won gate against git's own
// worktree-add DWIM branch-minting). dir's own branch, index, and working
// tree are untouched by this call, exactly like WorktreeAdd.
func WorktreeAddDetached(ctx context.Context, dir, path, sha string) error {
	if _, err := run(ctx, dir, "worktree", "add", "--detach", path, sha); err != nil {
		return fmt.Errorf("gitx: WorktreeAddDetached(%q, %q): %w", path, sha, err)
	}
	return nil
}

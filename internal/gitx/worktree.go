package gitx

// Working-tree state queries and rituals for the board-owned git
// affordance (05 §Workbench "Authoring": a commit/push button, a
// persistent uncommitted-changes indicator, and a branch-switch guard —
// a PM or designer must be able to author and durably save without git
// fluency).

import (
	"context"
	"fmt"
	"strings"
)

// StatusDirty reports whether dir's working tree has any uncommitted
// change (staged, unstaged, or untracked-and-unignored) — the
// uncommitted-changes indicator's single source of truth.
func StatusDirty(ctx context.Context, dir string) (bool, error) {
	out, err := run(ctx, dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// LocalBranches lists dir's local branch short names, sorted by git's
// default refname order (deterministic).
func LocalBranches(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// Checkout switches dir to an existing branch. It refuses a dirty working
// tree outright — the server-side half of the branch-switch guard ("an
// hour of board work evaporating in someone else's working tree is
// exactly the silent loss this system exists to forbid") — before git
// even gets a chance to merge working-tree changes across branches.
func Checkout(ctx context.Context, dir, branch string) error {
	dirty, err := StatusDirty(ctx, dir)
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("gitx: Checkout(%q): working tree has uncommitted changes; commit them first (branch-switch guard, 05 §Workbench)", branch)
	}
	if _, err := run(ctx, dir, "checkout", branch); err != nil {
		return fmt.Errorf("gitx: Checkout(%q): %w", branch, err)
	}
	return nil
}

// Push pushes dir's current branch to origin (setting upstream on first
// push). The board's commit affordance calls this right after
// CreateCommit — "commits + pushes the working tree on the design
// branch".
func Push(ctx context.Context, dir string) error {
	if _, err := run(ctx, dir, "push", "--set-upstream", "origin", "HEAD"); err != nil {
		return fmt.Errorf("gitx: Push: %w", err)
	}
	return nil
}

// HasRemote reports whether dir has a remote named name configured — the
// commit affordance pushes only when an origin exists (a purely local
// checkout can still commit durably).
func HasRemote(ctx context.Context, dir, name string) (bool, error) {
	out, err := run(ctx, dir, "remote")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == name {
			return true, nil
		}
	}
	return false, nil
}

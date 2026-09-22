package gitx

// Working-tree state queries and rituals for the board-owned git
// affordance (05 §Workbench "Authoring": a commit/push button, a
// persistent uncommitted-changes indicator, and a branch-switch guard —
// a PM or designer must be able to author and durably save without git
// fluency).

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrBranchCheckedOut is WorktreeAdd's typed refusal when branch is
// already checked out in dir itself (the serving checkout). It is
// detected PROACTIVELY — by asking git for dir's current branch before
// `git worktree add` ever runs — so the refusal never depends on parsing
// git's version-dependent "already checked out" stderr text (the D6-8
// environment-parity failure class: local git and a CI runner's git word
// the same fatal differently, and a string match that passes on one
// silently misclassifies on the other).
var ErrBranchCheckedOut = errors.New("gitx: branch is already checked out in this checkout")

// StatusDirty reports whether dir's working tree has any uncommitted
// change (staged, unstaged, or untracked-and-unignored) — the
// uncommitted-changes indicator's single source of truth, and the single
// dirty check spec/verdi-store-layout §gc-reclaim names before any
// destructive operation.
//
// The query passes --untracked-files=all explicitly (R-RR3-28) so the
// answer is independent of the repository's own display configuration: a
// plain `git status --porcelain` honors status.showUntrackedFiles, so an
// operator who set it to "no" — an ordinary display preference, not a
// safety decision — would otherwise be told a tree holding their own
// untracked, unignored work is clean. Ignored files stay out of scope
// (--untracked-files=all still respects .gitignore), which is exactly
// what "untracked-and-unignored" above promises.
func StatusDirty(ctx context.Context, dir string) (bool, error) {
	out, err := run(ctx, dir, "status", "--porcelain", "--untracked-files=all")
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

// WorktreeAdd cuts a new git worktree at path, checked out to branch —
// `git worktree add <path> <branch>` against dir (spec/worktree-manager
// dc-1's single git-worktree-mutating command class internal/wtmanager's
// EnsureWorktree ever runs against the serving checkout's own root; dir's
// own branch/index/working tree are untouched by this call, the same way
// `checkout`/`switch` never appear here).
func WorktreeAdd(ctx context.Context, dir, path, branch string) error {
	// Proactive checked-out-here guard: ask git directly for dir's own
	// current branch before mutating anything. A worktree cannot be cut
	// for a branch that dir already has checked out, and detecting that
	// here — rather than from git's post-hoc failure text — keeps the
	// refusal robust across git versions (see ErrBranchCheckedOut).
	current, err := CurrentBranch(ctx, dir)
	if err != nil {
		return fmt.Errorf("gitx: WorktreeAdd(%q, %q): checking current branch: %w", path, branch, err)
	}
	if current == branch {
		return fmt.Errorf("gitx: WorktreeAdd(%q, %q): %w", path, branch, ErrBranchCheckedOut)
	}

	if _, err := run(ctx, dir, "worktree", "add", path, branch); err != nil {
		// Defensive fallback only: if git refuses because branch is
		// checked out in some worktree the proactive check above did not
		// cover, still surface the typed refusal rather than raw stderr.
		if strings.Contains(err.Error(), "already checked out") {
			return fmt.Errorf("gitx: WorktreeAdd(%q, %q): %w", path, branch, ErrBranchCheckedOut)
		}
		return fmt.Errorf("gitx: WorktreeAdd(%q, %q): %w", path, branch, err)
	}
	return nil
}

// WorktreeRemove removes the linked worktree at path — `git -c
// status.showUntrackedFiles=all worktree remove <path>` against dir,
// deliberately WITHOUT --force (spec/worktree-manager dc-4: git's own
// dirty-tree refusal is a second, redundant guard behind the caller's
// own gitx.StatusDirty check, never the only one relied on; a worktree
// git itself refuses to remove is surfaced as an ordinary error, never
// silently forced through).
//
// That second guard is now INDEPENDENT of the repository's
// configuration, not merely redundant (R-RR3-29). Git answers `worktree
// remove`'s own cleanliness question through its own `git status`, which
// honors status.showUntrackedFiles exactly as StatusDirty's plain query
// did — so under `status.showUntrackedFiles=no` both guards failed
// together and git deleted a worktree holding the operator's untracked
// work. The global -c option, passed BEFORE the subcommand, pins that
// query for this invocation alone. -c is git's ordinary configuration
// override, not a force flag: the argv still carries none of SI-224's
// forbidden tokens, and a dirty worktree is still kept, never forced.
func WorktreeRemove(ctx context.Context, dir, path string) error {
	if _, err := run(ctx, dir, "-c", "status.showUntrackedFiles=all", "worktree", "remove", path); err != nil {
		return fmt.Errorf("gitx: WorktreeRemove(%q): %w", path, err)
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

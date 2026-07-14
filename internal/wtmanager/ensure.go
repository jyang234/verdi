package wtmanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/gitx"
)

// worktreeAdd is gitx.WorktreeAdd behind a package-level var so a test can
// wrap it to COUNT invocations (ac-3's "exactly one git worktree add"
// proof) without needing a second, parallel implementation.
var worktreeAdd = gitx.WorktreeAdd

// lockPollInterval is how often a caller that lost the lock-acquisition
// race polls for either the winner's completed cut (the path now exists)
// or the lock becoming free. dc-2 bounds every lock hold to a single
// short git invocation, so this is a narrow, fast poll — never a
// multi-second backoff.
const lockPollInterval = 10 * time.Millisecond

// EnsureWorktree lazily cuts a managed git worktree for LOCAL design
// branch branch under root's data zone on first call, and reuses it
// unchanged on every later call — dc-1's lazy, synchronous, idempotent
// contract. It runs exactly one git-worktree-mutating command — `git
// worktree add` — and only when no worktree is already cut for branch;
// the serving checkout's own branch, index, and working tree are never
// touched (no checkout/switch on root).
//
// It refuses with ErrNotLocalBranch when branch has no local ref
// (ac-2), and with ErrCheckedOutHere when branch is already checked out
// at root itself (ac-2). A per-worktree lockfile (internal/filelock,
// dc-2) makes exactly one caller — across goroutines or processes — the
// one that actually runs `git worktree add`; every other concurrent
// caller for the same not-yet-cut branch waits for the winner and
// returns its path (ac-3).
func EnsureWorktree(ctx context.Context, root, branch string) (string, error) {
	path := worktreePath(root, branch)
	lp := lockPath(root, branch)

	if pathExists(path) {
		return path, nil
	}

	if err := os.MkdirAll(worktreesRoot(root), 0o755); err != nil {
		return "", fmt.Errorf("wtmanager: EnsureWorktree(%q): %w", branch, err)
	}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		f, err := filelock.Acquire(lp)
		if err != nil {
			var held *filelock.ErrHeld
			if !errors.As(err, &held) {
				return "", fmt.Errorf("wtmanager: EnsureWorktree(%q): acquiring lock: %w", branch, err)
			}
			// Lost the race: someone else is cutting (or has just cut)
			// this worktree. Reuse if it's already there; otherwise wait
			// briefly and check again — never attempt a competing add.
			if pathExists(path) {
				return path, nil
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(lockPollInterval):
			}
			continue
		}

		// Won the lock. Re-check reuse under it — a winner that finished
		// between our pathExists check above and acquiring the lock.
		if pathExists(path) {
			_ = filelock.Release(f, lp)
			return path, nil
		}

		local, lerr := gitx.HasLocalBranch(ctx, root, branch)
		if lerr != nil {
			_ = filelock.Release(f, lp)
			return "", fmt.Errorf("wtmanager: EnsureWorktree(%q): checking local branch: %w", branch, lerr)
		}
		if !local {
			_ = filelock.Release(f, lp)
			return "", fmt.Errorf("wtmanager: EnsureWorktree(%q): %w", branch, ErrNotLocalBranch)
		}

		addErr := worktreeAdd(ctx, root, path, branch)
		_ = filelock.Release(f, lp)
		if addErr != nil {
			if isAlreadyCheckedOut(addErr) {
				return "", fmt.Errorf("wtmanager: EnsureWorktree(%q): %w", branch, ErrCheckedOutHere)
			}
			return "", fmt.Errorf("wtmanager: EnsureWorktree(%q): cutting worktree: %w", branch, addErr)
		}
		return path, nil
	}
}

// pathExists reports whether path exists on disk at all (file or
// directory) — the reuse fast-path's only question; EnsureWorktree never
// re-validates worktree internals once the deterministic path is there.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isAlreadyCheckedOut reports whether err is git's own "worktree add"
// refusal for a branch already checked out somewhere (root itself, in
// EnsureWorktree's calling convention, since a managed worktree is never
// cut twice for the same branch — reuse always wins first). Matched by
// git's stable stderr phrasing ("is already checked out at") rather than
// an exit code, since worktree add's failure modes share exit code 128.
func isAlreadyCheckedOut(err error) bool {
	return strings.Contains(err.Error(), "already checked out")
}

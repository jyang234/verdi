package wtmanager

import "errors"

// ErrNotLocalBranch is EnsureWorktree's typed refusal (ac-2) when branch
// has no LOCAL refs/heads/<branch> ref — a remote-tracking-only branch,
// or one that resolves nowhere at all. Matches feature dc-5's
// local-branches-only rule verbatim: EnsureWorktree never mints a local
// branch from a remote-tracking ref to route around this.
var ErrNotLocalBranch = errors.New("wtmanager: branch has no local ref (remote-tracking-only or absent)")

// ErrCheckedOutHere is EnsureWorktree's typed refusal (ac-2) when git
// reports branch in use by a worktree — checked out in the serving
// checkout (root) itself, or in use by another worktree, including one
// mid-rebase or mid-bisect on it (ledger SI-354 (1)) — surfaced as a
// named, human-readable error rather than raw git stderr. Despite its name
// it does not say which: a consumer that serves root's own branch re-checks
// gitx.CurrentBranch(root) before treating root as the holder.
var ErrCheckedOutHere = errors.New("wtmanager: branch is already checked out in the serving checkout")

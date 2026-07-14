package wtmanager

import "errors"

// ErrNotLocalBranch is EnsureWorktree's typed refusal (ac-2) when branch
// has no LOCAL refs/heads/<branch> ref — a remote-tracking-only branch,
// or one that resolves nowhere at all. Matches feature dc-5's
// local-branches-only rule verbatim: EnsureWorktree never mints a local
// branch from a remote-tracking ref to route around this.
var ErrNotLocalBranch = errors.New("wtmanager: branch has no local ref (remote-tracking-only or absent)")

// ErrCheckedOutHere is EnsureWorktree's typed refusal (ac-2) when branch
// is already checked out in the serving checkout (root) itself — git's
// own "already checked out" worktree-add refusal, surfaced as a named,
// human-readable error rather than raw git stderr.
var ErrCheckedOutHere = errors.New("wtmanager: branch is already checked out in the serving checkout")

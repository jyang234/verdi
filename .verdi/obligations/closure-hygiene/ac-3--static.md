---
id: obligation/closure-hygiene--ac-3--static
kind: obligation
title: "The survey performs zero git-mutating calls, built on the new gitx.WorktreeList and the existing IsAncestor/StatusDirty primitives"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/closure-hygiene" }
frozen: { at: 2026-07-20, commit: f8c298d3ad712ead9c108d707a10c49547a440ce }
---
# The survey performs zero git-mutating calls, built on the new gitx.WorktreeList and the existing IsAncestor/StatusDirty primitives

The static evidence must show the merged-branch survey (a) calls only
`gitx.LocalBranches` and `gitx.IsAncestor` against the default branch tip —
read-only — and the worktree survey (b) is built entirely on the new
`gitx.WorktreeList(ctx, dir)` primitive (dc-4: `git worktree list
--porcelain`, parsed; no such primitive exists before this story, verified
by grep against the pre-story `internal/gitx` surface),
`internal/wtmanager.WorktreesRoot` (dc-4's export of the previously
unexported `worktreesRoot`, reused rather than a second hardcoded
`.verdi/data/worktrees/` literal) for managed/unmanaged classification,
and `gitx.StatusDirty` for the clean/dirty signal — no new git-mutating
call anywhere in the new code path.

It must further show the primary checkout is excluded by `git worktree
list --porcelain`'s own first-entry-is-primary ordering, cross-checked by
that entry's `.git` being a directory rather than a linked-worktree `.git`
file (dc-4's two independent signals, not one assumed convention), and
that a detached-HEAD worktree's merge state is resolved at the commit
level (`gitx.IsAncestor` against the raw commit) rather than a guessed
branch-level property, with no branch name asserted where none exists.

An exhaustive command-surface check (an inventory of every exec call the
new code path makes) proves the list is `worktree list`,
`rev-parse`/`merge-base`, and status checks only — never `add`, `remove`,
or `prune`.

---
id: obligation/worktree-manager--ac-2--static
kind: obligation
title: "A local-ref existence check gates every git worktree add call; refusals are typed, never a raw git error"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/worktree-manager" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# A local-ref existence check gates every git worktree add call; refusals are typed, never a raw git error

The static evidence must show `EnsureWorktree` checks for a LOCAL `refs/heads/<branch>` ref (never a remote-tracking one) before ever calling `git worktree add`, returning a named, exported error value (e.g. `ErrNotLocal` or equivalent) when absent — never silently creating a local branch from a remote-tracking ref. It must also show the "already checked out elsewhere" failure path is caught and re-wrapped as a named, human-readable error rather than surfaced as raw git stderr.

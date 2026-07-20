---
id: obligation/gc-reclaim--ac-1--static
kind: obligation
title: "The eligibility predicate is a pure, ordered function over *residue.Result and the invoking checkout's identity, with a closed kept-reason enum and no seventh path"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/gc-reclaim" }
frozen: { at: 2026-07-20, commit: 1dcb8f67e8d20b5930de9aa7966bac935d16d3b8 }
---
# The eligibility predicate is a pure, ordered function over *residue.Result and the invoking checkout's identity, with a closed kept-reason enum and no seventh path

The static evidence must show the predicate takes only `*residue.Result`
(never re-deriving any fact `internal/residue.Scan` already computed — no
`gitx.IsAncestor`, no `gitx.StatusDirty` call anywhere in the new package)
plus the invoking checkout's own root and current branch, and runs the
same total, ORDERED switch over both row shapes dc-2 defines: worktree
rows (`Result.Worktrees`) checked in the fixed sequence
unresolved-state → unmerged → dirty → detached → managed → invoking, and
branch-only rows (`Result.MergedBranches` entries with no matching
`Worktrees[].Branch`) checked against invoking alone — mirroring
`internal/wtmanager.decideReclaim`'s own ordered-switch precedent, so a row
with multiple simultaneously-true exclusion facts still yields exactly one
deterministic reason, never an arbitrary or combinatorial one.

It must also show the kept-reason type is a closed enum (unmerged, dirty,
unresolved-state, detached, managed, invoking) with a compile-time
exhaustiveness check over its own `String`/rendering switch, so a future
case added to the type without a matching switch arm fails the build
rather than silently rendering an empty or generic label.

It must further show the "not primary checkout" condition is NOT
independently coded for either row shape (ledger R4-I-80): the worktree
shape relies, documented and cited, on `internal/residue.Scan`'s own
contract that `Result.Worktrees` never contains the primary checkout,
never re-verified here; the branch-only shape carries no primary-specific
check at all, since `Result` exposes no fact identifying the primary
checkout's own branch — that gap is covered by ac-2's execution-time
second guard, not by this predicate.

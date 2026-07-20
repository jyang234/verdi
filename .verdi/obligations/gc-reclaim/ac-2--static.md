---
id: obligation/gc-reclaim--ac-2--static
kind: obligation
title: "Execution ordering (worktree then branch, via the existing WorktreeRemove and the new DeleteMergedBranch) and the 0/2 exit-code map are fixed, inspectable sequences with no conditional reordering"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/gc-reclaim" }
frozen: { at: 2026-07-20, commit: 1dcb8f67e8d20b5930de9aa7966bac935d16d3b8 }
---
# Execution ordering (worktree then branch, via the existing WorktreeRemove and the new DeleteMergedBranch) and the 0/2 exit-code map are fixed, inspectable sequences with no conditional reordering

The static evidence must show that, per eligible item, `gitx.WorktreeRemove`
(the existing primitive, called WITHOUT `--force`) always runs before the
new `gitx.DeleteMergedBranch` (dc-3: `git branch -d`, never `-D`, composed
atop the existing `gitx.RevParse` and `gitx.DeleteBranch` rather than a
second, independent `-d` call — ledger R4-I-81), and that a branch-only row
skips the worktree step entirely rather than calling it on an empty path.

It must show `DeleteMergedBranch`'s own signature returns the branch's
pre-delete tip commit alongside its ordinary success/refusal outcome, and
that this value, not a separate lookup, is what AC-2's tip-SHA disclosure
prints.

It must show `verdi gc`'s exit-code map is unconditional: 0 whenever the
run completes, regardless of how many individual items were kept or
individually refused (a refusal never propagates to the process exit
code); 2 only for a whole-run operational failure — an unresolvable
default branch (checked once, before any plan is computed, dry-run and
`--apply` alike) or a usage error — and that no code path in the new
package can return a verdict-shaped exit 1, matching `verdi gc`'s existing
0/2-only contract.

It must further show `internal/wtmanager`'s own managed-worktree GC keeps
its looser "unresolved default branch means nothing eligible" posture
unchanged (co-2) — the two postures coexist deliberately, not by oversight,
because unmanaged reclamation reaches worktrees and branches this binary
did not itself create.

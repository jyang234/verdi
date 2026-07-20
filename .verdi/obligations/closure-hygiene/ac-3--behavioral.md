---
id: obligation/closure-hygiene--ac-3--behavioral
kind: obligation
title: "A four-worktree fixture — managed, two unmanaged (merged/unmerged), and detached-HEAD — plus a mixed branch set, prove every classification and a clean exit"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/closure-hygiene" }
frozen: { at: 2026-07-20, commit: f8c298d3ad712ead9c108d707a10c49547a440ce }
---
# A four-worktree fixture — managed, two unmanaged (merged/unmerged), and detached-HEAD — plus a mixed branch set, prove every classification and a clean exit

The behavioral evidence must drive a fixturegit repository with a mix of
merged and unmerged local branches — the test asserts the merged subset is
counted and named exactly (survey (a)) — plus four real `git worktree
add`-materialized worktrees on local disk (co-1): one under the managed
root (`internal/wtmanager.WorktreesRoot`) on a design branch, one
unmanaged (outside that root) on a branch that is merged into the default
branch, one unmanaged on a branch that is NOT merged, and one unmanaged
with a detached `HEAD` checked out at a commit that IS an ancestor of the
default branch tip but carries no branch name at all.

The test asserts each of the four is named in the survey with its correct
managed/unmanaged tag, its correct merged/not-merged (or, for the detached
case, commit-level-merged) state, and its correct clean/dirty state (one
fixture worktree carries an uncommitted edit to prove the dirty signal is
live) — and that the detached-HEAD entry discloses its commit-level merge
state with no branch name asserted, never guessed as a branch-level
property it does not have.

Finally, the test asserts the overall run's exit code is 0 regardless of
any of these four worktrees' states — the survey never flags (dc-3),
proven by including at least one unmerged, dirty, unmanaged worktree in
the same run that still exits clean.

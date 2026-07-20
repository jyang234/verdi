---
id: obligation/gc-reclaim--ac-2--behavioral
kind: obligation
title: "Six fixturegit cases prove dry-run's zero mutation, --apply's ordering and tip-SHA print, both independent second-guard refusals, the partial-outcome disclosure, and the unresolvable-default-branch refusal"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/gc-reclaim" }
frozen: { at: 2026-07-20, commit: 1dcb8f67e8d20b5930de9aa7966bac935d16d3b8 }
---
# Six fixturegit cases prove dry-run's zero mutation, --apply's ordering and tip-SHA print, both independent second-guard refusals, the partial-outcome disclosure, and the unresolvable-default-branch refusal

The behavioral evidence must drive six fixturegit cases.

1. Dry-run (`--reclaim-unmanaged` alone) against a repository containing at
   least one eligible item performs zero git-mutating calls — asserted
   directly against the fixture's own git state (branch list, worktree
   registration) unchanged before and after — and still prints the item as
   eligible.
2. `--apply` on a clean eligible worktree+branch pair removes the worktree,
   then deletes the branch, in that order, and prints the branch's own
   pre-delete tip commit.
3. A worktree reported clean (`Dirty` false) at scan time but dirtied
   BEFORE `--apply` runs is kept via `git worktree remove`'s own refusal on
   the now-dirty tree — the first second-guard witness — disclosed, with
   the sweep continuing to the next item.
4. A branch-only row checked out at a worktree that is itself
   NON-invoking and primary-shaped (constructed so the row is not caught by
   the invoking check) is instead caught by `git branch -d`'s own refusal
   at apply time — the second second-guard witness, proving ledger
   R4-I-80's resolution holds in practice, not only in the predicate's own
   silence.
5. A worktree whose removal succeeds but whose paired branch delete is then
   forced to fail (e.g. a branch made to appear checked out elsewhere
   between the two calls) asserts the dedicated partial-outcome line, never
   the generic reclaimed or generic failure wording.
6. An empty default branch ref refuses the whole run — dry-run and
   `--apply` alike — printing no plan and attempting no mutating call,
   with exit 2.

Each case additionally asserts the process exit code matches ac-2's own
0/2 map (test 6 alone expects 2; the rest expect 0 regardless of any
individual item's kept-or-refused outcome).

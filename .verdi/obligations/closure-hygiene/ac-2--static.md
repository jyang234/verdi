---
id: obligation/closure-hygiene--ac-2--static
kind: obligation
title: "close/<name> branch classification is a total two-outcome function sharing pattern-a's own tip-tree check"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/closure-hygiene" }
frozen: { at: 2026-07-20, commit: f8c298d3ad712ead9c108d707a10c49547a440ce }
---
# close/<name> branch classification is a total two-outcome function sharing pattern-a's own tip-tree check

The static evidence must show the `close/<name>` classification pass
enumerates local branches matching the `close/*` prefix
(`gitx.LocalBranches`, filtered), and restricts classification to the
subset unmerged into the default branch (`gitx.IsAncestor` false) — a
merged `close/<name>` branch is excluded entirely, never classified either
way.

For each unmerged `close/<name>` branch, the pass returns exactly one of
{ritual-incomplete, superseded-elsewhere} based on a single boolean —
whether that branch's own tip tree already contains
`.verdi/specs/archive/<name>/spec.md` — with no third outcome reachable.

It must also show this is the SAME tip-tree check AC-1 pattern (a)
performs, called once and shared by both report lines, not two
independently-maintained implementations of git-plumbing reads that could
silently disagree — `internal/residue`'s single scan produces both the
AC-1 finding and the AC-2 classification from one pass over `close/*`
branches.

Only the ritual-incomplete outcome sets the run's `flagged` bool;
superseded-elsewhere is appended to output but never reaches the
exit-code path (dc-3).

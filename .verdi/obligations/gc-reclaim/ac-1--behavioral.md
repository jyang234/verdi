---
id: obligation/gc-reclaim--ac-1--behavioral
kind: obligation
title: "One fixturegit survey combining every eligible and every excluded row shape proves each is classified exactly once, never silently dropped"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/gc-reclaim" }
frozen: { at: 2026-07-20, commit: 1dcb8f67e8d20b5930de9aa7966bac935d16d3b8 }
---
# One fixturegit survey combining every eligible and every excluded row shape proves each is classified exactly once, never silently dropped

The behavioral evidence must drive a single fixturegit repository whose
`internal/residue.Scan` result, fed into the predicate, contains — in one
survey — an eligible worktree+branch pair, an eligible branch-only row
(mirroring this repository's own live `board-polish`-shaped witness: a
merged branch with no worktree of its own), an unmerged row (mirroring the
spec's own `close/attest-helper`/`close/close-preflight`/
`close/disposition-verb`/`close/home-status-glance` witnesses — merged
into nothing despite being superseded-elsewhere), a dirty row, a row with
`MergedUnresolved` or `DirtyUnresolved` true and a populated `Reason`, a
detached-HEAD row (`Branch == ""`, mirroring the live `w6-exit` witness), a
managed-worktree row, and a row whose `Path` equals the invoking
checkout's own root (mirroring this very story's own `verdi-wt/
residue-reclamation` worktree, trivially merged and clean, kept only by
the invoking exclusion).

The test asserts every single row above is named in the plan's own output
exactly once, with its correct eligible-or-kept-and-reason classification,
and that no row is silently absent from the report — an assertion over the
full row count, not merely over the presence of a few expected lines,
closing the "did the loop skip something" gap a partial assertion would
leave open.

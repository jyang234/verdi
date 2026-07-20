---
id: obligation/gc-reclaim--ac-3--behavioral
kind: obligation
title: "Built-binary tests prove both invocation shapes disclose the other slice as not-run, and a golden transcript proves every AC-1 exclusion reason renders as its own single line"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/gc-reclaim" }
frozen: { at: 2026-07-20, commit: 1dcb8f67e8d20b5930de9aa7966bac935d16d3b8 }
---
# Built-binary tests prove both invocation shapes disclose the other slice as not-run, and a golden transcript proves every AC-1 exclusion reason renders as its own single line

The behavioral evidence must drive the built `verdi` binary, not `go run`
(matching this repository's own established convention for CLI-behavioral
proof, PLAN.md phase 1), through two invocation shapes against one
fixturegit repository: a plain `verdi gc` run, whose output must still
contain its pre-existing managed-worktree behavior byte-for-byte plus the
new available-but-not-run-this-invocation disclosure naming
`--reclaim-unmanaged`; and a `verdi gc --reclaim-unmanaged` run, whose
output must contain the mirrored managed-slice-not-run disclosure
alongside the pre-existing derived-cache/layout-cache disclosure. Neither
run may print the other's own reclaim/kept lines.

It must further drive a fixturegit repository exercising every one of
ac-1's exclusion reasons at once (unmerged, dirty, unresolved-state,
detached, managed, invoking) plus at least one eligible worktree+branch
pair and one eligible branch-only row, and assert the full transcript
against a committed golden — one line per item, in dc-4's exact
templates — so a future wording change to any line is a deliberate,
reviewed golden update, never a silent drift.

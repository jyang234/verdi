---
id: obligation/ref-index--ac-1--behavioral
kind: obligation
title: "One entry per default-branch spec and per design-branch draft, no duplicates or drops"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/ref-index" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# One entry per default-branch spec and per design-branch draft, no duplicates or drops

The behavioral evidence must show a Go test over a `fixturegit`-built repository carrying a default branch with N committed specs and M `design/*` local branches (M >= 2), each with its own draft spec.md at a distinct commit, asserting `ComputeIndex` returns exactly N + M entries: one per default-branch spec (by ref name) and one per design branch (by branch name), with no ref appearing twice in the output and no ref from either set missing. The test must also assert a second `ComputeIndex` call against the same unmodified fixture returns byte-identical output (determinism, not just count-correctness).

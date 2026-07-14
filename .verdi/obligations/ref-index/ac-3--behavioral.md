---
id: obligation/ref-index--ac-3--behavioral
kind: obligation
title: "A mixed fixture lands every entry in its ratified status group, deterministically"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/ref-index" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# A mixed fixture lands every entry in its ratified status group, deterministically

The behavioral evidence must show a Go test over a fixture repository whose default branch carries at least one `active` component spec, one `accepted-pending-build` story spec, and one `superseded` (or other terminal-status) spec, plus at least one `design/*` branch with a draft — asserting `ComputeIndex`'s returned entries land each in its ratified `StatusGroup`, and that running the computation twice against the identical unmodified fixture produces byte-identical group assignments (no incidental ordering dependency).

---
id: obligation/ref-index--ac-4--behavioral
kind: obligation
title: "A design branch created but never given a spec commit yields one disclosed entry, nil error"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/ref-index" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# A design branch created but never given a spec commit yields one disclosed entry, nil error

The behavioral evidence must show a Go test over a fixture repository with a `design/*` branch cut from the default branch but never committing a `.verdi/specs/active/<name>/spec.md` (mirroring `verdi design start`'s branch-cut-before-scaffold-commit window, or an older branch that never got one), asserting `ComputeIndex` returns `nil` error, exactly one entry for that branch, and that entry's `Disclosed` field is non-nil and distinguishable (by field, not by string-sniffing) from every ordinary draft entry in the same result set.

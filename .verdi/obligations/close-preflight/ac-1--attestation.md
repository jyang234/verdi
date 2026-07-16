---
id: obligation/close-preflight--ac-1--attestation
kind: obligation
title: "The operator affirms the disclosed paths and verdicts are read from the real fold, never hand-derived"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/close-preflight" }
frozen: { at: 2026-07-16, commit: 20b0525430727bbeb168bb1a0cb5d0593f40a70d }
---
# The operator affirms the disclosed paths and verdicts are read from the real fold, never hand-derived

The attestation must affirm, after reading the merged diff: every path a
`--preflight` disclosure names (the attestation path, the derived-tree
root, the `deviation-report.md` path) is produced by calling the real
path-construction helpers this story cites (`internal/evidence/
attestations.go`, `cmd/verdi/foldload.go`, `internal/evidence/records.go`)
— never a hand-typed string literal that could drift from the real
convention if those helpers ever change; and that the preflight's
eligibility verdict for every AC, story or feature scope alike, is read
directly from the `evidence.StoryResult`/`evidence.FeatureResult` the
shared `runClosureGate`/`runFeatureClosureGate` functions already compute
(dc-2) — no second, independently-derived eligibility computation exists
anywhere in the diff.

---
id: obligation/model-digest--ac-1--behavioral
kind: obligation
title: "Behavioral tests over all four mint sites prove each artifact's provenance.model equals the resolved model's Digest(), byte-identical across repeated runs"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/model-digest" }
frozen: { at: 2026-07-17, commit: b7a0cf9801ea852fcc1f4801da11ee115f6ffc41 }
---
# Behavioral tests over all four mint sites prove each artifact's provenance.model equals the resolved model's Digest(), byte-identical across repeated runs

The behavioral evidence must show tests in each of the four minting
packages — `internal/commitdesign/commitdesign_test.go` (board freeze),
`internal/align/report_test.go` (`Generate`, deviation reports),
`internal/align/decision_report_test.go` (`GenerateDecisionConflict`), and
`internal/align/diagram_report_test.go` (`GenerateDiagramSweep`) — proving
that the artifact each call actually produces carries a `provenance.model`
field equal to `sha256:` plus the hex digest the same resolved model's
`(*model.Model).Digest()` call independently produces over the fixture
store's model. At least one case per site must exercise a fixture
`.verdi/model.yaml` distinct from the embedded canonical, proving the
stamped value tracks the actual resolved model's digest rather than a
constant string that merely happens to match the one model every other
test fixture already uses. It must also show the existing per-package
byte-identical-across-runs convention (`report_test.go`'s own
`TestGenerate_ByteIdenticalAcrossRuns` is the named precedent) extended to
cover the new field: two fresh generate calls against unchanged inputs
must produce byte-identical `model:` lines, not merely two
independently-computed digest values that happen to be equal. Green in
CI's test step.

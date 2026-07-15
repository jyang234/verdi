---
schema: verdi.deviation/v1
covers: c8f37f04d6364e70daf41595aafd202e33debd13
findings:
  - { id: f-1, kind: computed, text: "declared impacts loansvc holds at build head", disposition: fixed }
  - { id: f-2, kind: judged, text: "refi rate rounding matches spec intent", disposition: accepted-deviation, note: "rounding mode differs from the design's draft note; documented in the implementation MR" }
digest: sha256:e5fe685a3bf03764605819c0b72f33f1b8f4c5f052d99fd2796f4343d8ba80f0
integrity: sha256:eed5482959a68fc7d83cfb6e1eda7f2d636ea7cb508cb01fb5977db9696985f9
frozen: { at: 2026-06-20, commit: c8f37f04d6364e70daf41595aafd202e33debd13 }
provenance: { generator: verdi-align, version: v0, inputs: [spec/loan-refi-2023@c8f37f04d6364e70daf41595aafd202e33debd13], digest: sha256:e5fe685a3bf03764605819c0b72f33f1b8f4c5f052d99fd2796f4343d8ba80f0 }
---
# Alignment report: loan-refi-2023 (final edition)

## Computed

Declared impact on `loansvc` holds at the build head that closure covers.

## Judged

Refinance rate rounding matches the spec's intent; the exact rounding mode
differs from the draft note and is documented in the implementation MR.

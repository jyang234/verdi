---
schema: verdi.deviation/v1
covers: 4bc15842c8ee3a122a0f7911f4def1b827848bdf
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21, the persistent align-judge timeout — here even on a feature close with a minimal spec-only diff). Reviewer adjudicated the feature closure manually: the feature closure gate passed 5/5 (every AC evidenced incl. the outcome floor, stub reconciliation not blocked, every implementing story closed, no spec-stale, no pending-supersession), all four stubs are archived closures on source:ci evidence, all four outcome attestations affirm their ACs, and no decision conflicts with another decision or an ADR (ADR corpus empty). Judged coverage accepted as absent for this closure." }
digest: sha256:62ad0ae51a2712362e650b2259b5a435cbb5c0694e7f98cbea66b2b5ac251364
frozen: { at: 2026-07-13, commit: 4bc15842c8ee3a122a0f7911f4def1b827848bdf }
provenance: { generator: verdi-align, version: v0, inputs: [spec/true-closure@4bc15842c8ee3a122a0f7911f4def1b827848bdf, spec/true-closure@dd871ca09001c61c28687349fce70bc48f1313cb], digest: sha256:62ad0ae51a2712362e650b2259b5a435cbb5c0694e7f98cbea66b2b5ac251364 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [UNDISPOSITIONED]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json")

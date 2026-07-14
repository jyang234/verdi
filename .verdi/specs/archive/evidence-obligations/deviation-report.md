---
schema: verdi.deviation/v1
covers: 2872172028420851161b28e7b364bdc75f0877ed
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer adjudicated the feature closure manually: gate 5/5 (every AC evidenced incl. the outcome floor, stub reconciliation not blocked, all three implementing stories closed, no spec-stale, no pending-supersession); all four feature ACs evidenced via verify-behavioral + operator attestations at .verdi/attestations/evidence-obligations/; no decision conflicts with another decision or an ADR (ADR corpus empty). The feature dogfoods itself — its own three stories carry 12 real obligations that VL-020 now enforces and verdi matrix reads out. Closed on source:ci evidence produced at da822c2 (an ancestor of the close commit), synced before a parallel code-health workstream advanced main. make verify green. Judged coverage accepted as absent." }
digest: sha256:3a09b6cd849363de10a51161482e055f6187ea49cca5fb9af14813dc4aced656
frozen: { at: 2026-07-14, commit: 2872172028420851161b28e7b364bdc75f0877ed }
provenance: { generator: verdi-align, version: v0, inputs: [spec/evidence-obligations@2872172028420851161b28e7b364bdc75f0877ed, spec/evidence-obligations@6b0f9a0924ae8360f0c5ff77f91a2b8535926565], digest: sha256:3a09b6cd849363de10a51161482e055f6187ea49cca5fb9af14813dc4aced656 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [UNDISPOSITIONED]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json")

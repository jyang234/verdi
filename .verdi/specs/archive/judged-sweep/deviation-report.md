---
schema: verdi.deviation/v1
covers: 7d37c4618edf35b47bb3a40f14f2b3c8e203784e
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer performed the alignment review manually against spec/judged-sweep and parent ac-8/co-1: the sweep mode dispatches off the existing align verb with static witnesses proving zero references from gate.go and every lint rule (ac-1), execJudgeEnvelope + ConflictFinding/ConflictDisposition + computeIntegrity reused verbatim (ac-2/ac-3, round-trip integrity test), byte-identity read-only proof and the unconditional advisory disclosure line rendered before any finding (ac-4). Salvage handled correctly: two artifact-layer files kept on merit, the salvaged judge file's dc-5 misreading (disclosure line in the prompt builder vs the report render) caught and corrected. Judgment calls reviewed and ACCEPTED: non-proposal diagrams refused by name; --freeze mutually exclusive with the sweep (a sweep report is never frozen); provenance digest formula via canonjson over covers+ref+body-sha+corpus-digest+scanned ids; constraints included in scannedIDs per ac-2's own text; disposition preservation reused. No decision conflicts (ADR corpus empty). Gate green (139 e2e; the single-command verify raced D6-28 port contention, disclosed with both tails). Judged coverage accepted as absent for this build." }
digest: sha256:71bcfd3d99af2d6755137f19610df750e7350c35fc926e3d88a3c0d135d47e59
frozen: { at: 2026-07-14, commit: 7d37c4618edf35b47bb3a40f14f2b3c8e203784e }
provenance: { generator: verdi-align, version: v0, inputs: [spec/judged-sweep@7d37c4618edf35b47bb3a40f14f2b3c8e203784e, spec/judged-sweep@1f7f9fc4b769bd20f47bd4620ef6ad3c3cec043e], digest: sha256:71bcfd3d99af2d6755137f19610df750e7350c35fc926e3d88a3c0d135d47e59 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

### Diagram alignment

- (no accepted proposals)
- (no illustrative diagrams in this spec's body)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m (D6-21). Reviewer performed the alignment review manually against spec/judged-sweep and parent ac-8/co-1: the sweep mode dispatches off the existing align verb with static witnesses proving zero references from gate.go and every lint rule (ac-1), execJudgeEnvelope + ConflictFinding/ConflictDisposition + computeIntegrity reused verbatim (ac-2/ac-3, round-trip integrity test), byte-identity read-only proof and the unconditional advisory disclosure line rendered before any finding (ac-4). Salvage handled correctly: two artifact-layer files kept on merit, the salvaged judge file's dc-5 misreading (disclosure line in the prompt builder vs the report render) caught and corrected. Judgment calls reviewed and ACCEPTED: non-proposal diagrams refused by name; --freeze mutually exclusive with the sweep (a sweep report is never frozen); provenance digest formula via canonjson over covers+ref+body-sha+corpus-digest+scanned ids; constraints included in scannedIDs per ac-2's own text; disposition preservation reused. No decision conflicts (ADR corpus empty). Gate green (139 e2e; the single-command verify raced D6-28 port contention, disclosed with both tails). Judged coverage accepted as absent for this build.

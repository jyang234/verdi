---
schema: verdi.deviation/v1
covers: 9066dc589ee12acd1e0c23b8d0c9820d75acd9bd
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer performed the alignment review manually against spec/judged-sweep and parent ac-8/co-1: the sweep mode dispatches off the existing align verb with static witnesses proving zero references from gate.go and every lint rule (ac-1), execJudgeEnvelope + ConflictFinding/ConflictDisposition + computeIntegrity reused verbatim (ac-2/ac-3, round-trip integrity test), byte-identity read-only proof and the unconditional advisory disclosure line rendered before any finding (ac-4). Salvage handled correctly: two artifact-layer files kept on merit, the salvaged judge file's dc-5 misreading (disclosure line in the prompt builder vs the report render) caught and corrected. Judgment calls reviewed and ACCEPTED: non-proposal diagrams refused by name; --freeze mutually exclusive with the sweep (a sweep report is never frozen); provenance digest formula via canonjson over covers+ref+body-sha+corpus-digest+scanned ids; constraints included in scannedIDs per ac-2's own text; disposition preservation reused. No decision conflicts (ADR corpus empty). Gate green (139 e2e; the single-command verify raced D6-28 port contention, disclosed with both tails). Judged coverage accepted as absent for this build." }
digest: sha256:278145bf50c4298df9fb0c57e5d5315344327c61ba551a15562a87b6296213d5
provenance: { generator: verdi-align, version: v0, inputs: [spec/judged-sweep@9066dc589ee12acd1e0c23b8d0c9820d75acd9bd, spec/judged-sweep@1f7f9fc4b769bd20f47bd4620ef6ad3c3cec043e], digest: sha256:278145bf50c4298df9fb0c57e5d5315344327c61ba551a15562a87b6296213d5 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — manual reviewer alignment; salvage misreading caught and corrected; judgment calls accepted; see frontmatter note.

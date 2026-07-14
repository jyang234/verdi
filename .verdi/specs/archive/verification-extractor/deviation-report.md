---
schema: verdi.deviation/v1
covers: 1342005fb9849e3c8dc4ed814d2b45f738324094
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21; PR #49 unmerged). Reviewer performed the alignment review manually against spec/verification-extractor and the parent feature: the closed 3-value Classification with no renamed value (parent dc-5 structural), the candidate-only witness contract documented at the type (a pickaxe hit proves occurrence-count change, never causation), whole-artifact coverage downgrade (dc-1) incl. the ShortName-collision arm (dc-2), RunGraph extended in place with argv byte-identity proven for old call sites, stale-base standalone from Compare with the 2x2 independence table. Judgment calls reviewed and ACCEPTED: end/subgraph reserved out of the bare-id form (prevents silent mis-parse of subgraph closers); blank lines in-grammar; ambiguity exclusion extended to edges touching ambiguous nodes (smallest consistent reading of excluded-rather-than-guessed); artifact.ResidualDiff left minimal (DiagramDisclosedStatus reads emptiness only). No decision conflicts (ADR corpus empty). make verify green (130 e2e). Judged coverage accepted as absent for this build." }
digest: sha256:8ccaab7552c7bf6f1ee002012b31988f08a34d86222e12e567a7060064d12bc9
frozen: { at: 2026-07-14, commit: 1342005fb9849e3c8dc4ed814d2b45f738324094 }
provenance: { generator: verdi-align, version: v0, inputs: [spec/verification-extractor@1342005fb9849e3c8dc4ed814d2b45f738324094, spec/verification-extractor@afdf237718c3e2f4cc211650dd19590c0b598289], digest: sha256:8ccaab7552c7bf6f1ee002012b31988f08a34d86222e12e567a7060064d12bc9 }
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

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m (D6-21; PR #49 unmerged). Reviewer performed the alignment review manually against spec/verification-extractor and the parent feature: the closed 3-value Classification with no renamed value (parent dc-5 structural), the candidate-only witness contract documented at the type (a pickaxe hit proves occurrence-count change, never causation), whole-artifact coverage downgrade (dc-1) incl. the ShortName-collision arm (dc-2), RunGraph extended in place with argv byte-identity proven for old call sites, stale-base standalone from Compare with the 2x2 independence table. Judgment calls reviewed and ACCEPTED: end/subgraph reserved out of the bare-id form (prevents silent mis-parse of subgraph closers); blank lines in-grammar; ambiguity exclusion extended to edges touching ambiguous nodes (smallest consistent reading of excluded-rather-than-guessed); artifact.ResidualDiff left minimal (DiagramDisclosedStatus reads emptiness only). No decision conflicts (ADR corpus empty). make verify green (130 e2e). Judged coverage accepted as absent for this build.

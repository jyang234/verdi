---
schema: verdi.deviation/v1
covers: eb49f965097b148ce5f9b5b61fbcea4db7d07600
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21; PR #49 unmerged). Reviewer performed the alignment review manually against spec/verification-extractor and the parent feature: the closed 3-value Classification with no renamed value (parent dc-5 structural), the candidate-only witness contract documented at the type (a pickaxe hit proves occurrence-count change, never causation), whole-artifact coverage downgrade (dc-1) incl. the ShortName-collision arm (dc-2), RunGraph extended in place with argv byte-identity proven for old call sites, stale-base standalone from Compare with the 2x2 independence table. Judgment calls reviewed and ACCEPTED: end/subgraph reserved out of the bare-id form (prevents silent mis-parse of subgraph closers); blank lines in-grammar; ambiguity exclusion extended to edges touching ambiguous nodes (smallest consistent reading of excluded-rather-than-guessed); artifact.ResidualDiff left minimal (DiagramDisclosedStatus reads emptiness only). No decision conflicts (ADR corpus empty). make verify green (130 e2e). Judged coverage accepted as absent for this build." }
digest: sha256:7f42ded72bb1505d5994915edf1190481996feff03bc1db7cb73f61e2464b7e7
provenance: { generator: verdi-align, version: v0, inputs: [spec/verification-extractor@eb49f965097b148ce5f9b5b61fbcea4db7d07600, spec/verification-extractor@afdf237718c3e2f4cc211650dd19590c0b598289], digest: sha256:7f42ded72bb1505d5994915edf1190481996feff03bc1db7cb73f61e2464b7e7 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — manual reviewer alignment; all 4 ACs verified at their seams; judgment calls accepted; see frontmatter note.

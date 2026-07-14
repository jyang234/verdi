---
schema: verdi.deviation/v1
covers: 18be337595ff791ec5b55c65859dba2225809fcf
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21; PR #49's configurable timeout reviewed-green but unmerged at build time). Reviewer performed the alignment review manually against spec/proposal-artifact, the parent feature, and the ratified 02 §Diagram proposals: ac-1's class-conditioned enum + frozen-iff-accepted at the strict-decode seam, ac-4's realized/stale refusal enforced at the decode boundary itself (ADJ-6 made structural), ac-3's ritual mirroring spec-accept mechanics with named refusals, ac-2's byte-identity regression over idiosyncratic-whitespace fixtures, ac-5's VL-021 with the deliberate decode-vs-lint split (dangling ref / malformed digest decode cleanly so the RULE catches them, pinned by test). Judgment calls reviewed and accepted: scope/derived_from not decode-forbidden on incumbents (smallest reversible); ResidualDiff minimal stand-in pending verification-extractor; VL-004 needs no code change (sole-legal-writer posture matches the existing superseded flip). No decision conflicts (ADR corpus empty). make verify green (130 e2e). Judged coverage accepted as absent for this build." }
digest: sha256:799755da27c3e8a16e2a741495df6c431f5dc7ed73fd2562bbaf5843abbc54a2
frozen: { at: 2026-07-14, commit: 18be337595ff791ec5b55c65859dba2225809fcf }
provenance: { generator: verdi-align, version: v0, inputs: [spec/proposal-artifact@18be337595ff791ec5b55c65859dba2225809fcf, spec/proposal-artifact@3427df94dafda631fb425e1cd9f2a6d825aaf02c], digest: sha256:799755da27c3e8a16e2a741495df6c431f5dc7ed73fd2562bbaf5843abbc54a2 }
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

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m (D6-21; PR #49's configurable timeout reviewed-green but unmerged at build time). Reviewer performed the alignment review manually against spec/proposal-artifact, the parent feature, and the ratified 02 §Diagram proposals: ac-1's class-conditioned enum + frozen-iff-accepted at the strict-decode seam, ac-4's realized/stale refusal enforced at the decode boundary itself (ADJ-6 made structural), ac-3's ritual mirroring spec-accept mechanics with named refusals, ac-2's byte-identity regression over idiosyncratic-whitespace fixtures, ac-5's VL-021 with the deliberate decode-vs-lint split (dangling ref / malformed digest decode cleanly so the RULE catches them, pinned by test). Judgment calls reviewed and accepted: scope/derived_from not decode-forbidden on incumbents (smallest reversible); ResidualDiff minimal stand-in pending verification-extractor; VL-004 needs no code change (sole-legal-writer posture matches the existing superseded flip). No decision conflicts (ADR corpus empty). make verify green (130 e2e). Judged coverage accepted as absent for this build.

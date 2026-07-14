---
schema: verdi.deviation/v1
covers: 18be337595ff791ec5b55c65859dba2225809fcf
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer performed the alignment review manually against spec/badge-computes and the parent wall-receipts: attachBadges as the ONE loadBoard attachment point feeding page/fragment/get_board (parity test), the dc-3 self-classification realized as Finding.Locus with nothing-declared = off-the-wall fail-closed, ac-3's same-code-path constraint witnessed statically (decisionsweep.ScanSpecStale / evidence.PendingSupersession call sites), dc-5 digest revisions incl. the OpenSupersessionCandidate.Digest addition, dc-4's button + data-badge-source contract with server-only rendering, e2e across all three modes. Adjudicated and ACCEPTED (ADJ-8): (1) the spec-level locus bucket includes VL-014/VL-015 as the closest spec-stale-adjacent lint rules (spec-stale itself is a ladder compute, not a lint rule - the spec's enumeration was loose here); (2) Path == spec.md exact-match scoping, behaviorally identical to directory-prefix today, tighter and documented; (3) internal/wallbadge as the record home preserving workbench's no-forge-import boundary, with the SupersessionCandidateLoader port mirroring reviewfeed; (4) data-badge-record as the serialized-record attribute name - RATIFIED as the drawer opener contract, the derivation-drawer story must consume exactly this name; (5) chips speak the disclosed-ochre register, never red - receipts, not alarms. No decision conflicts (ADR corpus empty). make verify green (133 e2e). Judged coverage accepted as absent for this build." }
digest: sha256:799755da27c3e8a16e2a741495df6c431f5dc7ed73fd2562bbaf5843abbc54a2
frozen: { at: 2026-07-14, commit: 18be337595ff791ec5b55c65859dba2225809fcf }
provenance: { generator: verdi-align, version: v0, inputs: [spec/badge-computes@18be337595ff791ec5b55c65859dba2225809fcf, spec/badge-computes@b8a2002dcced29c5455e69d6103cafb1a97712fb], digest: sha256:799755da27c3e8a16e2a741495df6c431f5dc7ed73fd2562bbaf5843abbc54a2 }
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

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m (D6-21). Reviewer performed the alignment review manually against spec/badge-computes and the parent wall-receipts: attachBadges as the ONE loadBoard attachment point feeding page/fragment/get_board (parity test), the dc-3 self-classification realized as Finding.Locus with nothing-declared = off-the-wall fail-closed, ac-3's same-code-path constraint witnessed statically (decisionsweep.ScanSpecStale / evidence.PendingSupersession call sites), dc-5 digest revisions incl. the OpenSupersessionCandidate.Digest addition, dc-4's button + data-badge-source contract with server-only rendering, e2e across all three modes. Adjudicated and ACCEPTED (ADJ-8): (1) the spec-level locus bucket includes VL-014/VL-015 as the closest spec-stale-adjacent lint rules (spec-stale itself is a ladder compute, not a lint rule - the spec's enumeration was loose here); (2) Path == spec.md exact-match scoping, behaviorally identical to directory-prefix today, tighter and documented; (3) internal/wallbadge as the record home preserving workbench's no-forge-import boundary, with the SupersessionCandidateLoader port mirroring reviewfeed; (4) data-badge-record as the serialized-record attribute name - RATIFIED as the drawer opener contract, the derivation-drawer story must consume exactly this name; (5) chips speak the disclosed-ochre register, never red - receipts, not alarms. No decision conflicts (ADR corpus empty). make verify green (133 e2e). Judged coverage accepted as absent for this build.

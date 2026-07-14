---
schema: verdi.deviation/v1
covers: 36c1ec20ff411ec88f8d0ee5016eac7356f282b7
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer performed the alignment review manually against spec/draft-boards and the parent: dc-1's /b/{branch-escaped}/ prefix mounting the EXISTING board route table per branch over the wtmanager-obtained worktree (one server instance per branch, no second implementation), ac-2's two-tab isolation proven end to end with the inspection server (an authoring edit lands only in its own branch's worktree; the serving checkout stays clean), ac-3's no-new-mode witnessed statically (the branch-state mode computation applied per instance), dc-4's remote-only sealed render with no worktree cut and no minted branch, and the directory-home reconciliation (the /b/ catch-all now routes live branches to the real handler, dead ones to the disclosed 404). Adjudicated and ACCEPTED (ADJ-17): (1) the inspection server folded into the D6-28 port quartet as base+3 (an infrastructure completion of PR #76's own pattern, unset = historical 4178 byte-for-byte, unit-tabled); (2) the SIGTERMed first verify disclosed and superseded by a clean watched 155/155 run. No decision conflicts (ADR corpus empty). make verify green (155 e2e). Judged coverage accepted as absent for this build." }
digest: sha256:f3b719a882ec1841791971ae38e5e29d64caeaad5c199ac42b1065d7680ec5f6
frozen: { at: 2026-07-14, commit: 36c1ec20ff411ec88f8d0ee5016eac7356f282b7 }
provenance: { generator: verdi-align, version: v0, inputs: [spec/draft-boards@36c1ec20ff411ec88f8d0ee5016eac7356f282b7, spec/draft-boards@26646b193ba1be5466d3a7158e56d203bb7a08d2], digest: sha256:f3b719a882ec1841791971ae38e5e29d64caeaad5c199ac42b1065d7680ec5f6 }
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

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m (D6-21). Reviewer performed the alignment review manually against spec/draft-boards and the parent: dc-1's /b/{branch-escaped}/ prefix mounting the EXISTING board route table per branch over the wtmanager-obtained worktree (one server instance per branch, no second implementation), ac-2's two-tab isolation proven end to end with the inspection server (an authoring edit lands only in its own branch's worktree; the serving checkout stays clean), ac-3's no-new-mode witnessed statically (the branch-state mode computation applied per instance), dc-4's remote-only sealed render with no worktree cut and no minted branch, and the directory-home reconciliation (the /b/ catch-all now routes live branches to the real handler, dead ones to the disclosed 404). Adjudicated and ACCEPTED (ADJ-17): (1) the inspection server folded into the D6-28 port quartet as base+3 (an infrastructure completion of PR #76's own pattern, unset = historical 4178 byte-for-byte, unit-tabled); (2) the SIGTERMed first verify disclosed and superseded by a clean watched 155/155 run. No decision conflicts (ADR corpus empty). make verify green (155 e2e). Judged coverage accepted as absent for this build.

---
schema: verdi.deviation/v1
covers: 01811b761f1b0e27fa108e1f1b1706e8a8b7830c
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer performed the alignment review manually against spec/draft-boards and the parent: dc-1's /b/{branch-escaped}/ prefix mounting the EXISTING board route table per branch over the wtmanager-obtained worktree (one server instance per branch, no second implementation), ac-2's two-tab isolation proven end to end with the inspection server (an authoring edit lands only in its own branch's worktree; the serving checkout stays clean), ac-3's no-new-mode witnessed statically (the branch-state mode computation applied per instance), dc-4's remote-only sealed render with no worktree cut and no minted branch, and the directory-home reconciliation (the /b/ catch-all now routes live branches to the real handler, dead ones to the disclosed 404). Adjudicated and ACCEPTED (ADJ-17): (1) the inspection server folded into the D6-28 port quartet as base+3 (an infrastructure completion of PR #76's own pattern, unset = historical 4178 byte-for-byte, unit-tabled); (2) the SIGTERMed first verify disclosed and superseded by a clean watched 155/155 run. No decision conflicts (ADR corpus empty). make verify green (155 e2e). Judged coverage accepted as absent for this build." }
digest: sha256:b28de8a79ef7416c365769e2ab5ee92a5fa3f07ac48143daff5cdf1a299a4815
provenance: { generator: verdi-align, version: v0, inputs: [spec/draft-boards@01811b761f1b0e27fa108e1f1b1706e8a8b7830c, spec/draft-boards@26646b193ba1be5466d3a7158e56d203bb7a08d2], digest: sha256:b28de8a79ef7416c365769e2ab5ee92a5fa3f07ac48143daff5cdf1a299a4815 }
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

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — manual reviewer alignment; ADJ-17 calls ratified; see frontmatter note.

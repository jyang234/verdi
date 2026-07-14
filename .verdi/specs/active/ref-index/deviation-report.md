---
schema: verdi.deviation/v1
covers: a0be9fcf8e1a690f2b00168426e87a581bf644e5
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21; the configurable-timeout fix is reviewed-green in PR #49, unmerged at build time). Reviewer performed the alignment review manually against spec/ref-index and the parent feature: all 5 ACs implemented at their named seams (ComputeIndex/port/entry/status + gitx LsTree/RemoteDesignBranches), the ac-5 no-checkout guarantee doubly witnessed (reflection-pinned port method set + before/after byte-identity), ac-3's unconditional drafts-in-progress override proven against a contrary frontmatter fixture, and no decision conflicts with another decision or an ADR (corpus empty). One build-time judgment call adjudicated and ACCEPTED (ADJ-7): the ac-4 content probe runs BEFORE dc-5's merged-branch exclusion, because gitx.IsAncestor's self-inclusive semantics would classify a freshly-cut, zero-commit design branch as trivially merged and silently drop exactly the disclosed entry ac-4 requires; dc-5's own rationale (no duplicate for REAL content) is preserved intact for branches with content. make verify green (130 e2e). Judged coverage accepted as absent for this build." }
digest: sha256:e045df03fbaf6de99e27abde62a1d4efe31f358628cda3dee56e62f2e8d5a624
provenance: { generator: verdi-align, version: v0, inputs: [spec/ref-index@a0be9fcf8e1a690f2b00168426e87a581bf644e5, spec/ref-index@7e425b6ed982b44605c29bef0b0580565e8a9cbc], digest: sha256:e045df03fbaf6de99e27abde62a1d4efe31f358628cda3dee56e62f2e8d5a624 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m (D6-21). Reviewer performed the alignment review manually; all 5 ACs verified at their seams; ADJ-7 sequencing call (content probe before merged-exclusion) accepted; no decision conflicts (ADR corpus empty). See frontmatter note.

---
schema: verdi.deviation/v1
covers: 3b39555646b82531e80424c0c451bf4c4e8fa783
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer performed the alignment review manually against spec/board-editor and the parent: dc-2's structural-op grammar as pure byte-splicing functions (add/connect/rename-label-only/delete, ops disclosed-unavailable outside the subset), ac-3 byte preservation across every editor write path, ac-1's fail-visible preview under the one vendored mermaid, dc-4's rail consuming the extractor seam with the disclosed unavailable state, dc-1's route grammar mirroring /board/spec/. THE substantive adjudication: the build's diagrambase digest formula unilaterally redefined derived_from.digest to the Parse convention, contradicting verification-extractor's frozen ac-4 (flowmap-regenerated comparand) — REMEDIATED per ADJ-16 (ratified into 02+08+mirror): derived_from gains optional source_digest (the Parse formula, hermetic, gates peek/reset; absent = disclosed-unavailable) while digest keeps flowmap semantics for stale-base; align's StaleBase call verified to stay on .Digest; VL-021 format-checks source_digest when present; a substring-matching fixture test that would have masked the split was tightened. Also accepted: the acceptdiagram write-path inventory amendment (D6-18 discipline) and the incidental boardspec.js link-swallow fix (real <a> links now navigate — a pre-existing defect surfaced by the editor reachability link). Composition with illustrative-class verified (client-side preview never touches the server render dispatch; 25/25 joint suite). No decision conflicts (ADR corpus empty). make verify green (169 e2e, twice + post-remediation). Judged coverage accepted as absent for this build." }
digest: sha256:7468a707a0008e01009d5862b3618b2d70abffcaadb40fe4d5523e699c7534b1
provenance: { generator: verdi-align, version: v0, inputs: [spec/board-editor@3b39555646b82531e80424c0c451bf4c4e8fa783, spec/board-editor@0781efbfac98ccfe474f4bf93b68f88f90c60299], digest: sha256:7468a707a0008e01009d5862b3618b2d70abffcaadb40fe4d5523e699c7534b1 }
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

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — manual reviewer alignment; ADJ-16 remediation applied on-branch; see frontmatter note.

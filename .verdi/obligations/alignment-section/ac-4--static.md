---
id: obligation/alignment-section--ac-4--static
kind: obligation
title: "Diagram findings are appended into the single findings slice ComputeDigest already covers, no second digest field"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/alignment-section" }
frozen: { at: 2026-07-14, commit: bd4e93b262179dc1ff3d3c363fd66addb1a875c9 }
---
# Diagram findings are appended into the single findings slice ComputeDigest already covers, no second digest field

The static evidence must show the diagram Findings this story produces are
appended into the SAME `[]artifact.Finding` slice `Compute`'s existing
boundary findings already populate, before that combined slice reaches
`ComputeDigest` — i.e. `ComputeDigest`'s signature is unchanged and
`artifact.DeviationFrontmatter` gains no new digest-shaped field for
diagrams. The evidence must also show these Findings use `id:
"diagram-<name>"` (mirroring the existing `"boundary-..."` id convention)
so they are unambiguously distinguishable from boundary findings in the
same list without a new Kind value.

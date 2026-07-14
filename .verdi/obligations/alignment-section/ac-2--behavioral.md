---
id: obligation/alignment-section--ac-2--behavioral
kind: obligation
title: "A test proves an unchanged proposal discloses realized and a divergent one discloses divergent with a witness"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/alignment-section" }
frozen: { at: 2026-07-14, commit: bd4e93b262179dc1ff3d3c363fd66addb1a875c9 }
---
# A test proves an unchanged proposal discloses realized and a divergent one discloses divergent with a witness

The behavioral evidence must show a test over a fixture accepted proposal
whose declared elements are still all present in a canned truth graph,
asserting the resulting computed Finding's text discloses
`realized (full coverage)`; a second fixture proposal whose
base-inherited element the canned truth graph (at a later fixturegit
commit) no longer has, asserting the resulting Finding's text discloses
`divergent` and names that element's candidate witness commit sha; and a
third fixture proposal whose mermaid source falls outside
verification-extractor's declared grammar (partial coverage) but whose
comparable elements show no divergence, asserting the Finding's text
discloses partial coverage explicitly (e.g. "realized (partial coverage —
N elements excluded)") rather than a bare `realized` indistinguishable
from the full-coverage case. All three must use the fake
`Runner`/fixturegit seam already established by verification-extractor,
never a live flowmap exec.

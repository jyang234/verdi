---
id: obligation/alignment-section--ac-1--behavioral
kind: obligation
title: "A fixture test proves corpus-wide proposal discovery and spec-scoped illustrative discovery, with no cross-leakage"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/alignment-section" }
frozen: { at: 2026-07-14, commit: bd4e93b262179dc1ff3d3c363fd66addb1a875c9 }
---
# A fixture test proves corpus-wide proposal discovery and spec-scoped illustrative discovery, with no cross-leakage

The behavioral evidence must show a test fixture with: two accepted
`class: proposal` diagrams (unrelated to the spec under test by any
`impacts:` or link) and one `status: proposed` (not yet accepted) diagram,
asserting discovery returns exactly the two accepted ones and not the
proposed one; a fixture spec body containing one fenced illustrative
mermaid block, and a SECOND, unrelated spec's body also containing a
fenced block, asserting illustrative discovery for the spec under test
returns only its own body's figure, never the other spec's.

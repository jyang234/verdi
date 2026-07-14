---
id: obligation/alignment-section--ac-4--behavioral
kind: obligation
title: "A round-trip test proves a diagram finding's disposition survives regeneration through PreserveDispositions"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/alignment-section" }
frozen: { at: 2026-07-14, commit: bd4e93b262179dc1ff3d3c363fd66addb1a875c9 }
---
# A round-trip test proves a diagram finding's disposition survives regeneration through PreserveDispositions

The behavioral evidence must show a test that: generates a report
containing a divergent diagram finding; hand-dispositions it
`accepted-deviation` with a note (mirroring how an existing boundary
finding's disposition is set in this codebase's own tests); regenerates
the report against the SAME unchanged inputs via `Generate`/`Compute`
again, passing the prior findings as `ExistingFindings`; and asserts the
diagram finding's disposition and note survive unchanged in the new
report, exactly as an existing boundary finding's disposition already
does through `PreserveDispositions`. This proves the diagram finding is a
first-class citizen of the SAME preservation path, not a special case.

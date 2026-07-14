---
id: obligation/proposal-artifact--ac-4--static
kind: obligation
title: "DiagramDisclosedStatus is a pure function, and realized/stale are absent from the authored enum"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/proposal-artifact" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# DiagramDisclosedStatus is a pure function, and realized/stale are absent from the authored enum

The static evidence must show a function with the signature
`DiagramDisclosedStatus(fm DiagramFrontmatter, residual *ResidualDiff) Status`
(or an equivalent named type standing in for `ResidualDiff` — the type
verification-extractor's diff produces, consumed here by reference, not
redefined) that takes no I/O, no clock, and no global state, and that
`proposalStatuses` (AC-1's enum) contains only `proposed` and `accepted` —
`realized` and `stale` do not appear in it anywhere in the source, which is
the mechanism by which strict decode refuses them as authored input.

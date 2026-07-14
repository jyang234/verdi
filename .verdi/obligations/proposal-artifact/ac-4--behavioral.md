---
id: obligation/proposal-artifact--ac-4--behavioral
kind: obligation
title: "A table-driven test covers every DiagramDisclosedStatus input and proves realized/stale are decode-rejected"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/proposal-artifact" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# A table-driven test covers every DiagramDisclosedStatus input and proves realized/stale are decode-rejected

The behavioral evidence must show a table-driven test over
`DiagramDisclosedStatus` covering: `status: proposed` with `residual: nil`
→ `proposed`; `status: accepted` with `residual: nil` → `accepted`;
`status: accepted` with an empty residual → `realized`; `status: accepted`
with a non-empty residual → `stale`. It must also show a decode test
asserting that a frontmatter fixture literally containing
`status: realized` or `status: stale` under `class: proposal` fails
`DecodeDiagram` with a named "not a known status" error — proving the
never-written invariant is enforced at the decode boundary, not merely by
convention.

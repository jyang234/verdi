---
id: obligation/alignment-section--ac-1--static
kind: obligation
title: "Two discovery functions are named: corpus-wide accepted proposals, and spec-body illustrative figures"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/alignment-section" }
frozen: { at: 2026-07-14, commit: bd4e93b262179dc1ff3d3c363fd66addb1a875c9 }
---
# Two discovery functions are named: corpus-wide accepted proposals, and spec-body illustrative figures

The static evidence must show two distinct discovery functions in
`internal/align` (a new file, e.g. `diagram_computed.go`): one that walks
`.verdi/diagrams/*` (or the store's known diagram set) and returns every
diagram whose decoded frontmatter is `class: proposal` and `status:
accepted`, scoped to the WHOLE corpus (no filter by the current spec's
`impacts:` or any other field — DC-1's documented scope decision); and one
that scans ONLY the current spec's own body text for fenced ` ```mermaid `
blocks and `diagrams/` references, scoped to that spec alone. The evidence
must show neither function silently drops a decode failure or an empty
result — an empty set is returned as an explicit empty slice, not nil
treated as "nothing to check" without disclosure.

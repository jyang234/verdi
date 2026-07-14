---
id: obligation/proposal-artifact--ac-1--behavioral
kind: obligation
title: "Table-driven decode/validate tests cover class: proposal's happy path and every negative"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/proposal-artifact" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Table-driven decode/validate tests cover class: proposal's happy path and every negative

The behavioral evidence must show `internal/artifact/diagram_test.go` table
cases that: (1) round-trip a `class: proposal, status: proposed` diagram
with no `frozen:` and no `derived_from:` cleanly; (2) round-trip a
`class: proposal, status: accepted` diagram carrying `frozen:` and a
well-formed `derived_from: {ref, digest}` cleanly; (3) reject
`class: proposal, status: active` (an incumbent-only status leaking into a
proposal); (4) reject a `class: proposal, status: accepted` diagram missing
`frozen:`; (5) reject a `class: proposal, status: proposed` diagram that
illegally carries `frozen:`; (6) reject an unknown frontmatter field on a
`class: proposal` diagram; and (7) confirm an existing incumbent diagram
fixture (`class` absent) with `status: active`/`superseded` still decodes
exactly as before this change, unaffected by the new branch.

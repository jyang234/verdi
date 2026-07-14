---
id: obligation/proposal-artifact--ac-2--static
kind: obligation
title: "Every diagram write path is inventoried and shown to touch frontmatter bytes only"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/proposal-artifact" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Every diagram write path is inventoried and shown to touch frontmatter bytes only

The static evidence must show, by naming each real write path that can
persist a `kind: diagram` file (the workbench/board save API, `verdi accept
diagram/<name>`'s status-flip write, and any scaffold/copy path
`verdi design start` exercises), that each one operates via
`artifact.SplitFrontmatter` (or equivalent) and re-emits the body slice
verbatim rather than any parsed/re-marshaled representation — i.e. the
function signature or call site for each write path is named and shown to
never construct a mermaid string from a graph, board, or other
intermediate structure. An enumeration that turns up a path this story
does not yet cover must say so explicitly rather than omit it.

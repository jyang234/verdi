---
id: obligation/alignment-section--ac-3--static
kind: obligation
title: "RenderBody's diagram-alignment subsection mirrors renderBaselineDiffs' shape, never omitted when empty"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/alignment-section" }
frozen: { at: 2026-07-14, commit: bd4e93b262179dc1ff3d3c363fd66addb1a875c9 }
---
# RenderBody's diagram-alignment subsection mirrors renderBaselineDiffs' shape, never omitted when empty

The static evidence must show `internal/align/render.go`'s `RenderBody`
emits a `### Diagram alignment` heading under `## Computed` (alongside the
existing `### Boundary diff vs acceptance baseline`), and a rendering
function parallel to `renderBaselineDiffs` in structure: one line per
accepted proposal, one line per illustrative diagram, and an explicit
placeholder line (not an omitted heading) when either set is empty. The
evidence must show this heading and its render call are unconditional in
`RenderBody` — never behind an `if len(...) > 0` guard that would make the
whole subsection vanish rather than read empty.

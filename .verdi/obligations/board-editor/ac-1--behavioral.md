---
id: obligation/board-editor--ac-1--behavioral
kind: obligation
title: "e2e drives the editor pane and sees the SVG on valid source and the visible renderer error on invalid source"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/board-editor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# e2e drives the editor pane and sees the SVG on valid source and the visible renderer error on invalid source

The behavioral evidence must show a Playwright e2e spec under e2e/tests/ that
opens GET /board/diagram/{name} against a fixture store containing a
class: proposal diagram and proves, in the live page: (1) the code pane holds
the artifact's mermaid source; (2) with valid flowchart source the preview
region contains a rendered `<svg>` produced by the vendored
/assets/mermaid.min.js (no external request — the suite runs with no network);
(3) after typing source the pinned renderer rejects, the preview shows a
visible render-error element carrying the renderer's own message text, the
previous SVG is NOT silently retained, and the pane text is untouched.
Evidence that only exercises valid source, or that asserts an error state
without asserting the stale-picture removal, does not satisfy this
obligation.

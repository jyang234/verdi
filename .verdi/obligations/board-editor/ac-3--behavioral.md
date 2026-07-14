---
id: obligation/board-editor--ac-3--behavioral
kind: obligation
title: "e2e pastes an adversarially formatted diagram, saves, reloads, and reads it back bit-identical"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/board-editor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# e2e pastes an adversarially formatted diagram, saves, reloads, and reads it back bit-identical

The behavioral evidence must show a Playwright e2e test that pastes a
mermaid source with deliberately unusual but renderer-legal formatting
(mixed indentation, %% comments, blank lines, trailing spaces) into the code
pane, saves it through the editor's real save action, reloads the page, and
asserts the pane content is bit-identical to what was pasted — the feature's
"pasted diagrams survive bit-for-bit" claim exercised through the full
HTTP-write-to-disk-to-reload loop, not a unit seam. It must additionally
perform one structural operation on that pasted source and assert the
reloaded document differs from the pre-op text only on the op's own lines.
Evidence that compares normalized or trimmed strings does not satisfy this
obligation.

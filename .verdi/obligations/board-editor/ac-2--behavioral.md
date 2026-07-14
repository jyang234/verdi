---
id: obligation/board-editor--ac-2--behavioral
kind: obligation
title: "e2e performs every structural operation and asserts the exact source text each one produces"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/board-editor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# e2e performs every structural operation and asserts the exact source text each one produces

The behavioral evidence must show Playwright e2e coverage that performs each
of the four operations through the real UI against a fixture proposal and
asserts the resulting code-pane source text (not just the picture): add node
(a new `n<k>["<label>"]` line appears with the expected next-free id),
connect via BOTH gestures the spec names — click-click and drag-to-connect —
(a new `<from> --> <to>` line appears; the drag changes no node's placement
and stores nothing spatial), rename inline (only the label text changes; the
node id is byte-identical before and after), and delete (the node's defining
line and its referencing edge lines are gone). It must also prove the
disclosed-unavailable path: on a fixture whose body is outside the op
grammar's flowchart subset, the structural operations are visibly disclosed
unavailable while typing in the code pane still works. Evidence exercising
only one gesture for connect, or asserting operations by preview appearance
alone, does not satisfy this obligation.

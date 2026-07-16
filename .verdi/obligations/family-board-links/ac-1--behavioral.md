---
id: obligation/family-board-links--ac-1--behavioral
kind: obligation
title: "An e2e follows a story board's parent-feature affordance to the feature's own board"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/family-board-links" }
frozen: { at: 2026-07-16, commit: 71bf8422596fadd4c3060aa719785b91e114924f }
---
# An e2e follows a story board's parent-feature affordance to the feature's own board

The behavioral evidence must show a Playwright e2e in
`e2e/tests/43-family-board-links.spec.ts` driving a served fixture store — the
already-committed showcase story board `borrower-update-api`, whose
document-level `implements` edge names `spec/stale-decline#ac-2` — and
asserting the rendered parent-feature affordance resolves to
`/board/spec/stale-decline` (the feature's own board, dc-5), then following it
and landing on that feature board. No network in any case (co-2).

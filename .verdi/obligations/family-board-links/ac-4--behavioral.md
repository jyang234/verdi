---
id: obligation/family-board-links--ac-4--behavioral
kind: obligation
title: "An e2e sees the unresolved-ref notice and no dead link on a story board whose implements target is absent"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/family-board-links" }
frozen: { at: 2026-07-16, commit: 71bf8422596fadd4c3060aa719785b91e114924f }
---
# An e2e sees the unresolved-ref notice and no dead link on a story board whose implements target is absent

The behavioral evidence must show a Playwright e2e in
`e2e/tests/43-family-board-links.spec.ts` driving an EDGE-zone fixture (dc-5;
`fixtures.ts`'s SHOWCASE/EDGE convention) — a story board whose `implements`
edge targets a feature ref absent from the store — and asserting the board
renders a disclosed inline notice naming the unresolved ref in place of the
affordance, and that NO dead `<a href>` is rendered for it (co-3). No network
(co-2).

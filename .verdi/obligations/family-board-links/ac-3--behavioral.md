---
id: obligation/family-board-links--ac-3--behavioral
kind: obligation
title: "An e2e sees the in-between disclosure for a design-branch stub with no match, and the plain state for a no-match-no-ref stub"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/family-board-links" }
frozen: { at: 2026-07-16, commit: 71bf8422596fadd4c3060aa719785b91e114924f }
---
# An e2e sees the in-between disclosure for a design-branch stub with no match, and the plain state for a no-match-no-ref stub

The behavioral evidence must show a Playwright e2e in
`e2e/tests/43-family-board-links.spec.ts` driving the NEW `cmd/e2eharness`
fixture (dc-5) whose stub's `refs/heads/design/<slug>` exists locally with no
matching spec anywhere in the served checkout's store, asserting the card
discloses "instantiated on design/<slug>, not yet in this checkout's active
store" with the correct branch name shown. Per ADJ-28's firing semantics it must
ALSO drive a no-match-no-ref stub (no matching spec anywhere AND no
`design/<slug>` branch) and assert the plain un-instantiated state renders
unchanged — never the in-between notice. No network (co-2).

---
id: obligation/tool-view-exit--ac-1--behavioral
kind: obligation
title: "e2e drives the diagram designer's exit affordance and Escape, and the honest-degradation fallback, against the originating board"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/tool-view-exit" }
frozen: { at: 2026-07-16, commit: 7ebf1b7522beb2a3e103fd2e1c332fe281bf4c11 }
---
# e2e drives the diagram designer's exit affordance and Escape, and the honest-degradation fallback, against the originating board

The behavioral evidence must show `e2e/tests/43-tool-view-exit.spec.ts`, a
Playwright spec that drives the built binary against a fixture store
containing a spec board with a pinned `class: proposal` diagram reference
card, and proves, in the live page: (1) following the reference card into
`/board/diagram/{name}` shows a visible exit affordance in the page
chrome, distinct from the existing `index`/`artifact` nav links; (2)
activating that affordance navigates back to the exact spec board
(`/board/spec/<name>`) it was entered from, which renders fully — its own
cards, not a blank or error page; (3) entering the editor again and
pressing Escape produces the identical return, proving the affordance and
Escape are two paths to the same exit, not two different behaviors; and
(4) opening the editor with no originating board known (no `board` query
parameter) shows the affordance and Escape both falling back to the
index, labeled honestly as not knowing an originating board, never a
broken link. Evidence that proves only the affordance, or only Escape, or
omits the no-origin fallback, does not satisfy this obligation.

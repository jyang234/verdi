---
id: obligation/obligation-artifact--ac-3--behavioral
kind: obligation
title: "A browser e2e graduates a sticky into an obligation and refuses a bad target"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-artifact" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# A browser e2e graduates a sticky into an obligation and refuses a bad target

The behavioral evidence must show a Playwright e2e (35-board-obligation-graduate) proving that, on a story wall, a scratch sticky's yarn dropped on an AC opens the for_kind picker and graduates into a persisted obligation file bound to that AC, and that a drop on a non-AC target (a decision, an undeclared AC) or on a non-story wall is refused legibly with nothing written.

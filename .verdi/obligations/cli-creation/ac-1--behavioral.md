---
id: obligation/cli-creation--ac-1--behavioral
kind: obligation
title: "scaffolded obligation: ac-1 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/cli-creation" }
frozen: { at: 2026-07-22, commit: e1cd2d1f957a200804b97a78829482d1ca8b57f9 }
---
# scaffolded obligation: ac-1 behavioral evidence

This obligation was scaffolded at accept; not elaborated. It is a placeholder for ac-1's behavioral evidence, written by accept's
freeze-moment backstop because no obligation existed for this pair
when spec/cli-creation was accepted (spec/creation-surfaces#ac-4). Replace this body
with a first-person statement of what that evidence must specifically
show before relying on it — by hand, or via `verdi obligation author
spec/cli-creation ac-1 behavioral` on a design branch before the replacement itself freezes.
The acceptance criterion's own declared text, for reference:

design start grows a --problem and --outcome pair of flags: given together, they source the scaffold's problem/outcome sections directly, so every section the class template declares renders TODO-free — never the `TODO: replace with the real problem statement before accept` / `TODO: design notes.` placeholders the unflagged path always emitted before this story. --defer-statements is the opposite, explicit choice: it commits the same placeholder TODOs the old default always did, but never silently — the invocation prints a disclosure line naming problem/outcome as deliberately deferred, so a reader of the ritual's own output can see the deferral was chosen, not missed. The two are mutually exclusive with each other, and --problem/--outcome must be given together or not at all — a lone flag refuses by name rather than leaving one section templated and the other not


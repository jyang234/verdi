---
id: obligation/cli-creation--ac-2--behavioral
kind: obligation
title: "scaffolded obligation: ac-2 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/cli-creation" }
frozen: { at: 2026-07-22, commit: e1cd2d1f957a200804b97a78829482d1ca8b57f9 }
---
# scaffolded obligation: ac-2 behavioral evidence

This obligation was scaffolded at accept; not elaborated. It is a placeholder for ac-2's behavioral evidence, written by accept's
freeze-moment backstop because no obligation existed for this pair
when spec/cli-creation was accepted (spec/creation-surfaces#ac-4). Replace this body
with a first-person statement of what that evidence must specifically
show before relying on it — by hand, or via `verdi obligation author
spec/cli-creation ac-2 behavioral` on a design branch before the replacement itself freezes.
The acceptance criterion's own declared text, for reference:

given no creation flags at all — no --problem, no --outcome, no --defer-statements — on an attached terminal, design start runs a TTY interview that prompts for exactly the class template's own statement placeholders, enumerated through internal/designscaffold.Fields, the identical descriptor list the board's creation form already validates its own submissions against (spec/creation-form ac-1): one field contract, two front ends, never a second hand-rolled field list to drift from the first. The identical invocation with no creation flags and no attached terminal refuses outright, by name, rather than falling back to the old silent TODO placeholders: statement fields are required content now, exactly as the board form already requires them, and every non-interactive way to skip them is the explicit --defer-statements flag, never an implicit default


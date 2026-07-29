---
id: obligation/instantiate-from-stub--ac-1--behavioral
kind: obligation
title: "scaffolded obligation: ac-1 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/instantiate-from-stub" }
frozen: { at: 2026-07-28, commit: 3466590a25b3c655f4fb4d385b3dd50f08a6b62e }
---
# scaffolded obligation: ac-1 behavioral evidence

This obligation was scaffolded at accept; not elaborated. It is a placeholder for ac-1's behavioral evidence, written by accept's
freeze-moment backstop because no obligation existed for this pair
when spec/instantiate-from-stub was accepted (spec/creation-surfaces#ac-4). Replace this body
with a first-person statement of what that evidence must specifically
show before relying on it — by hand, or via `verdi obligation author
spec/instantiate-from-stub ac-1 behavioral` on a design branch before the replacement itself freezes.
The acceptance criterion's own declared text, for reference:

instantiating a plain stub scaffolds the story spec pre-filled — title from the slug, a story-ref prompt, implements edges to exactly the stub's acceptance_criteria — bound to its stub by slug equality with no new provenance record, on a design branch cut and committed by the action


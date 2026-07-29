---
id: obligation/spike-resolution--ac-3--behavioral
kind: obligation
title: "scaffolded obligation: ac-3 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/spike-resolution" }
frozen: { at: 2026-07-28, commit: 14f42c45969427cdded6384136be7b88008f34a9 }
---
# scaffolded obligation: ac-3 behavioral evidence

This obligation was scaffolded at accept; not elaborated. It is a placeholder for ac-3's behavioral evidence, written by accept's
freeze-moment backstop because no obligation existed for this pair
when spec/spike-resolution was accepted (spec/creation-surfaces#ac-4). Replace this body
with a first-person statement of what that evidence must specifically
show before relying on it — by hand, or via `verdi obligation author
spec/spike-resolution ac-3 behavioral` on a design branch before the replacement itself freezes.
The acceptance criterion's own declared text, for reference:

the round-5.4 grammar fails closed: resolves requires spike: true, a spike stub declares resolves and no acceptance_criteria, a plain stub the reverse — and a resolves entry naming an undeclared open question of the same spec is a named lint refusal


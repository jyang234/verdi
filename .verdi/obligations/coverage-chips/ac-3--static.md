---
id: obligation/coverage-chips--ac-3--static
kind: obligation
title: "scaffolded obligation: ac-3 static evidence"
owners: ["johnyang"]
for_kind: static
links:
  - { type: verifies, ref: "spec/coverage-chips" }
frozen: { at: 2026-07-28, commit: ef6760fcdbc2ccc9a32bb4871508c200dc08e768 }
---
# scaffolded obligation: ac-3 static evidence

This obligation was scaffolded at accept; not elaborated. It is a placeholder for ac-3's static evidence, written by accept's
freeze-moment backstop because no obligation existed for this pair
when spec/coverage-chips was accepted (spec/creation-surfaces#ac-4). Replace this body
with a first-person statement of what that evidence must specifically
show before relying on it — by hand, or via `verdi obligation author
spec/coverage-chips ac-3 static` on a design branch before the replacement itself freezes.
The acceptance criterion's own declared text, for reference:

the feature document stays downward-blind: only stubs enter it, and no coverage count, chip state, or derived summary is ever written into the spec or layout — coverage exists only in the rendered projection


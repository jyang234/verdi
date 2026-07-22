---
id: obligation/cli-creation--ac-4--static
kind: obligation
title: "scaffolded obligation: ac-4 static evidence"
owners: ["johnyang"]
for_kind: static
links:
  - { type: verifies, ref: "spec/cli-creation" }
frozen: { at: 2026-07-22, commit: e1cd2d1f957a200804b97a78829482d1ca8b57f9 }
---
# scaffolded obligation: ac-4 static evidence

This obligation was scaffolded at accept; not elaborated. It is a placeholder for ac-4's static evidence, written by accept's
freeze-moment backstop because no obligation existed for this pair
when spec/cli-creation was accepted (spec/creation-surfaces#ac-4). Replace this body
with a first-person statement of what that evidence must specifically
show before relying on it — by hand, or via `verdi obligation author
spec/cli-creation ac-4 static` on a design branch before the replacement itself freezes.
The acceptance criterion's own declared text, for reference:

--owners deliberately stays out of design start's flag surface — the same posture I-10/X-4 already ratified (05 §CLI: no magic, no tracker-derived naming, and no CLI-supplied owner override either), disclosed here rather than silently reconsidered now that the verb grows other creation flags: the usage text and the verb's whole flag-parsing source carry no --owners token anywhere


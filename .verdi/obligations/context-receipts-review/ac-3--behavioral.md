---
id: obligation/context-receipts-review--ac-3--behavioral
kind: obligation
title: "scaffolded obligation: ac-3 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/context-receipts-review" }
frozen: { at: 2026-08-25, commit: 87e2827eadf4ae6e038858522d9d05b5a335d878 }
---
# scaffolded obligation: ac-3 behavioral evidence

This obligation was scaffolded by `verdi obligation scaffold`; not elaborated. It is a placeholder for ac-3's behavioral evidence, written by `verdi
obligation scaffold` because no obligation existed for this pair yet
(spec/creation-surfaces#ac-4). Replace this body with a first-person
statement of what that evidence must specifically show before relying
on it — by hand, or via `verdi obligation author spec/context-receipts-review ac-3 behavioral` on this
same design branch, before this pull request merges.
The acceptance criterion's own declared text, for reference:

R0 and R2 each run through the sealed execution service with a newly compiled minimum review packet containing the accepted spec, exact current diff, evidence bundle, builder receipt, and review policy but excluding builder conversation, personal or global memory, unrelated context, and prior reviewer conversation

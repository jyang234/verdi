---
id: obligation/context-receipts-review--ac-2--static
kind: obligation
title: "scaffolded obligation: ac-2 static evidence"
owners: ["johnyang"]
for_kind: static
links:
  - { type: verifies, ref: "spec/context-receipts-review" }
frozen: { at: 2026-08-25, commit: 87e2827eadf4ae6e038858522d9d05b5a335d878 }
---
# scaffolded obligation: ac-2 static evidence

This obligation was scaffolded by `verdi obligation scaffold`; not elaborated. It is a placeholder for ac-2's static evidence, written by `verdi
obligation scaffold` because no obligation existed for this pair yet
(spec/creation-surfaces#ac-4). Replace this body with a first-person
statement of what that evidence must specifically show before relying
on it — by hand, or via `verdi obligation author spec/context-receipts-review ac-2 static` on this
same design branch, before this pull request merges.
The acceptance criterion's own declared text, for reference:

verdi context receipt verify recomputes every available digest and continuity fact, authenticates the configured trusted managed-runner principal, rejects stale, incomplete, wrong-tree, unsigned, untrusted, discontinuous, or malformed receipts, and renders unsigned local receipts only as visibly advisory

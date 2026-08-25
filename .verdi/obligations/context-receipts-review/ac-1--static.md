---
id: obligation/context-receipts-review--ac-1--static
kind: obligation
title: "scaffolded obligation: ac-1 static evidence"
owners: ["johnyang"]
for_kind: static
links:
  - { type: verifies, ref: "spec/context-receipts-review" }
frozen: { at: 2026-08-25, commit: 87e2827eadf4ae6e038858522d9d05b5a335d878 }
---
# scaffolded obligation: ac-1 static evidence

This obligation was scaffolded by `verdi obligation scaffold`; not elaborated. It is a placeholder for ac-1's static evidence, written by `verdi
obligation scaffold` because no obligation existed for this pair yet
(spec/creation-surfaces#ac-4). Replace this body with a first-person
statement of what that evidence must specifically show before relying
on it — by hand, or via `verdi obligation author spec/context-receipts-review ac-1 static` on this
same design branch, before this pull request merges.
The acceptance criterion's own declared text, for reference:

verdi.context-receipt/v1 strict-decodes and canonically binds receipt role and authority, manifest and dispatch digests, ATC runway and execution-workspace identities, input and output commits and trees, worktree cleanliness, the ordered revision-segment execution-event-chain root through the acknowledged execution-result plus execution-terminal manifest/source/VATC-global sequences, expansion ledger, obligations, evidence commands and results, runner principal resolution, adapter identity, review inputs, review-of link, and its own digest; its subsequently acknowledged receipt event is outside that self-digest boundary

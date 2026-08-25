---
id: obligation/sealed-claude-vatc-events--ac-2--behavioral
kind: obligation
title: "scaffolded obligation: ac-2 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/sealed-claude-vatc-events" }
frozen: { at: 2026-08-25, commit: 87e2827eadf4ae6e038858522d9d05b5a335d878 }
---
# scaffolded obligation: ac-2 behavioral evidence

This obligation was scaffolded by `verdi obligation scaffold`; not elaborated. It is a placeholder for ac-2's behavioral evidence, written by `verdi
obligation scaffold` because no obligation existed for this pair yet
(spec/creation-surfaces#ac-4). Replace this body with a first-person
statement of what that evidence must specifically show before relying
on it — by hand, or via `verdi obligation author spec/sealed-claude-vatc-events ac-2 behavioral` on this
same design branch, before this pull request merges.
The acceptance criterion's own declared text, for reference:

the U4-owned verdi.context-event/v1 envelope binds source sequence, flight, lane, epoch, manifest revision and digest, session, ATC runway, execution-workspace identity, candidate commit and tree, adapter and version, declared occurrence stamp, closed payload discriminator and body, prior-event digest, optional prior-revision bridge, event digest, and VATC acknowledgment into an idempotent revision-local chain and one complete cross-revision execution chain

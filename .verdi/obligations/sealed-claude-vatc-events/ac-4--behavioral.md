---
id: obligation/sealed-claude-vatc-events--ac-4--behavioral
kind: obligation
title: "scaffolded obligation: ac-4 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/sealed-claude-vatc-events" }
frozen: { at: 2026-08-25, commit: 87e2827eadf4ae6e038858522d9d05b5a335d878 }
---
# scaffolded obligation: ac-4 behavioral evidence

This obligation was scaffolded by `verdi obligation scaffold`; not elaborated. It is a placeholder for ac-4's behavioral evidence, written by `verdi
obligation scaffold` because no obligation existed for this pair yet
(spec/creation-surfaces#ac-4). Replace this body with a first-person
statement of what that evidence must specifically show before relying
on it — by hand, or via `verdi obligation author spec/sealed-claude-vatc-events ac-4 behavioral` on this
same design branch, before this pull request merges.
The acceptance criterion's own declared text, for reference:

secret-bearing values are rejected or redacted before emission, each event kind strict-decodes through its fixed verdi.context-event-payload/<kind>/v1 body, large variable detail uses the closed redacted inline-or-segment union, replay is idempotent, and duplicate conflict, sequence or revision-bridge gap, stale identity, invalid kind or payload, failed redaction, sink discontinuity, or unavailable durable append prevents an authoritative result and receipt

---
id: obligation/sealed-codex-execution--ac-2--behavioral
kind: obligation
title: "scaffolded obligation: ac-2 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/sealed-codex-execution" }
frozen: { at: 2026-08-25, commit: 87e2827eadf4ae6e038858522d9d05b5a335d878 }
---
# scaffolded obligation: ac-2 behavioral evidence

This obligation was scaffolded by `verdi obligation scaffold`; not elaborated. It is a placeholder for ac-2's behavioral evidence, written by `verdi
obligation scaffold` because no obligation existed for this pair yet
(spec/creation-surfaces#ac-4). Replace this body with a first-person
statement of what that evidence must specifically show before relying
on it — by hand, or via `verdi obligation author spec/sealed-codex-execution ac-2 behavioral` on this
same design branch, before this pull request merges.
The acceptance criterion's own declared text, for reference:

the flight-scoped context MCP server exposes only get_flight_plan and request_context; an approved in-scope request emits a digest-bound child manifest and expansion event, an out-of-scope read is denied and recorded, and any authority, capability, profile, or declared-scope change invalidates the epoch rather than expanding it

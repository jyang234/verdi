---
id: obligation/vatc-machine-projections--ac-3--behavioral
kind: obligation
title: "scaffolded obligation: ac-3 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/vatc-machine-projections" }
frozen: { at: 2026-08-25, commit: 87e2827eadf4ae6e038858522d9d05b5a335d878 }
---
# scaffolded obligation: ac-3 behavioral evidence

This obligation was scaffolded by `verdi obligation scaffold`; not elaborated. It is a placeholder for ac-3's behavioral evidence, written by `verdi
obligation scaffold` because no obligation existed for this pair yet
(spec/creation-surfaces#ac-4). Replace this body with a first-person
statement of what that evidence must specifically show before relying
on it — by hand, or via `verdi obligation author spec/vatc-machine-projections ac-3 behavioral` on this
same design branch, before this pull request merges.
The acceptance criterion's own declared text, for reference:

malformed flags and unresolvable inputs exit 2, a successfully produced report exits 0 even when it reports a violated or blocked state, and built-binary tests prove deterministic CLI and MCP parity

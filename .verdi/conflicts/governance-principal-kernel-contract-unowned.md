---
id: conflict/governance-principal-kernel-contract-unowned
kind: conflict
title: "The shared governance-principal kernel contract is delimited by neither spec that requires it"
status: superseded
owners: [platform-team]
links:
  - { type: challenges, ref: spec/guided-lifecycle-governance }
  - { type: challenges, ref: spec/context-integrity }
---
# Conflict: the shared kernel contract both specs require is delimited by neither

## What is disputed

`spec/guided-lifecycle-governance` and `spec/context-integrity` each
require one shared governance-profile and authenticated-principal kernel
("one schema and one implementation seam") and each forbids parallel
profile or actor schemas — yet neither spec delimits the kernel's owned
surfaces, custody, or prohibited duplicates, and the profile schema,
trust-source lists, and policy-inheritance semantics are stated
independently in both texts. Yesterday's accepted truth — two specs each
carrying its own statement of the shared contract — is contested as
incomplete and drift-capable: the texts forbid exactly the parallel
definition they jointly constitute.

## Witness

The owner-merged cross-feature authority audit
(`docs/superpowers/plans/2026-08-03-cross-feature-authority-audit.md`,
landed via PR #264) records the contested state with file-and-line
witnesses:

- CX-1 (DUPLICATE): the governance-profile schema defined in GLG AC-3 and
  CI AC-1/dc-19, with both texts forbidding parallel schemas.
- CX-2 (GAP): no committed custody rule for the kernel contract between
  its delivery and GLG `lifecycle-governance`.
- CX-3 (DUPLICATE): the trust-source and non-authoritative-input lists
  stated twice (GLG AC-3/dc-7; CI AC-1/DC-17).
- CX-4 (GAP): no committed rule separating authoritative principal
  resolution from advisory attribution.
- CX-5 (DUPLICATE): three feature-local narrow-only policy-inheritance
  mechanisms.

The owner resolved the dispute on 2026-08-03: rulings OD-1 through OD-5
in the owner-merged adjudication record
(`docs/superpowers/specs/2026-08-03-four-feature-owner-adjudications-design.md`,
landed via PR #268), whose OD-2 names this joint amendment — audit
recommendation R-5 — as the vehicle.

## Resolution

Feature supersession (03 §The amendment ladder rung 4, both features):
`spec/guided-lifecycle-governance-v2` and `spec/context-integrity-v2`
supersede the challenged specs in one owner-merged pull request carrying
the kernel-contract delimitation the rulings require. Cascade fold: zero
affected stories — no in-flight or closed story declares edges into
either feature — so the single-owner acceptance price applies.

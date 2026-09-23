---
id: conflict/context-integrity-unsealed-work-cannot-close
kind: conflict
title: "Context Integrity v2 leaves work built outside sealed execution no governed way to close"
status: open
owners: [platform-team]
links:
  - { type: challenges, ref: spec/context-integrity-v2 }
---
# Conflict: work built outside sealed execution has no governed way to close

## What is disputed

`spec/context-integrity-v2` requires authoritative builder and fresh
independent-review evidence to close only through authenticated receipts (AC-5,
DC-13), and the compiler's review capsule marks the builder receipt, evidence
bundle, and result diff `unproven` because no v1 port can resolve them; the
policy-conflict gate makes all three codes block the closure verdict. DC-3 says
every departure requires a bounded governed exemption, but the exemption artifact
DC-24 makes exclusive can name only policy-claim departures, not these required
inputs. So once a repository adopts a constitution — which the close countersign
needs, because its governance profile lives there — no story built before a sealed
review path existed can close at all, however complete its evidence, and the spec
offers no governed exception for it. Yesterday's accepted truth (the rule without
an exception path) is contested as incomplete for already-built work, not as wrong
for sealed work.

## Witness

1. `internal/contextcompile/capsule.go` `reviewRequiredInputs` emits the three
   inputs as `unproven` on every review-phase compile ("v1 has no port that could
   resolve them").
2. `internal/policyconflict/report.go` `blockingCompilerDisclosure` makes
   `review-builder-receipt-unproven`, `review-evidence-bundle-unproven`, and
   `review-result-diff-unproven` block the verdict.
3. The closing-machinery wave-1 CF-1 investigation reproduced the consequence
   (`verdi close` on an adopted store exits blocked-unproven), and the controller
   verified the load-bearing claims in code; the wave report
   `docs/superpowers/reports/2026-09-22-closing-machinery-wave-1.md` records it.
4. `internal/policyartifact/exemption.go`: an exemption's witnesses are
   `(policy, claim, claim_digest)` only.

## Resolution

The owner chose (decision D-OC-1, 2026-09-23) a governed exception, not a second
way to close: "we want to make very sure this is the exception and not the rule.
This is not a carte blanche to break our rules." `spec/context-integrity-v3`
carries every v2 item unchanged and adds DC-25 (the unsealed-provenance exemption,
under `docs/superpowers/specs/2026-09-23-unsealed-provenance-exemption-design.md`)
and CO-7 (the permanent unproven label). Plan:
`docs/superpowers/plans/2026-09-23-unsealed-provenance-exemption.md`.

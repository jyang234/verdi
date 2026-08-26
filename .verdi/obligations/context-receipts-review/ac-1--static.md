---
id: obligation/context-receipts-review--ac-1--static
kind: obligation
title: "The context receipt is one closed acyclic authority record"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "verdi.context-receipt/v1 strictly and canonically binds role, authority, manifest and dispatch, runway and execution workspace, repository inputs and outputs, cleanliness, complete revision segments through execution-result, expansion ledger, obligations, evidence, runner, adapter, review linkage, and self-digest while excluding its later receipt-event acknowledgment."
  falsifier: "The schema accepts an unknown, duplicate, null, unordered, or missing operand, permits the receipt event to enter its own digest boundary, loses a terminal sequence or event-chain bridge, or canonical re-encoding changes the digest."
  scope: "Receipt and revision-segment types, strict decoder, canonical encoder, ordering rules, self-digest boundary, builder and reviewer arms, and receipt-event acknowledgment separation."
  producer: { kind: test, ref: "go-test:internal/contextreceipt:TestContextReceiptContract_Static" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/contextreceipt:TestContextReceiptContract_Static in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/context-receipts-review" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# The context receipt is one closed acyclic authority record

CI job `verify` must record producer `go-test:internal/contextreceipt:TestContextReceiptContract_Static` at the exact candidate commit.

The evidence must prove: verdi.context-receipt/v1 strictly and canonically binds role, authority, manifest and dispatch, runway and execution workspace, repository inputs and outputs, cleanliness, complete revision segments through execution-result, expansion ledger, obligations, evidence, runner, adapter, review linkage, and self-digest while excluding its later receipt-event acknowledgment.

It is falsified when: The schema accepts an unknown, duplicate, null, unordered, or missing operand, permits the receipt event to enter its own digest boundary, loses a terminal sequence or event-chain bridge, or canonical re-encoding changes the digest.

Scope: Receipt and revision-segment types, strict decoder, canonical encoder, ordering rules, self-digest boundary, builder and reviewer arms, and receipt-event acknowledgment separation.

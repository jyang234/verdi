---
id: obligation/context-receipts-review--ac-2--behavioral
kind: obligation
title: "Receipt verification rejects every stale or unauthenticated claim"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The verifier recomputes all available digests and continuity facts, authenticates trusted managed-runner evidence through the governance principal resolver, proves exact repository and event operands, and renders unsigned local receipts only as advisory."
  falsifier: "A stale manifest, wrong tree, unsigned authoritative claim, untrusted runner, discontinuous chain, incomplete expansion ledger, malformed receipt, missing event, opaque isolation fact, or telemetry gap returns a proven authoritative verdict."
  scope: "Hermetic positive and negative verification tables plus built-binary context receipt verify input, output, exit, and no-network behavior."
  producer: { kind: test, ref: "go-test:internal/contextreceipt:TestContextReceiptVerifyContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/contextreceipt:TestContextReceiptVerifyContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/context-receipts-review" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Receipt verification rejects every stale or unauthenticated claim

CI job `verify` must record producer `go-test:internal/contextreceipt:TestContextReceiptVerifyContract_Behavioral` at the exact candidate commit.

The evidence must prove: The verifier recomputes all available digests and continuity facts, authenticates trusted managed-runner evidence through the governance principal resolver, proves exact repository and event operands, and renders unsigned local receipts only as advisory.

It is falsified when: A stale manifest, wrong tree, unsigned authoritative claim, untrusted runner, discontinuous chain, incomplete expansion ledger, malformed receipt, missing event, opaque isolation fact, or telemetry gap returns a proven authoritative verdict.

Scope: Hermetic positive and negative verification tables plus built-binary context receipt verify input, output, exit, and no-network behavior.

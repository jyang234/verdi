---
id: obligation/context-receipts-review--ac-4--behavioral
kind: obligation
title: "R0 and R2 are distinct receipt-bound review executions"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "R0 and R2 bind the configured reviewer lane, adapter, model, version, and isolated profile while using distinct sessions, manifests, dispatches, event roots, packets, and receipts, and neither inherits unrecorded builder or R0 context."
  falsifier: "A session, packet, dispatch, event root, or receipt is reused; configured reviewer identity drifts; R2 lacks adjudication or current evidence; or freshness depends on an orchestrator assertion rather than bound bytes."
  scope: "Two sequential hermetic sealed review executions using the same configured reviewer identity with distinct runtime identities, receipt chain, packet inventories, and negative context-inheritance fixtures."
  producer: { kind: test, ref: "go-test:internal/sealedreview:TestSealedReviewFreshnessContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/sealedreview:TestSealedReviewFreshnessContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/context-receipts-review" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# R0 and R2 are distinct receipt-bound review executions

CI job `verify` must record producer `go-test:internal/sealedreview:TestSealedReviewFreshnessContract_Behavioral` at the exact candidate commit.

The evidence must prove: R0 and R2 bind the configured reviewer lane, adapter, model, version, and isolated profile while using distinct sessions, manifests, dispatches, event roots, packets, and receipts, and neither inherits unrecorded builder or R0 context.

It is falsified when: A session, packet, dispatch, event root, or receipt is reused; configured reviewer identity drifts; R2 lacks adjudication or current evidence; or freshness depends on an orchestrator assertion rather than bound bytes.

Scope: Two sequential hermetic sealed review executions using the same configured reviewer identity with distinct runtime identities, receipt chain, packet inventories, and negative context-inheritance fixtures.

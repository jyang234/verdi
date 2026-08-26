---
id: obligation/context-receipts-review--ac-3--behavioral
kind: obligation
title: "Each review receives a newly compiled minimum packet"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "R0 and R2 each run through sealed execution with a new packet containing only the accepted spec, exact current diff, evidence bundle, builder receipt, and review policy, with R2 additionally containing accepted adjudication and current candidate evidence."
  falsifier: "Either packet includes builder conversation, personal or global memory, unrelated context, prior reviewer conversation, stale diff or evidence, omits a required item, or starts outside sealed execution."
  scope: "Fresh R0 and R2 manifest compilation, packet inventory and digests, excluded and opaque rows, builder receipt binding, adjudication inclusion, and sealed review launch fixtures."
  producer: { kind: test, ref: "go-test:internal/sealedreview:TestSealedReviewPacketContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/sealedreview:TestSealedReviewPacketContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/context-receipts-review" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Each review receives a newly compiled minimum packet

CI job `verify` must record producer `go-test:internal/sealedreview:TestSealedReviewPacketContract_Behavioral` at the exact candidate commit.

The evidence must prove: R0 and R2 each run through sealed execution with a new packet containing only the accepted spec, exact current diff, evidence bundle, builder receipt, and review policy, with R2 additionally containing accepted adjudication and current candidate evidence.

It is falsified when: Either packet includes builder conversation, personal or global memory, unrelated context, prior reviewer conversation, stale diff or evidence, omits a required item, or starts outside sealed execution.

Scope: Fresh R0 and R2 manifest compilation, packet inventory and digests, excluded and opaque rows, builder receipt binding, adjudication inclusion, and sealed review launch fixtures.

---
id: obligation/sealed-claude-vatc-events--ac-2--behavioral
kind: obligation
title: "VATC ingestion is idempotent across revisions and executions"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Hermetic ingestion accepts exact retries idempotently, rejects conflicting duplicates and stale identities, allocates a never-resetting global sequence only after durable append, and proves every child-manifest bridge and final execution-result boundary in the complete execution chain."
  falsifier: "A duplicate changes committed bytes or order, a gap or stale revision is accepted, global order resets, an unacknowledged event contributes authority, a final chain truncates, or receipt acknowledgment enters the receipt root."
  scope: "Multi-flight, multi-lane, multi-epoch, multi-session, multi-adapter, and multi-revision event fixtures with retries, conflicts, gaps, bridges, durable acknowledgments, and final receipt events."
  producer: { kind: test, ref: "go-test:internal/contextevent:TestContextEventEnvelopeContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/contextevent:TestContextEventEnvelopeContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-claude-vatc-events" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# VATC ingestion is idempotent across revisions and executions

CI job `verify` must record producer `go-test:internal/contextevent:TestContextEventEnvelopeContract_Behavioral` at the exact candidate commit.

The evidence must prove: Hermetic ingestion accepts exact retries idempotently, rejects conflicting duplicates and stale identities, allocates a never-resetting global sequence only after durable append, and proves every child-manifest bridge and final execution-result boundary in the complete execution chain.

It is falsified when: A duplicate changes committed bytes or order, a gap or stale revision is accepted, global order resets, an unacknowledged event contributes authority, a final chain truncates, or receipt acknowledgment enters the receipt root.

Scope: Multi-flight, multi-lane, multi-epoch, multi-session, multi-adapter, and multi-revision event fixtures with retries, conflicts, gaps, bridges, durable acknowledgments, and final receipt events.

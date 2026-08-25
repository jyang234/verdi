---
id: obligation/sealed-claude-vatc-events--ac-3--behavioral
kind: obligation
title: "Every observable event kind has positive and negative parity fixtures"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Table-driven canned Codex and Claude streams emit and strict-decode every ratified event kind with its exact payload while provider summaries remain operator telemetry and unavailable observable telemetry emits telemetry-gap."
  falsifier: "Any event kind lacks a positive fixture or unknown-field negative fixture, adapters normalize the same activity differently, hidden reasoning is treated as observable or evidentiary, or unavailable telemetry is silent."
  scope: "All ratified event kinds and payload schemas across both adapters, including messages, tools, repository and forge activity, gates, deviations, adjudication, lifecycle, execution result, receipt, retry, resume, suspension, and telemetry gaps."
  producer: { kind: test, ref: "go-test:internal/contextevent:TestContextEventRegistryContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/contextevent:TestContextEventRegistryContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-claude-vatc-events" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Every observable event kind has positive and negative parity fixtures

CI job `verify` must record producer `go-test:internal/contextevent:TestContextEventRegistryContract_Behavioral` at the exact candidate commit.

The evidence must prove: Table-driven canned Codex and Claude streams emit and strict-decode every ratified event kind with its exact payload while provider summaries remain operator telemetry and unavailable observable telemetry emits telemetry-gap.

It is falsified when: Any event kind lacks a positive fixture or unknown-field negative fixture, adapters normalize the same activity differently, hidden reasoning is treated as observable or evidentiary, or unavailable telemetry is silent.

Scope: All ratified event kinds and payload schemas across both adapters, including messages, tools, repository and forge activity, gates, deviations, adjudication, lifecycle, execution result, receipt, retry, resume, suspension, and telemetry gaps.

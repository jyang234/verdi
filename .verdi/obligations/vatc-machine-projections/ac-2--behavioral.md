---
id: obligation/vatc-machine-projections--ac-2--behavioral
kind: obligation
title: "Explicit journey JSON is byte-identical to the legacy mode"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "For every supported story and feature fixture, verdi journey --json emits bytes identical to the no-flag invocation through the existing journey projector and remains read-only."
  falsifier: "The explicit and legacy invocations differ, use different projectors, mutate the store, or change canonical ordering or newline behavior."
  scope: "Built-binary journey invocations for story and feature targets, including a report containing blockers."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestJourneyJSONContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestJourneyJSONContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-machine-projections" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Explicit journey JSON is byte-identical to the legacy mode

CI job `verify` must record producer `go-test:cmd/verdi:TestJourneyJSONContract_Behavioral` at the exact candidate commit.

The evidence must prove: For every supported story and feature fixture, verdi journey --json emits bytes identical to the no-flag invocation through the existing journey projector and remains read-only.

It is falsified when: The explicit and legacy invocations differ, use different projectors, mutate the store, or change canonical ordering or newline behavior.

Scope: Built-binary journey invocations for story and feature targets, including a report containing blockers.

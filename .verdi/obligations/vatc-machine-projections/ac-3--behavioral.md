---
id: obligation/vatc-machine-projections--ac-3--behavioral
kind: obligation
title: "Machine projection exits and failures remain fail-closed"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Matrix and journey machine modes exit 0 after successfully producing a deterministic report even when it is violated or blocked, and exit 2 for malformed flags, missing references, or operational resolution failures."
  falsifier: "A reported adverse state changes success to exit 1, malformed or duplicate flags are accepted, an operational failure exits 0 or 1, or built-binary output is nondeterministic."
  scope: "Built-binary matrix and journey success, adverse-report, malformed-argument, missing-target, and operational-failure cases plus MCP parity."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestMachineProjectionFailureContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestMachineProjectionFailureContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-machine-projections" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Machine projection exits and failures remain fail-closed

CI job `verify` must record producer `go-test:cmd/verdi:TestMachineProjectionFailureContract_Behavioral` at the exact candidate commit.

The evidence must prove: Matrix and journey machine modes exit 0 after successfully producing a deterministic report even when it is violated or blocked, and exit 2 for malformed flags, missing references, or operational resolution failures.

It is falsified when: A reported adverse state changes success to exit 1, malformed or duplicate flags are accepted, an operational failure exits 0 or 1, or built-binary output is nondeterministic.

Scope: Built-binary matrix and journey success, adverse-report, malformed-argument, missing-target, and operational-failure cases plus MCP parity.

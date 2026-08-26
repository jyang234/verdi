---
id: obligation/vatc-forge-countersign--ac-3--behavioral
kind: obligation
title: "Gate and closure share one read-only countersign resolver"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Build gate and story or feature close preflight consume the same countersign resolver, preserve approval and principal witnesses in canonical journey or closure evidence, and never write a countersign artifact or request approval."
  falsifier: "A lifecycle consumer implements separate approval logic, accepts an unavailable or adverse forge fact, drops the approval or principal witness, mutates the candidate tree, or contacts an unconfigured forge as if approval were proven."
  scope: "Built-binary gate, close --preflight, close --prepare, journey, and closure publication paths over hermetic forge fakes and candidate trees."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestCountersignLifecycleContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestCountersignLifecycleContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-forge-countersign" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Gate and closure share one read-only countersign resolver

CI job `verify` must record producer `go-test:cmd/verdi:TestCountersignLifecycleContract_Behavioral` at the exact candidate commit.

The evidence must prove: Build gate and story or feature close preflight consume the same countersign resolver, preserve approval and principal witnesses in canonical journey or closure evidence, and never write a countersign artifact or request approval.

It is falsified when: A lifecycle consumer implements separate approval logic, accepts an unavailable or adverse forge fact, drops the approval or principal witness, mutates the candidate tree, or contacts an unconfigured forge as if approval were proven.

Scope: Built-binary gate, close --preflight, close --prepare, journey, and closure publication paths over hermetic forge fakes and candidate trees.

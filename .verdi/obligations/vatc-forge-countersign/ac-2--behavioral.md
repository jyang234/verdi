---
id: obligation/vatc-forge-countersign--ac-2--behavioral
kind: obligation
title: "Countersign reduction proves roles, freshness, count, and separation"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The resolver proves a countersign only from active exact-candidate fresh role-authorized approvals by authenticated distinct principals that meet required count and separation policy, preserving every rejected or unproven operand as a witness."
  falsifier: "A stale, wrong-head, revoked, duplicate-principal, unauthorized-role, self-approved, insufficient-count, future-stamped, or unproven approval contributes to a proven verdict, or eligible ordering varies."
  scope: "Hermetic resolver tables for story-review and feature-UAT obligations, multi-approval counts, policy freshness, principal normalization, and separation of duties."
  producer: { kind: test, ref: "go-test:internal/countersign:TestCountersignWitnessContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/countersign:TestCountersignWitnessContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-forge-countersign" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Countersign reduction proves roles, freshness, count, and separation

CI job `verify` must record producer `go-test:internal/countersign:TestCountersignWitnessContract_Behavioral` at the exact candidate commit.

The evidence must prove: The resolver proves a countersign only from active exact-candidate fresh role-authorized approvals by authenticated distinct principals that meet required count and separation policy, preserving every rejected or unproven operand as a witness.

It is falsified when: A stale, wrong-head, revoked, duplicate-principal, unauthorized-role, self-approved, insufficient-count, future-stamped, or unproven approval contributes to a proven verdict, or eligible ordering varies.

Scope: Hermetic resolver tables for story-review and feature-UAT obligations, multi-approval counts, policy freshness, principal normalization, and separation of duties.

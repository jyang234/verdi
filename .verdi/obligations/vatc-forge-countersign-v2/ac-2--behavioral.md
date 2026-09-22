---
id: obligation/vatc-forge-countersign-v2--ac-2--behavioral
kind: obligation
title: "Countersign reduction proves roles, freshness, count, and separation"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The resolver proves a countersign only from active exact-candidate fresh role-authorized approvals by authenticated distinct principals that meet required count and separation policy, preserving every rejected or unproven operand as a witness."
  falsifier: "A stale, wrong-head, revoked, duplicate-principal, unauthorized-role, self-approved under a profile that requires separation of duties, insufficient-count, future-stamped, or unproven approval contributes to a proven verdict, or eligible ordering varies."
  scope: "Hermetic resolver tables for story-review and feature-UAT obligations, multi-approval counts, policy freshness, principal normalization, and separation of duties."
  producer: { kind: test, ref: "go-test:internal/countersign:TestCountersignWitnessContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/countersign:TestCountersignWitnessContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-forge-countersign-v2" }
frozen: { at: 2026-09-22, commit: 293e3b220fb85937601fae0b2aa49cd9be05ae11 }
---
# Countersign reduction proves roles, freshness, count, and separation

CI job `verify` must record producer `go-test:internal/countersign:TestCountersignWitnessContract_Behavioral` at the exact candidate commit.

The evidence must prove: The resolver proves a countersign only from active exact-candidate fresh role-authorized approvals by authenticated distinct principals that meet required count and separation policy, preserving every rejected or unproven operand as a witness.

It is falsified when: A stale, wrong-head, revoked, duplicate-principal, unauthorized-role, self-approved under a profile that requires separation of duties, insufficient-count, future-stamped, or unproven approval contributes to a proven verdict, or eligible ordering varies.

Scope: Hermetic resolver tables for story-review and feature-UAT obligations, multi-approval counts, policy freshness, principal normalization, and separation of duties.

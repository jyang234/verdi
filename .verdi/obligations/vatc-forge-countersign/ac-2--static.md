---
id: obligation/vatc-forge-countersign--ac-2--static
kind: obligation
title: "The countersign witness preserves every reduction operand"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "verdi.countersign-witness/v1 is a strict canonical schema containing the complete ordered approval set, obligation and freshness policies, deterministic principal reduction, separation and freshness verdicts, witnesses, and self-digest."
  falsifier: "The schema omits a required operand, accepts an unknown or duplicate field, permits null collections, loses deterministic order, counts without approval witnesses, or computes a digest over noncanonical bytes."
  scope: "The countersign witness types, strict codec, canonical encoder, digest calculation, approval ordering, and closed verdict vocabulary."
  producer: { kind: test, ref: "go-test:internal/countersign:TestCountersignWitnessContract_Static" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/countersign:TestCountersignWitnessContract_Static in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-forge-countersign" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# The countersign witness preserves every reduction operand

CI job `verify` must record producer `go-test:internal/countersign:TestCountersignWitnessContract_Static` at the exact candidate commit.

The evidence must prove: verdi.countersign-witness/v1 is a strict canonical schema containing the complete ordered approval set, obligation and freshness policies, deterministic principal reduction, separation and freshness verdicts, witnesses, and self-digest.

It is falsified when: The schema omits a required operand, accepts an unknown or duplicate field, permits null collections, loses deterministic order, counts without approval witnesses, or computes a digest over noncanonical bytes.

Scope: The countersign witness types, strict codec, canonical encoder, digest calculation, approval ordering, and closed verdict vocabulary.

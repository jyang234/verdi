---
id: obligation/vatc-build-worktree-contract--ac-3--behavioral
kind: obligation
title: "Runway base mismatches and Git failures refuse without fallback"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "An existing feature branch at the wrong base and an operational Git failure produce deterministic refusals without selecting another checkout or partially mutating the runway or primary checkout."
  falsifier: "Either adverse fixture falls back to another branch or checkout, partially mutates Git state, weakens an existing lifecycle gate, or produces a nondeterministic witness or exit class."
  scope: "Fixture Git wrong-base and injected operational-failure cases with complete before-and-after snapshots of both primary and runway repositories."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestBuildCommandsFromATCRunway_Refusals" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestBuildCommandsFromATCRunway_Refusals in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-build-worktree-contract" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Runway base mismatches and Git failures refuse without fallback

CI job `verify` must record producer `go-test:cmd/verdi:TestBuildCommandsFromATCRunway_Refusals` at the exact candidate commit.

The evidence must prove: An existing feature branch at the wrong base and an operational Git failure produce deterministic refusals without selecting another checkout or partially mutating the runway or primary checkout.

It is falsified when: Either adverse fixture falls back to another branch or checkout, partially mutates Git state, weakens an existing lifecycle gate, or produces a nondeterministic witness or exit class.

Scope: Fixture Git wrong-base and injected operational-failure cases with complete before-and-after snapshots of both primary and runway repositories.

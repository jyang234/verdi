---
id: obligation/vatc-forge-countersign--ac-1--static
kind: obligation
title: "The forge port exposes strict candidate-bound approval facts"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "The consumer-defined forge approval port carries repository, change, immutable approval identity, exact candidate SHA, closed state and stamps, authenticated principal evidence, and provider freshness witnesses without provider judgments."
  falsifier: "A required identity or freshness operand is absent, a provider-specific type leaks across the port, an unknown state can decode, or display text or caller claims can substitute for authenticated principal evidence."
  scope: "The shared forge approval value and port plus GitHub and GitLab strict decoding boundaries."
  producer: { kind: test, ref: "go-test:internal/forge:TestForgeApprovalContract_Static" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/forge:TestForgeApprovalContract_Static in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-forge-countersign" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# The forge port exposes strict candidate-bound approval facts

CI job `verify` must record producer `go-test:internal/forge:TestForgeApprovalContract_Static` at the exact candidate commit.

The evidence must prove: The consumer-defined forge approval port carries repository, change, immutable approval identity, exact candidate SHA, closed state and stamps, authenticated principal evidence, and provider freshness witnesses without provider judgments.

It is falsified when: A required identity or freshness operand is absent, a provider-specific type leaks across the port, an unknown state can decode, or display text or caller claims can substitute for authenticated principal evidence.

Scope: The shared forge approval value and port plus GitHub and GitLab strict decoding boundaries.

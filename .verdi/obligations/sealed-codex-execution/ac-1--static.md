---
id: obligation/sealed-codex-execution--ac-1--static
kind: obligation
title: "The sealed execution boundary is strict and harness-neutral"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "The sealed execution request is a strict start-or-resume union whose shared service owns authority, runway, execution-workspace, profile, projection, grant, conflict, recorder, adapter, and opaque-boundary operands before provider launch."
  falsifier: "The request admits an unknown or duplicate field, accepts both or neither action arm, lets an adapter bypass a prerequisite, creates a second provider materializer, or mixes deterministic context with runtime dispatch identity."
  scope: "Sealed execution schemas and codecs, consumer-defined service ports, execution-workspace integration, Codex adapter boundary, and canonical result types."
  producer: { kind: test, ref: "go-test:internal/sealedexec:TestContextExecutionContract_Static" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/sealedexec:TestContextExecutionContract_Static in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-codex-execution" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# The sealed execution boundary is strict and harness-neutral

CI job `verify` must record producer `go-test:internal/sealedexec:TestContextExecutionContract_Static` at the exact candidate commit.

The evidence must prove: The sealed execution request is a strict start-or-resume union whose shared service owns authority, runway, execution-workspace, profile, projection, grant, conflict, recorder, adapter, and opaque-boundary operands before provider launch.

It is falsified when: The request admits an unknown or duplicate field, accepts both or neither action arm, lets an adapter bypass a prerequisite, creates a second provider materializer, or mixes deterministic context with runtime dispatch identity.

Scope: Sealed execution schemas and codecs, consumer-defined service ports, execution-workspace integration, Codex adapter boundary, and canonical result types.

---
id: obligation/sealed-claude-vatc-events--ac-1--static
kind: obligation
title: "Claude is a closed adapter to the shared sealed core"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "The claude adapter uses the same sealed service, request and result schemas, scoped context MCP, event sink, receipt verifier, execution-workspace materializer, authority states, and consumer-defined process port as codex, with no Claude-only shortcut."
  falsifier: "Claude introduces a second authority model, context or receipt surface, bypasses profile, grant, recorder, resume, runway, workspace, or receipt checks, imports provider packages, or accepts an unknown adapter value."
  scope: "Claude adapter construction and ports, shared sealedexec schemas and service integration, closed adapter enum, official CLI argument-vector boundary, and forbidden alternate surfaces."
  producer: { kind: test, ref: "go-test:internal/sealedexec/claude:TestClaudeAdapterParityContract_Static" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/sealedexec/claude:TestClaudeAdapterParityContract_Static in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-claude-vatc-events" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Claude is a closed adapter to the shared sealed core

CI job `verify` must record producer `go-test:internal/sealedexec/claude:TestClaudeAdapterParityContract_Static` at the exact candidate commit.

The evidence must prove: The claude adapter uses the same sealed service, request and result schemas, scoped context MCP, event sink, receipt verifier, execution-workspace materializer, authority states, and consumer-defined process port as codex, with no Claude-only shortcut.

It is falsified when: Claude introduces a second authority model, context or receipt surface, bypasses profile, grant, recorder, resume, runway, workspace, or receipt checks, imports provider packages, or accepts an unknown adapter value.

Scope: Claude adapter construction and ports, shared sealedexec schemas and service integration, closed adapter enum, official CLI argument-vector boundary, and forbidden alternate surfaces.

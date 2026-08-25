---
id: obligation/sealed-codex-execution--ac-2--static
kind: obligation
title: "The flight-scoped MCP surface has exactly two tools"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "The scoped context MCP registry exposes exactly get_flight_plan with an empty argument object and request_context with required ref and purpose, and every response is inspection data rather than an instruction or mutation channel."
  falsifier: "A generic read, shell, mutation, receipt-mint, or undeclared tool is registered, an argument schema is open, a response becomes project authority, or the adapter can inject instructions through MCP."
  scope: "Scoped MCP tool registry and strict argument or result codecs plus the separation between MCP inspection data and the sealed instruction projection."
  producer: { kind: test, ref: "go-test:internal/sealedexec:TestScopedContextMCPContract_Static" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/sealedexec:TestScopedContextMCPContract_Static in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-codex-execution" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# The flight-scoped MCP surface has exactly two tools

CI job `verify` must record producer `go-test:internal/sealedexec:TestScopedContextMCPContract_Static` at the exact candidate commit.

The evidence must prove: The scoped context MCP registry exposes exactly get_flight_plan with an empty argument object and request_context with required ref and purpose, and every response is inspection data rather than an instruction or mutation channel.

It is falsified when: A generic read, shell, mutation, receipt-mint, or undeclared tool is registered, an argument schema is open, a response becomes project authority, or the adapter can inject instructions through MCP.

Scope: Scoped MCP tool registry and strict argument or result codecs plus the separation between MCP inspection data and the sealed instruction projection.

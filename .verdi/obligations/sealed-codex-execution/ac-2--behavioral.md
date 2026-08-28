---
id: obligation/sealed-codex-execution--ac-2--behavioral
kind: obligation
title: "Context expansion is an acknowledged manifest transition"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "An approved in-scope request_context call emits a digest-bound child manifest and acknowledged expansion event, an out-of-scope request is denied and recorded, and any authority, capability, profile, or declared-scope change invalidates the epoch."
  falsifier: "A context read occurs without a request and event, a denied request becomes data, a child manifest is unbound or unacknowledged, a changed invariant silently expands context, or another MCP tool is callable."
  scope: "Built-binary context mcp over approved, denied, malformed, recorder-failure, and epoch-invalidation fixtures."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestScopedContextMCPContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestScopedContextMCPContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-codex-execution" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Context expansion is an acknowledged manifest transition

CI job `verify` must record producer `go-test:cmd/verdi:TestScopedContextMCPContract_Behavioral` at the exact candidate commit.

The evidence must prove: An approved in-scope request_context call emits a digest-bound child manifest and acknowledged expansion event, an out-of-scope request is denied and recorded, and any authority, capability, profile, or declared-scope change invalidates the epoch.

It is falsified when: A context read occurs without a request and event, a denied request becomes data, a child manifest is unbound or unacknowledged, a changed invariant silently expands context, or another MCP tool is callable.

Scope: Built-binary context mcp over approved, denied, malformed, recorder-failure, and epoch-invalidation fixtures.

---
id: obligation/vatc-machine-projections--ac-1--behavioral
kind: obligation
title: "Matrix CLI and MCP preserve canonical story and feature facts"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Built-binary matrix JSON and MCP get_matrix return equivalent canonical records for story, feature, preview, empty-collection, and adverse fold fixtures while legacy matrix text consumes the same projection."
  falsifier: "CLI and MCP records differ after protocol-envelope removal, canonical encoding changes across identical runs, a story or feature fact is missing or invented, or legacy text observes different fold semantics."
  scope: "Hermetic built-binary matrix invocations and direct MCP calls over matching story and feature fixture stores."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestMatrixProjectionContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestMatrixProjectionContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-machine-projections" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Matrix CLI and MCP preserve canonical story and feature facts

CI job `verify` must record producer `go-test:cmd/verdi:TestMatrixProjectionContract_Behavioral` at the exact candidate commit.

The evidence must prove: Built-binary matrix JSON and MCP get_matrix return equivalent canonical records for story, feature, preview, empty-collection, and adverse fold fixtures while legacy matrix text consumes the same projection.

It is falsified when: CLI and MCP records differ after protocol-envelope removal, canonical encoding changes across identical runs, a story or feature fact is missing or invented, or legacy text observes different fold semantics.

Scope: Hermetic built-binary matrix invocations and direct MCP calls over matching story and feature fixture stores.

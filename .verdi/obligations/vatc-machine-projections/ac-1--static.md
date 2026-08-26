---
id: obligation/vatc-machine-projections--ac-1--static
kind: obligation
title: "One strict tagged matrix union feeds every adapter"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "The matrix contract is one strict verdi.matrix/v1 class-tagged union assembled by internal/matrixprojection and consumed by CLI text, CLI JSON, and MCP without alternate field assembly."
  falsifier: "A decoder accepts an unknown or cross-arm field, either tagged body is missing or duplicated, a native story or feature fold field or order is lost, or any adapter assembles a second matrix record."
  scope: "The matrix record and codec, story and feature fold mapping, CLI matrix formatters, and the MCP get_matrix binding."
  producer: { kind: test, ref: "go-test:internal/matrixprojection:TestMatrixProjectionContract_Static" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/matrixprojection:TestMatrixProjectionContract_Static in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-machine-projections" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# One strict tagged matrix union feeds every adapter

CI job `verify` must record producer `go-test:internal/matrixprojection:TestMatrixProjectionContract_Static` at the exact candidate commit.

The evidence must prove: The matrix contract is one strict verdi.matrix/v1 class-tagged union assembled by internal/matrixprojection and consumed by CLI text, CLI JSON, and MCP without alternate field assembly.

It is falsified when: A decoder accepts an unknown or cross-arm field, either tagged body is missing or duplicated, a native story or feature fold field or order is lost, or any adapter assembles a second matrix record.

Scope: The matrix record and codec, story and feature fold mapping, CLI matrix formatters, and the MCP get_matrix binding.

---
id: obligation/vatc-build-worktree-contract--ac-2--behavioral
kind: obligation
title: "Every build lifecycle mutation is attributable to the runway"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Built-binary align, disposition, gate, and close --prepare run from the runway and the transcript records each exit class, branch, HEAD, tree, status, and changed path while the primary checkout remains unchanged."
  falsifier: "Any command resolves its mutation root through the primary checkout, changes an undeclared path, omits a transcript boundary, or produces an exit class inconsistent with its result."
  scope: "The complete hermetic runway lifecycle transcript after build start, including all permitted mutations and an immutable primary-checkout comparison."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestBuildCommandsFromATCRunway_Lifecycle" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestBuildCommandsFromATCRunway_Lifecycle in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-build-worktree-contract" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Every build lifecycle mutation is attributable to the runway

CI job `verify` must record producer `go-test:cmd/verdi:TestBuildCommandsFromATCRunway_Lifecycle` at the exact candidate commit.

The evidence must prove: Built-binary align, disposition, gate, and close --prepare run from the runway and the transcript records each exit class, branch, HEAD, tree, status, and changed path while the primary checkout remains unchanged.

It is falsified when: Any command resolves its mutation root through the primary checkout, changes an undeclared path, omits a transcript boundary, or produces an exit class inconsistent with its result.

Scope: The complete hermetic runway lifecycle transcript after build start, including all permitted mutations and an immutable primary-checkout comparison.

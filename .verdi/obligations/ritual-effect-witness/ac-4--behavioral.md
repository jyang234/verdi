---
id: obligation/ritual-effect-witness--ac-4--behavioral
kind: obligation
title: "Branch cuts come from the resolved default branch and check collisions there"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "With HEAD on another branch whose tree differs from the default branch, build start and constitution propose create their branches at the resolved default branch's commit; and build start refuses, with nothing created, a branch name that exists in the tree it cuts from."
  falsifier: "A branch is cut from HEAD, or build start creates a branch whose name collides in the tree it cuts from."
  scope: "build start and constitution propose driven as the built binary over fixturegit repositories with a local bare remote."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestBranchCuts_FromResolvedDefaultBranch" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestBranchCuts_FromResolvedDefaultBranch in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/ritual-effect-witness" }
frozen: { at: 2026-09-30, commit: 4a5da769127ba8da923e667a43dfe243eb5f85da }
---
# Branch cuts come from the resolved default branch and check collisions there

CI job `verify` must record producer `go-test:cmd/verdi:TestBranchCuts_FromResolvedDefaultBranch` at the exact candidate
commit. UAT-021, UAT-033, and UAT-034 keep their existing tests; this test adds the open halves.

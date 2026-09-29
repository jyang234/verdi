---
id: obligation/wall-changes--ac-3--behavioral
kind: obligation
title: "The branch-switch guard ignores the changes summary"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The branch-switch guard refuses a switch whenever git reports the working tree dirty, whatever the changes summary's counts or state."
  falsifier: "A switch proceeds over uncommitted changes because the typed count is zero or the comparison is unreadable."
  scope: "The branch-switch endpoint over fixturegit repositories with zero typed changes and an unreadable comparison, each with a dirty tree."
  producer: { kind: test, ref: "go-test:internal/workbench:TestWallChanges_BranchGuardIndependent" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/workbench:TestWallChanges_BranchGuardIndependent in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-changes" }
frozen: { at: 2026-09-29, commit: f67d6d5a2d9e148a2e3985c597bb272d78580aa6 }
---
# The branch-switch guard ignores the changes summary

CI job `verify` must record producer `go-test:internal/workbench:TestWallChanges_BranchGuardIndependent` at the exact
candidate commit. An integration test driving the branch-switch endpoint and asserting the refusal in each case.

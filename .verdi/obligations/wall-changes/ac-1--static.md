---
id: obligation/wall-changes--ac-1--static
kind: obligation
title: "The changes classifier reports typed, unclassified, and unreadable changes"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "For every pair of HEAD and working-tree inputs, the classifier returns recognized operations as typed changes, every other difference as an unclassified change with its reason, and an unreadable comparison with its reason when either side cannot be read or diffed."
  falsifier: "An input yields a change in neither list, an unrecognized difference is dropped, or an unreadable input reads as no changes."
  scope: "Table-driven cases: a typed-only edit, a prose-only edit, a layout-only edit, another staged path, an untracked file, a mixed tree, an undecodable HEAD spec, an undecodable working-tree spec, and a missing spec."
  producer: { kind: test, ref: "go-test:internal/workbench:TestWallChanges_Classify" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/workbench:TestWallChanges_Classify in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-changes" }
frozen: { at: 2026-09-29, commit: f67d6d5a2d9e148a2e3985c597bb272d78580aa6 }
---
# The changes classifier reports typed, unclassified, and unreadable changes

CI job `verify` must record producer `go-test:internal/workbench:TestWallChanges_Classify` at the exact candidate
commit. A table-driven unit test of the classifier over the cases in scope, happy and negative paths.

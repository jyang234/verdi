---
id: obligation/wall-changes--ac-2--static
kind: obligation
title: "A zero-operation diff never clears the dirty indicator"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "When the semantic diff recognizes no operation but any other change remains, the classifier keeps the tree dirty and lists each remaining change as unclassified."
  falsifier: "Any remaining change with zero recognized operations yields a clean result."
  scope: "Prose-only, layout-only, another-staged-path, and untracked-file trees with zero typed operations."
  producer: { kind: test, ref: "go-test:internal/workbench:TestWallChanges_ZeroOperationsStayDirty" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/workbench:TestWallChanges_ZeroOperationsStayDirty in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-changes" }
frozen: { at: 2026-09-29, commit: 601c07557c19a7a0baf79f901f8b483aed041b96 }
---
# A zero-operation diff never clears the dirty indicator

CI job `verify` must record producer `go-test:internal/workbench:TestWallChanges_ZeroOperationsStayDirty` at the exact
candidate commit. Unit cases over the classifier for each zero-operation tree in scope.

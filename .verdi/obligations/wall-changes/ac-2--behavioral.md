---
id: obligation/wall-changes--ac-2--behavioral
kind: obligation
title: "A served wall with only unclassified changes stays dirty"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "A wall served over a repository whose only changes are unclassified reports dirty in its snapshot."
  falsifier: "The served snapshot reports a clean tree while git reports changes."
  scope: "An httptest-served wall over fixturegit repositories with prose-only and untracked-only changes."
  producer: { kind: test, ref: "go-test:internal/workbench:TestWallChanges_SnapshotStaysDirty" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/workbench:TestWallChanges_SnapshotStaysDirty in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-changes" }
frozen: { at: 2026-09-29, commit: f67d6d5a2d9e148a2e3985c597bb272d78580aa6 }
---
# A served wall with only unclassified changes stays dirty

CI job `verify` must record producer `go-test:internal/workbench:TestWallChanges_SnapshotStaysDirty` at the exact
candidate commit. An integration test serving the wall and asserting the dirty indicator and the unclassified list.

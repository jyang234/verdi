---
id: obligation/wall-changes--ac-1--behavioral
kind: obligation
title: "The wall's page model and JSON carry the three-state changes over a real repository"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Served over a fixturegit repository, the wall's page model and its JSON carry the typed, unclassified, and unreadable changes the working tree actually has."
  falsifier: "The served snapshot omits a change git reports, or reports a readable or unreadable state the repository does not have."
  scope: "An httptest-served wall over fixturegit repositories in each state of the static cases."
  producer: { kind: test, ref: "go-test:internal/workbench:TestWallChanges_Snapshot" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/workbench:TestWallChanges_Snapshot in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-changes" }
frozen: { at: 2026-09-29, commit: 601c07557c19a7a0baf79f901f8b483aed041b96 }
---
# The wall's page model and JSON carry the three-state changes over a real repository

CI job `verify` must record producer `go-test:internal/workbench:TestWallChanges_Snapshot` at the exact candidate
commit. An integration test serving the wall over fixturegit repositories and decoding the snapshot JSON for each state.

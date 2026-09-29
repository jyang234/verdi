---
id: obligation/index-data--ac-1--behavioral
kind: obligation
title: "Last-change dates over a real repository"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Over a fixturegit repository with design branches and default-branch specs at known commit dates, each entry's date equals its tip's or landing commit's committer date."
  falsifier: "Any entry's date differs from the fixture's commit date, or the walk runs more than once."
  scope: "refindex's production git port over a fixturegit repository with pinned commit dates."
  producer: { kind: test, ref: "go-test:internal/refindex:TestComputeIndex_LastChangeDatesFixturegit" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/refindex:TestComputeIndex_LastChangeDatesFixturegit in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-data" }
frozen: { at: 2026-09-29, commit: 18b943da44eb8d366b56a31ae92c88b2a190ac6d }
---
# Last-change dates over a real repository

CI job `verify` must record producer `go-test:internal/refindex:TestComputeIndex_LastChangeDatesFixturegit` at the exact
candidate commit. An integration test over a fixturegit repository with stable SHAs and dates.

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
frozen: { at: 2026-09-29, commit: c8f4aad8cd1d969b578e08f27bc2d042d5bf5568 }
---
# Last-change dates over a real repository

CI job `verify` must record producer `go-test:internal/refindex:TestComputeIndex_LastChangeDatesFixturegit` at the exact
candidate commit. An integration test over a fixturegit repository with stable SHAs and dates.

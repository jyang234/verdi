---
id: obligation/index-data--ac-1--static
kind: obligation
title: "Every index entry carries its last-change date, or a disclosure"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "For every entry the index computes, a design-branch entry carries its tip's committer date, a default-branch entry carries its landing commit's committer date, and an unreadable date is disclosed on the entry."
  falsifier: "An entry has no date and no disclosure, a zero or current date stands in for an unreadable one, or a date comes from a commit other than the tip or the landing commit."
  scope: "refindex over its fake git port: design-branch and default-branch entries, an unreadable tip date, and an unreadable landing commit."
  producer: { kind: test, ref: "go-test:internal/refindex:TestComputeIndex_LastChangeDates" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/refindex:TestComputeIndex_LastChangeDates in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-data" }
frozen: { at: 2026-09-29, commit: be706198be544721484b0d2fed7974477b3c68d7 }
---
# Every index entry carries its last-change date, or a disclosure

CI job `verify` must record producer `go-test:internal/refindex:TestComputeIndex_LastChangeDates` at the exact candidate
commit. A table-driven test of ComputeIndex over the fake port for each case in scope.

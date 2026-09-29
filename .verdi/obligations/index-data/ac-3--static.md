---
id: obligation/index-data--ac-3--static
kind: obligation
title: "Test doubles give every fixture entry a date"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The refindex fake port returns a deterministic date for every fixture branch and landing commit."
  falsifier: "A fixture entry computed through the fake port has no date."
  scope: "refindex's fake port over its fixture entries."
  producer: { kind: test, ref: "go-test:internal/refindex:TestFakePort_DatesEveryEntry" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/refindex:TestFakePort_DatesEveryEntry in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-data" }
frozen: { at: 2026-09-29, commit: c8f4aad8cd1d969b578e08f27bc2d042d5bf5568 }
---
# Test doubles give every fixture entry a date

CI job `verify` must record producer `go-test:internal/refindex:TestFakePort_DatesEveryEntry` at the exact candidate
commit. A test enumerating the fake port's fixture entries and asserting a date for each.

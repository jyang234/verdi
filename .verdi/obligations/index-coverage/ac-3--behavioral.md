---
id: obligation/index-coverage--ac-3--behavioral
kind: obligation
title: "The index serves the disclosures count once per render"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The served index carries the disclosures count, computed once per render."
  falsifier: "The served count differs from the enumeration, or the enumeration runs more than once per render."
  scope: "An httptest-served index over fixture stores with and without disclosures."
  producer: { kind: test, ref: "go-test:internal/workbench:TestIndex_DisclosuresCount" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/workbench:TestIndex_DisclosuresCount in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-coverage" }
frozen: { at: 2026-09-29, commit: 46186349d2eeff711dd26270bd48d3b821ccee93 }
---
# The index serves the disclosures count once per render

CI job `verify` must record producer `go-test:internal/workbench:TestIndex_DisclosuresCount` at the exact candidate
commit. An integration test serving the index and counting enumeration calls.

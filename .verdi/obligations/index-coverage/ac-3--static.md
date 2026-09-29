---
id: obligation/index-coverage--ac-3--static
kind: obligation
title: "The disclosures count equals the disclosures page's enumeration"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "For every fixture store, the count equals the number of entries the disclosures page enumerates."
  falsifier: "The count differs from the enumeration, or is computed by a second enumeration."
  scope: "The committed disclosure fixtures, including a store with none."
  producer: { kind: test, ref: "go-test:internal/disclosureview:TestCount_MatchesEnumeration" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/disclosureview:TestCount_MatchesEnumeration in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-coverage" }
frozen: { at: 2026-09-29, commit: e0fe3b544598b5e01af5627502c1441d59c47759 }
---
# The disclosures count equals the disclosures page's enumeration

CI job `verify` must record producer `go-test:internal/disclosureview:TestCount_MatchesEnumeration` at the exact
candidate commit. A test comparing the count with the enumeration.

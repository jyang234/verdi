---
id: obligation/index-coverage--ac-1--static
kind: obligation
title: "One pure coverage function per accepted feature"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "For every fixture feature, the function returns each criterion's covering stubs and stories exactly as declared stubs and implements edges give them, and reads no other input."
  falsifier: "A criterion's coverage differs from its stubs and implements edges, or the result depends on another input."
  scope: "Table-driven features: uncovered, stub-covered, story-covered through a stub, story-covered on an unstubbed criterion, both, and several stubs and stories per criterion."
  producer: { kind: test, ref: "go-test:internal/featurecoverage:TestCoverage_PerCriterion" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/featurecoverage:TestCoverage_PerCriterion in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-coverage" }
frozen: { at: 2026-09-29, commit: e0fe3b544598b5e01af5627502c1441d59c47759 }
---
# One pure coverage function per accepted feature

CI job `verify` must record producer `go-test:internal/featurecoverage:TestCoverage_PerCriterion` at the exact candidate
commit. A table-driven test of the shared function.

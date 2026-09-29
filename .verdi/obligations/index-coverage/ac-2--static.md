---
id: obligation/index-coverage--ac-2--static
kind: obligation
title: "Unreadable coverage input is disclosed, never zero"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "An undecodable stub declaration or story yields a disclosure on each affected criterion and never a no-coverage result, and no result claims implementation or evidence."
  falsifier: "An unreadable input yields no coverage without a disclosure, or a result claims implementation."
  scope: "Undecodable stub declarations and stories on covered and uncovered criteria."
  producer: { kind: test, ref: "go-test:internal/featurecoverage:TestCoverage_UnreadableInputDisclosed" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/featurecoverage:TestCoverage_UnreadableInputDisclosed in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-coverage" }
frozen: { at: 2026-09-29, commit: af6fbfadb81bcedb4f9959a97f6fca816a46f103 }
---
# Unreadable coverage input is disclosed, never zero

CI job `verify` must record producer `go-test:internal/featurecoverage:TestCoverage_UnreadableInputDisclosed` at the
exact candidate commit. Negative-path cases of the shared function.

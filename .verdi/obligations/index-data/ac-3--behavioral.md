---
id: obligation/index-data--ac-3--behavioral
kind: obligation
title: "The e2e harness serves fixture dates and a settable clock"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The e2e harness provisions stores whose entries carry known dates and serves the index with a clock the harness sets, so an entry's age and quiet mark are fixed."
  falsifier: "A harness-served entry's date or quiet mark depends on the real date."
  scope: "cmd/e2eharness provisioning and its control endpoint or flag for the clock."
  producer: { kind: test, ref: "go-test:cmd/e2eharness:TestHarness_IndexDatesAndClock" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/e2eharness:TestHarness_IndexDatesAndClock in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-data" }
frozen: { at: 2026-09-29, commit: be706198be544721484b0d2fed7974477b3c68d7 }
---
# The e2e harness serves fixture dates and a settable clock

CI job `verify` must record producer `go-test:cmd/e2eharness:TestHarness_IndexDatesAndClock` at the exact candidate
commit. A test of the harness asserting provisioned dates and the served quiet mark under a set clock; the later index
story's Playwright files rely on it.

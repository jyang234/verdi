---
id: obligation/readiness-page--ac-4--static
kind: obligation
title: "The per-request stamp and one derivation"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The server-rendered readiness page carries the per-request derivation stamp, contains no startup-snapshot text, and renders from the same readiness facts value the Readiness tab renders."
  falsifier: "The page carries a startup-snapshot line, lacks the per-request stamp, or computes readiness separately from the tab."
  scope: "Server renders of the readiness page and of the Readiness tab's projection over the same fixture."
  producer: { kind: test, ref: "go-test:internal/workbench:TestReadinessPage_PerRequestStampAndSharedFacts" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/workbench:TestReadinessPage_PerRequestStampAndSharedFacts in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/readiness-page" }
frozen: { at: 2026-09-29, commit: 01c200e50eacb38505ba12850d187be361d1301a }
---
# The per-request stamp and one derivation

CI job `verify` must record producer `go-test:internal/workbench:TestReadinessPage_PerRequestStampAndSharedFacts` at the
exact candidate commit. A render test comparing the page's facts with the tab projection's facts over one fixture.

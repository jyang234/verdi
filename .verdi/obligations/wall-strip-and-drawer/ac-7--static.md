---
id: obligation/wall-strip-and-drawer--ac-7--static
kind: obligation
title: "The Readiness tab shares the loader, and the parity witness shows no gap"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The Readiness tab's facts are the per-request loader's facts for the same spec and head, and the committed wall-shell versus loader parity witness lists no gap for the e2e harness fixture."
  falsifier: "The tab derives readiness on its own, or the parity witness still lists a gap, or a gap was removed from the witness without the behavior changing."
  scope: "The Readiness tab projection and the loader over the e2e harness fixture, and the committed parity witness."
  producer: { kind: test, ref: "go-test:internal/workbench:TestReadinessTab_LoaderParityNoGaps" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/workbench:TestReadinessTab_LoaderParityNoGaps in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer" }
frozen: { at: 2026-09-29, commit: 2e3c21487ee26132146d2aebeec5133fa943cde9 }
---
# The Readiness tab shares the loader, and the parity witness shows no gap

CI job `verify` must record producer `go-test:internal/workbench:TestReadinessTab_LoaderParityNoGaps` at the exact
candidate commit. A test comparing the tab's facts with the loader's and asserting the parity witness's gap list is
empty.

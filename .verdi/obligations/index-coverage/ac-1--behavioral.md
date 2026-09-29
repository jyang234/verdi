---
id: obligation/index-coverage--ac-1--behavioral
kind: obligation
title: "The wall's coverage chips come from the shared function, texts unchanged"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Served over fixture walls, every coverage chip's text equals the text the wall rendered before the extraction, and the chips are computed by the shared function."
  falsifier: "Any chip text changes, or the wall computes coverage outside the shared function."
  scope: "The wall's served snapshot over the committed coverage fixtures, against a golden captured before extraction."
  producer: { kind: test, ref: "go-test:internal/workbench:TestWallCoverageChips_SharedFunction" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/workbench:TestWallCoverageChips_SharedFunction in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-coverage" }
frozen: { at: 2026-09-29, commit: 664e55d0e29c1d8c2858e16469cb80dc20c6acae }
---
# The wall's coverage chips come from the shared function, texts unchanged

CI job `verify` must record producer `go-test:internal/workbench:TestWallCoverageChips_SharedFunction` at the exact
candidate commit. An integration test comparing the served chips with the pre-extraction golden.

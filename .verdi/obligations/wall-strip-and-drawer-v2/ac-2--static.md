---
id: obligation/wall-strip-and-drawer-v2--ac-2--static
kind: obligation
title: "The strip's chip markup carries the badge contract"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The server-rendered strip gives every badge chip a button with data-badge-source and its serialized derivation record, renders flags three-valued from the badge compute layer's entry points, and draws a disclosed-unproven value with the disclosure class."
  falsifier: "A badge chip lacks its source or derivation record, a flag bypasses the compute layer, or an unproven value is drawn with the flag class."
  scope: "Server-rendered strips for flagged, proven-unflagged, and disclosed-unproven fixture walls."
  producer: { kind: test, ref: "go-test:internal/workbench:TestCaseFileStrip_BadgeAndFlagChips" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/workbench:TestCaseFileStrip_BadgeAndFlagChips in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer-v2" }
frozen: { at: 2026-09-30, commit: da729113549288efdee9961dcfa089c2d4ac199a }
---
# The strip's chip markup carries the badge contract

CI job `verify` must record producer `go-test:internal/workbench:TestCaseFileStrip_BadgeAndFlagChips` at the exact
candidate commit. A render test of the strip over fixture walls in each flag state.

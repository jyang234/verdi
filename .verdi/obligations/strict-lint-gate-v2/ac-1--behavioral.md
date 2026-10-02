---
id: obligation/strict-lint-gate-v2--ac-1--behavioral
kind: obligation
title: "The strict configuration reports each gated linter's violation over a fixture module"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Running the pinned golangci-lint with .golangci.strict.yml over a committed fixture module that holds one violation per gated linter and one clean file reports exactly one finding per gated linter and none in the clean file."
  falsifier: "A gated linter's violation goes unreported, a finding appears in the clean file, or an extra finding appears."
  scope: "A committed fixture module under internal/lintratchet/testdata, linted for linux/amd64."
  producer: { kind: test, ref: "go-test:internal/lintratchet:TestLintStrict_ReportsGroundRuleFindings" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/lintratchet:TestLintStrict_ReportsGroundRuleFindings in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/strict-lint-gate-v2" }
frozen: { at: 2026-10-01, commit: 45ac10e7adcc9cb7c2198dfbde8dde8f43e86f48 }
---
# The strict configuration reports each gated linter's violation over a fixture module

CI job `verify` must record producer `go-test:internal/lintratchet:TestLintStrict_ReportsGroundRuleFindings` at the
exact candidate commit. It runs the Makefile's pinned golangci-lint; where that binary is absent it skips with a printed
reason, so in CI job verify, which has it (ledger SI-309), its record is pass or fail, and a skip there would be
recorded as abstain, never as a pass.

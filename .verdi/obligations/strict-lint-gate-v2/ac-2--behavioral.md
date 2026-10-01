---
id: obligation/strict-lint-gate-v2--ac-2--behavioral
kind: obligation
title: "The baseline check passes, fails, and errors exactly as specified"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Over captured golangci-lint reports and fixturegit repositories: equal findings and baseline exit 0; a new finding, a stale allowance, or a baseline grown against the merge base, or on the default branch against HEAD's first parent, exits 1; a line shift or a move within a package exits 0; a move across packages exits 1; a fix followed by a later reintroduction fails the later change; and a truncated report, a malformed baseline, a failing golangci-lint, or an unresolvable merge base or parent exits 2."
  falsifier: "Any listed case yields a different exit status, or a failing case exits 0."
  scope: "internal/lintratchet over captured reports and fixturegit repositories with a baseline at the merge base and at the first parent."
  producer: { kind: test, ref: "go-test:internal/lintratchet:TestRatchet_Verdicts" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/lintratchet:TestRatchet_Verdicts in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/strict-lint-gate-v2" }
frozen: { at: 2026-10-01, commit: 0c42bb3f3d23ce26258ed38807d8190962351f88 }
---
# The baseline check passes, fails, and errors exactly as specified

CI job `verify` must record producer `go-test:internal/lintratchet:TestRatchet_Verdicts` at the exact candidate commit.
A table-driven test with no network; the fix-then-reintroduce case runs across commits of one fixturegit repository.

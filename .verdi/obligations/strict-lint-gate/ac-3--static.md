---
id: obligation/strict-lint-gate--ac-3--static
kind: obligation
title: "Exclusions are named, reasoned, and counted"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "Every exclusion in .golangci.strict.yml names a path or a text pattern and carries a reason, no gated linter is disabled or excluded wholesale, and the number of exclusions equals the count the test pins."
  falsifier: "An exclusion without a path, pattern, or reason; a gated linter disabled or excluded wholesale; or an exclusion count other than the pinned one."
  scope: ".golangci.strict.yml at the candidate commit."
  producer: { kind: test, ref: "go-test:internal/specalign:TestStrictLintExclusionsCounted" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/specalign:TestStrictLintExclusionsCounted in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/strict-lint-gate" }
frozen: { at: 2026-09-30, commit: 9643a79dd1a137f2f9b5cf09d1f905ea997d0a7c }
---
# Exclusions are named, reasoned, and counted

CI job `verify` must record producer `go-test:internal/specalign:TestStrictLintExclusionsCounted` at the exact candidate
commit. A static test; adding an exclusion means changing the pinned count in the same change.

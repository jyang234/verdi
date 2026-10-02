---
id: obligation/strict-lint-gate-v2--ac-3--static
kind: obligation
title: "Exclusions, configured or in source, are named, reasoned, and counted"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "Every exclusion in .golangci.strict.yml names a path or a text pattern and carries a reason, and no gated linter is disabled or excluded wholesale; every //nolint directive in the module's linux/amd64 lint set that names a gated linter carries a reason; none suppresses every linter (a bare //nolint, or a list containing all at any position); and the configuration's exclusions and those source directives each number exactly the count the test pins."
  falsifier: "An exclusion without a path, pattern, or reason; a gated linter disabled or excluded wholesale; a //nolint naming a gated linter without a reason; a bare //nolint, or a //nolint whose list contains all at any position (such as //nolint:unused,all); or either count other than the pinned one."
  scope: ".golangci.strict.yml and the module's linux/amd64 lint set (test files included; testdata directories and nested modules excluded), at the candidate commit."
  producer: { kind: test, ref: "go-test:internal/specalign:TestStrictLintExclusionsCounted" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/specalign:TestStrictLintExclusionsCounted in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/strict-lint-gate-v2" }
frozen: { at: 2026-10-01, commit: 45ac10e7adcc9cb7c2198dfbde8dde8f43e86f48 }
---
# Exclusions, configured or in source, are named, reasoned, and counted

CI job `verify` must record producer `go-test:internal/specalign:TestStrictLintExclusionsCounted` at the exact candidate
commit. A static test; adding an exclusion means changing the pinned count in the same change.

v2 adds the source half: the same test reads every //nolint directive in the linux/amd64 lint set, counts those that
name a gated linter, and fails on one without a reason or one that suppresses every linter: no list, or all anywhere in
its list (spec dc-4).

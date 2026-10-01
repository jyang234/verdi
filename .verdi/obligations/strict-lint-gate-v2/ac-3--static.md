---
id: obligation/strict-lint-gate-v2--ac-3--static
kind: obligation
title: "Exclusions, configured or in source, are named, reasoned, and counted"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "Every exclusion in .golangci.strict.yml names a path or a text pattern and carries a reason, and no gated linter is disabled or excluded wholesale; every //nolint directive in the module's Go source outside testdata that names a gated linter carries a reason; none names no linter (bare or :all); and the configuration's exclusions and those source directives each number exactly the count the test pins."
  falsifier: "An exclusion without a path, pattern, or reason; a gated linter disabled or excluded wholesale; a //nolint naming a gated linter without a reason; a bare //nolint or //nolint:all; or either count other than the pinned one."
  scope: ".golangci.strict.yml and every Go file of the module outside testdata directories and nested modules, at the candidate commit."
  producer: { kind: test, ref: "go-test:internal/specalign:TestStrictLintExclusionsCounted" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/specalign:TestStrictLintExclusionsCounted in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/strict-lint-gate-v2" }
frozen: { at: 2026-10-01, commit: 0c42bb3f3d23ce26258ed38807d8190962351f88 }
---
# Exclusions, configured or in source, are named, reasoned, and counted

CI job `verify` must record producer `go-test:internal/specalign:TestStrictLintExclusionsCounted` at the exact candidate
commit. A static test; adding an exclusion means changing the pinned count in the same change.

v2 adds the source half: the same test reads every //nolint directive outside testdata, counts those that name a gated
linter, and fails on one without a reason or one that names no linter (spec dc-4).

---
id: obligation/strict-lint-gate--ac-1--static
kind: obligation
title: "The strict target is configured and wired, and the parity file is pinned"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The strict configuration enables exactly the five gated linters with errorlint's errorf check only, each under a comment quoting a sentence present in docs/ground-rules.md, and caps no findings; .golangci.yml's sha256 equals the pinned digest; lint-strict is in VERIFY_STEPS; the static job of merge-gate.yml and verify.yml runs make lint-strict; and make -n lint-strict shows --config .golangci.strict.yml for GOOS=linux GOARCH=amd64."
  falsifier: "Any linter added, dropped, or misconfigured; a comment quoting no ground-rules sentence; a nonzero cap; a changed .golangci.yml; lint-strict missing from VERIFY_STEPS or from either workflow's static job; or the recipe lacking the explicit config or the platform."
  scope: ".golangci.strict.yml, .golangci.yml, docs/ground-rules.md, the Makefile, and both workflow files at the candidate commit."
  producer: { kind: test, ref: "go-test:internal/specalign:TestStrictLintTargetIsWired" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/specalign:TestStrictLintTargetIsWired in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/strict-lint-gate" }
frozen: { at: 2026-09-30, commit: 20e97b370067142426db29d8e29198f4a52b0b84 }
---
# The strict target is configured and wired, and the parity file is pinned

CI job `verify` must record producer `go-test:internal/specalign:TestStrictLintTargetIsWired` at the exact candidate
commit. A static test over the committed files, beside the repository's other workflow and Makefile guards.

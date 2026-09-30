---
id: obligation/strict-lint-reach--ac-1--behavioral
kind: obligation
title: "The strict target reports the readiness loader globals before their removal and not after"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Exported from this repository's history, commit 410db101 linted with .golangci.strict.yml for linux/amd64 over cmd/verdi, internal/readinessload, and internal/store reports gochecknoglobals findings for serveReadinessLoader and serveReadinessDefaultSpec, and commit e4d5b141 reports neither."
  falsifier: "Either variable is unreported at 410db101, either is reported at e4d5b141, or a missing commit lets the witness pass."
  scope: "The two commits in the repository's history, exported with git archive into temporary directories."
  producer: { kind: test, ref: "go-test:internal/lintratchet:TestRetroWitness_ReadinessLoaderGlobals" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/lintratchet:TestRetroWitness_ReadinessLoaderGlobals in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/strict-lint-reach" }
frozen: { at: 2026-09-30, commit: 39479c741cf3041646cb743f3dce70a4cd437884 }
---
# The strict target reports the readiness loader globals before their removal and not after

CI job `verify` must record producer `go-test:internal/lintratchet:TestRetroWitness_ReadinessLoaderGlobals` at the exact
candidate commit. It runs the Makefile's pinned golangci-lint; where that binary is absent it skips with a printed
reason, so in CI job verify, which installs it (SI-309), its record is pass or fail, and a skip there would be recorded
as abstain, never as a pass.

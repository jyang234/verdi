---
id: obligation/gitx-recorder-seam--ac-1--static
kind: obligation
title: "One shared token list, git only through gitx, no update-ref in branch creation"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The forbidden-token list is defined once, in a package recovery imports, and holds at least the seven tokens; no production Go file outside internal/gitx executes git; and no gitx function used for design-branch creation passes update-ref."
  falsifier: "A second token list, a missing token, a git execution outside internal/gitx, or update-ref in design-branch creation."
  scope: "The module's production Go source at the candidate commit."
  producer: { kind: test, ref: "go-test:internal/specalign:TestGitRecorderSeamStaticContract" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/specalign:TestGitRecorderSeamStaticContract in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/gitx-recorder-seam" }
frozen: { at: 2026-09-30, commit: b74f35500b7f685387bb4940a58fbb04de4eb303 }
---
# One shared token list, git only through gitx, no update-ref in branch creation

CI job `verify` must record producer `go-test:internal/specalign:TestGitRecorderSeamStaticContract` at the exact
candidate commit. A static test over the source, beside the repository's other structural guards.

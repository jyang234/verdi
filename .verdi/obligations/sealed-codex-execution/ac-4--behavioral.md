---
id: obligation/sealed-codex-execution--ac-4--behavioral
kind: obligation
title: "Resume accepts only a fully reverified continuity record"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Resume reconstructs only from a canonical digest-bound continuity record and fresh recorder, repository, profile, grant, authority, workspace, candidate, manifest, projection, expansion, revision-chain, acknowledgment, and adapter-session facts, with no concurrent replacement dispatch."
  falsifier: "A caller summary, chat history, stale or missing invariant, truncated revision array, mismatched acknowledgment, changed workspace or candidate, missing provider session, or concurrent replacement can resume authoritatively."
  scope: "Hermetic complete-resume and every single-invariant mismatch case, recorder re-query, workspace sidecars, adapter session identity, interruption, suspension, and replacement-dispatch exclusion."
  producer: { kind: test, ref: "go-test:internal/sealedexec:TestContextExecutionResumeContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/sealedexec:TestContextExecutionResumeContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-codex-execution" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Resume accepts only a fully reverified continuity record

CI job `verify` must record producer `go-test:internal/sealedexec:TestContextExecutionResumeContract_Behavioral` at the exact candidate commit.

The evidence must prove: Resume reconstructs only from a canonical digest-bound continuity record and fresh recorder, repository, profile, grant, authority, workspace, candidate, manifest, projection, expansion, revision-chain, acknowledgment, and adapter-session facts, with no concurrent replacement dispatch.

It is falsified when: A caller summary, chat history, stale or missing invariant, truncated revision array, mismatched acknowledgment, changed workspace or candidate, missing provider session, or concurrent replacement can resume authoritatively.

Scope: Hermetic complete-resume and every single-invariant mismatch case, recorder re-query, workspace sidecars, adapter session identity, interruption, suspension, and replacement-dispatch exclusion.

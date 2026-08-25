---
id: obligation/sealed-codex-execution--ac-1--behavioral
kind: obligation
title: "Codex starts only after every declared prerequisite is proven"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Hermetic start fixtures launch the pinned Codex argument vector only after exact accepted authority, clean runway input, detached execution child, isolated project profile, immutable projection, grants, conflict verdict, recorder binding, and opaque vendor disclosures are proven."
  falsifier: "The adapter starts after any missing, mismatched, dirty, ambient, unproven, or malformed prerequisite, edits the runway, reads excluded context, upgrades advisory authority, or returns an unbound output commit or tree."
  scope: "Sealed service preparation and start over fixture Git, fake process, isolated profile, execution-workspace, policy, capability, recorder, and advisory or authoritative requests."
  producer: { kind: test, ref: "go-test:internal/sealedexec:TestContextExecutionContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/sealedexec:TestContextExecutionContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-codex-execution" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Codex starts only after every declared prerequisite is proven

CI job `verify` must record producer `go-test:internal/sealedexec:TestContextExecutionContract_Behavioral` at the exact candidate commit.

The evidence must prove: Hermetic start fixtures launch the pinned Codex argument vector only after exact accepted authority, clean runway input, detached execution child, isolated project profile, immutable projection, grants, conflict verdict, recorder binding, and opaque vendor disclosures are proven.

It is falsified when: The adapter starts after any missing, mismatched, dirty, ambient, unproven, or malformed prerequisite, edits the runway, reads excluded context, upgrades advisory authority, or returns an unbound output commit or tree.

Scope: Sealed service preparation and start over fixture Git, fake process, isolated profile, execution-workspace, policy, capability, recorder, and advisory or authoritative requests.

---
id: obligation/vatc-forge-countersign-v2--ac-1--behavioral
kind: obligation
title: "GitHub and GitLab normalize current approvals without ambiguity"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Hermetic GitHub and GitLab adapters normalize paginated current approval facts bound to the forge-reported candidate SHA and reject revoked, dismissed, duplicate, incomplete, unknown, trailing, or ambiguously paginated responses."
  falsifier: "Either adapter accepts an adverse or ambiguous provider response, loses approval identity or principal evidence, binds the wrong change head, contacts the network, or produces provider-dependent semantics."
  scope: "Canned GitHub and GitLab HTTP responses, pagination boundaries, revocation and dismissal transitions, duplicate identities, and current-head binding."
  producer: { kind: test, ref: "go-test:internal/forge:TestForgeApprovalContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/forge:TestForgeApprovalContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-forge-countersign-v2" }
frozen: { at: 2026-09-22, commit: 293e3b220fb85937601fae0b2aa49cd9be05ae11 }
---
# GitHub and GitLab normalize current approvals without ambiguity

CI job `verify` must record producer `go-test:internal/forge:TestForgeApprovalContract_Behavioral` at the exact candidate commit.

The evidence must prove: Hermetic GitHub and GitLab adapters normalize paginated current approval facts bound to the forge-reported candidate SHA and reject revoked, dismissed, duplicate, incomplete, unknown, trailing, or ambiguously paginated responses.

It is falsified when: Either adapter accepts an adverse or ambiguous provider response, loses approval identity or principal evidence, binds the wrong change head, contacts the network, or produces provider-dependent semantics.

Scope: Canned GitHub and GitLab HTTP responses, pagination boundaries, revocation and dismissal transitions, duplicate identities, and current-head binding.

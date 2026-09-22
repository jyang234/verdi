---
id: obligation/vatc-forge-countersign-v2--ac-4--behavioral
kind: obligation
title: "Only a solo profile honors the owner's close-run environment review"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Under a solo profile whose close roles map to the owner's forge principal, an approved environment review of the dispatch-only close run whose head equals the open close change head and the locally evaluated commit proves the close countersign with the kernel's solo role-collapse disclosure; every other case yields no approval with its witness."
  falsifier: "An environment review contributes under a team or high-assurance profile, a rejected, pending, absent, cancelled, other-run, or other-attempt review contributes, a head mismatch still proves, the solo disclosure is missing from the witness, or the resolver requests or writes an approval."
  scope: "Hermetic resolver tables over the governance kernel with solo, team, and high-assurance profiles, and canned GitHub run, job, and review-history facts including rejected, pending, cancelled, retried, and wrong-head runs."
  producer: { kind: test, ref: "go-test:internal/lifecyclecountersign:TestSoloEnvironmentReviewCountersign_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/lifecyclecountersign:TestSoloEnvironmentReviewCountersign_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-forge-countersign-v2" }
frozen: { at: 2026-09-22, commit: 293e3b220fb85937601fae0b2aa49cd9be05ae11 }
---
# Only a solo profile honors the owner's close-run environment review

CI job `verify` must record producer `go-test:internal/lifecyclecountersign:TestSoloEnvironmentReviewCountersign_Behavioral` at the exact candidate commit.

The evidence must prove: Under a solo profile whose close roles map to the owner's forge principal, an approved environment review of the dispatch-only close run whose head equals the open close change head and the locally evaluated commit proves the close countersign with the kernel's solo role-collapse disclosure; every other case yields no approval with its witness.

It is falsified when: An environment review contributes under a team or high-assurance profile, a rejected, pending, absent, cancelled, other-run, or other-attempt review contributes, a head mismatch still proves, the solo disclosure is missing from the witness, or the resolver requests or writes an approval.

Scope: Hermetic resolver tables over the governance kernel with solo, team, and high-assurance profiles, and canned GitHub run, job, and review-history facts including rejected, pending, cancelled, retried, and wrong-head runs.

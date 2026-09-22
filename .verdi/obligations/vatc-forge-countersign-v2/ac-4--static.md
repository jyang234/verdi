---
id: obligation/vatc-forge-countersign-v2--ac-4--static
kind: obligation
title: "An environment review is a strict, derived-and-disclosed approval fact"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "The GitHub adapter normalizes an approved environment review of the dispatch-only close run's first attempt into the shared approval fact with the composite identity of repository, run id, run attempt, environment id, and reviewer id, the run's head commit as candidate, the shared active state with GitHub's approved kept as a provider witness, the gated job's creation stamp as a conservative approved-at, and provider witnesses that state the derived fields, the job's start separately, and the environment's self-review setting when reported."
  falsifier: "The fact lacks a composite component, is produced for a rerun attempt, reuses one identity across runs or attempts, takes its candidate from anything but the run's head commit, carries a state other than active, uses the job's start or any stamp later than the review as approved-at, invents a review id or review time, drops a derived-field disclosure, decodes an unknown review state, or relies on test fixtures that add an attempt field GitHub's review history does not supply."
  scope: "The shared forge approval value, the GitHub environment-review decoder and normalizer, and their strict decoding boundaries over canned run, attempt-specific job, and review-history responses shaped exactly as GitHub publishes them."
  producer: { kind: test, ref: "go-test:internal/forge:TestEnvironmentReviewApprovalContract_Static" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/forge:TestEnvironmentReviewApprovalContract_Static in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-forge-countersign-v2" }
frozen: { at: 2026-09-22, commit: 293e3b220fb85937601fae0b2aa49cd9be05ae11 }
---
# An environment review is a strict, derived-and-disclosed approval fact

CI job `verify` must record producer `go-test:internal/forge:TestEnvironmentReviewApprovalContract_Static` at the exact candidate commit.

The evidence must prove: The GitHub adapter normalizes an approved environment review of the dispatch-only close run's first attempt into the shared approval fact with the composite identity of repository, run id, run attempt, environment id, and reviewer id, the run's head commit as candidate, the shared active state with GitHub's approved kept as a provider witness, the gated job's creation stamp as a conservative approved-at, and provider witnesses that state the derived fields, the job's start separately, and the environment's self-review setting when reported.

It is falsified when: The fact lacks a composite component, is produced for a rerun attempt, reuses one identity across runs or attempts, takes its candidate from anything but the run's head commit, carries a state other than active, uses the job's start or any stamp later than the review as approved-at, invents a review id or review time, drops a derived-field disclosure, decodes an unknown review state, or relies on test fixtures that add an attempt field GitHub's review history does not supply.

Scope: The shared forge approval value, the GitHub environment-review decoder and normalizer, and their strict decoding boundaries over canned run, attempt-specific job, and review-history responses shaped exactly as GitHub publishes them.

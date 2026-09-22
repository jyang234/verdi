---
id: obligation/vatc-forge-countersign-v2--ac-4--static
kind: obligation
title: "An environment review is a strict, derived-and-disclosed approval fact"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "The GitHub adapter normalizes an approved environment review of the dispatch-only close run into the shared approval fact with the composite identity of repository, run id, run attempt, environment id, and reviewer id, the run's head commit as candidate, the gated job's start as approved-at, and provider witnesses that state both derived fields and the environment's self-review setting when reported."
  falsifier: "The fact lacks a composite component, reuses one identity across runs or attempts, takes its candidate from anything but the run's head commit, invents a review id or review time, drops the derived-field disclosure, or decodes an unknown review state."
  scope: "The shared forge approval value, the GitHub environment-review decoder and normalizer, and their strict decoding boundaries over canned run, job, and review-history responses."
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

The evidence must prove: The GitHub adapter normalizes an approved environment review of the dispatch-only close run into the shared approval fact with the composite identity of repository, run id, run attempt, environment id, and reviewer id, the run's head commit as candidate, the gated job's start as approved-at, and provider witnesses that state both derived fields and the environment's self-review setting when reported.

It is falsified when: The fact lacks a composite component, reuses one identity across runs or attempts, takes its candidate from anything but the run's head commit, invents a review id or review time, drops the derived-field disclosure, or decodes an unknown review state.

Scope: The shared forge approval value, the GitHub environment-review decoder and normalizer, and their strict decoding boundaries over canned run, job, and review-history responses.

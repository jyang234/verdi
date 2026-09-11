---
id: obligation/public-execution-contract--ac-2--static
kind: obligation
title: "Validation and admission — static evidence"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The validation inventory maps every relevant incumbent check to a new function and invocation point at both endpoints, classifies all 16 relation-bearing and six no-extra-relation arms, and identifies bounded-read and observer admission paths."
  falsifier: "A check is absent, weakened, assigned only to the opposite endpoint, or the claimed reader/admission path is not the invoked one."
  scope: "All 22 owner arms, six explicitly classified no-extra-relation arms, claims registration, recorder observation, incremental reply reading, cancellation, EOF, partial frames, and the check-to-function/invocation/mutation witness."
  producer: { kind: checker, ref: "public-execution-contract:ac-2:static" }
  authoritative_source: { kind: ci-job, ref: "public-execution-contract-release" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun public-execution-contract:ac-2:static in CI job public-execution-contract-release for the exact paired source commits, executable SHA-256 values, contract bytes and governing specification after a change to either repository or contract. Refuse missing, stale, skipped or unproven required evidence."
links:
  - { type: verifies, ref: "spec/public-execution-contract" }
frozen: { at: 2026-09-10, commit: f36be8e33f3cebea81b7c9690b5f2b8aeeeee320 }
---
# Validation and admission — static evidence

The validation inventory maps every relevant incumbent check to a new function and invocation point at both endpoints, classifies all 16 relation-bearing and six no-extra-relation arms, and identifies bounded-read and observer admission paths.

Falsifier: A check is absent, weakened, assigned only to the opposite endpoint, or the claimed reader/admission path is not the invoked one.

Scope: All 22 owner arms, six explicitly classified no-extra-relation arms, claims registration, recorder observation, incremental reply reading, cancellation, EOF, partial frames, and the check-to-function/invocation/mutation witness.

Producer `public-execution-contract:ac-2:static` must report through CI job
`public-execution-contract-release` for the exact paired artifacts. These are
future internal evidence-producer identities, not existing public CLI commands
or a claim that the release job currently exists. Implementation supplies them
as part of contract §6’s matched-pair release gate. Both ordinary repository
verification gates remain mandatory and cannot be replaced by this report.

The checker must bind actual source/check results and reject absent, stale,
skipped or unproven required rows; a hand-written success declaration is not
evidence. Static evidence examines code, registries, call sites and inventories;
behavioral evidence executes the required positive and rejecting mutations,
built-binary paths, or crash/replay matrix within this obligation’s scope.

No runtime evidence has been produced during specification preparation.

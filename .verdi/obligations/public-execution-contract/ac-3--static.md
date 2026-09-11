---
id: obligation/public-execution-contract--ac-3--static
kind: obligation
title: "Stable identities and durable recovery — static evidence"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The paired schema and identity inventory preserves the precise legacy preimage transformation and all durable records, distinguishes receipt row and control-triple writes, and maps each crash cut to its baseline classifier."
  falsifier: "A durable format or identity changes, separate writes are described as one transaction, or the recovery matrix omits a cut or the same-epoch start limitation."
  scope: "Golden preimages, dispatch and receipt identities, two receipt writes, all five deterministic process-crash cuts under baseline and candidate, actual re-entry classifiers/exits/rows/launches, and two-way copied-store decode/replay."
  producer: { kind: checker, ref: "public-execution-contract:ac-3:static" }
  authoritative_source: { kind: ci-job, ref: "public-execution-contract-release" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun public-execution-contract:ac-3:static in CI job public-execution-contract-release for the exact paired source commits, executable SHA-256 values, contract bytes and governing specification after a change to either repository or contract. Refuse missing, stale, skipped or unproven required evidence."
links:
  - { type: verifies, ref: "spec/public-execution-contract" }
frozen: { at: 2026-09-10, commit: f36be8e33f3cebea81b7c9690b5f2b8aeeeee320 }
---
# Stable identities and durable recovery — static evidence

The paired schema and identity inventory preserves the precise legacy preimage transformation and all durable records, distinguishes receipt row and control-triple writes, and maps each crash cut to its baseline classifier.

Falsifier: A durable format or identity changes, separate writes are described as one transaction, or the recovery matrix omits a cut or the same-epoch start limitation.

Scope: Golden preimages, dispatch and receipt identities, two receipt writes, all five deterministic process-crash cuts under baseline and candidate, actual re-entry classifiers/exits/rows/launches, and two-way copied-store decode/replay.

Producer `public-execution-contract:ac-3:static` must report through CI job
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

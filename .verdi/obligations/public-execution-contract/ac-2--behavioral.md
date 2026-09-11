---
id: obligation/public-execution-contract--ac-2--behavioral
kind: obligation
title: "Validation and admission — behavioral evidence"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Both endpoints preserve structural validation and independently enforce all 16 published request/result relations at their required invocation points; ATC validates a recorder arm before observing its event, and both receivers enforce the exclusive 32 MiB bound while reading with the existing capability-usability distinction."
  falsifier: "ATC frames a contradictory answer as success, Verdi omits its consuming check, a malformed recorder arm creates a row, an oversized or unterminated reply grows without the bound, or a relation refusal poisons the quarantine-capable channel."
  scope: "All 22 owner arms, six explicitly classified no-extra-relation arms, claims registration, recorder observation, incremental reply reading, cancellation, EOF, partial frames, and the check-to-function/invocation/mutation witness."
  producer: { kind: checker, ref: "public-execution-contract:ac-2:behavioral" }
  authoritative_source: { kind: ci-job, ref: "public-execution-contract-release" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun public-execution-contract:ac-2:behavioral in CI job public-execution-contract-release for the exact paired source commits, executable SHA-256 values, contract bytes and governing specification after a change to either repository or contract. Refuse missing, stale, skipped or unproven required evidence."
links:
  - { type: verifies, ref: "spec/public-execution-contract" }
frozen: { at: 2026-09-10, commit: f36be8e33f3cebea81b7c9690b5f2b8aeeeee320 }
---
# Validation and admission — behavioral evidence

Both endpoints preserve structural validation and independently enforce all 16 published request/result relations at their required invocation points; ATC validates a recorder arm before observing its event, and both receivers enforce the exclusive 32 MiB bound while reading with the existing capability-usability distinction.

Falsifier: ATC frames a contradictory answer as success, Verdi omits its consuming check, a malformed recorder arm creates a row, an oversized or unterminated reply grows without the bound, or a relation refusal poisons the quarantine-capable channel.

Scope: All 22 owner arms, six explicitly classified no-extra-relation arms, claims registration, recorder observation, incremental reply reading, cancellation, EOF, partial frames, and the check-to-function/invocation/mutation witness.

Producer `public-execution-contract:ac-2:behavioral` must report through CI job
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

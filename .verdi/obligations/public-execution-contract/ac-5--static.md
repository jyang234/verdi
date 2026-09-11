---
id: obligation/public-execution-contract--ac-5--static
kind: obligation
title: "One public operation codec per repository — static evidence"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The deletion inventory proves there is one public operation-wire implementation per repository, required domain/artifact validators remain owned, and every removed private check has its function/invocation/mutation destination."
  falsifier: "Private operation serialization remains duplicated, a domain validator is deleted, or a forbidden dependency/policy owner is introduced."
  scope: "Verdi contextowner and sealedexec seams, ATC independent public validator, standalone compatibility verbs, retained domain/artifact validators, and the completed two-flight release evidence."
  producer: { kind: checker, ref: "public-execution-contract:ac-5:static" }
  authoritative_source: { kind: ci-job, ref: "public-execution-contract-release" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun public-execution-contract:ac-5:static in CI job public-execution-contract-release for the exact paired source commits, executable SHA-256 values, contract bytes and governing specification after a change to either repository or contract. Refuse missing, stale, skipped or unproven required evidence."
links:
  - { type: verifies, ref: "spec/public-execution-contract" }
frozen: { at: 2026-09-10, commit: f36be8e33f3cebea81b7c9690b5f2b8aeeeee320 }
---
# One public operation codec per repository — static evidence

The deletion inventory proves there is one public operation-wire implementation per repository, required domain/artifact validators remain owned, and every removed private check has its function/invocation/mutation destination.

Falsifier: Private operation serialization remains duplicated, a domain validator is deleted, or a forbidden dependency/policy owner is introduced.

Scope: Verdi contextowner and sealedexec seams, ATC independent public validator, standalone compatibility verbs, retained domain/artifact validators, and the completed two-flight release evidence.

Producer `public-execution-contract:ac-5:static` must report through CI job
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

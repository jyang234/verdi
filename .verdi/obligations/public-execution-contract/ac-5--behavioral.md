---
id: obligation/public-execution-contract--ac-5--behavioral
kind: obligation
title: "One public operation codec per repository — behavioral evidence"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The completed change removes Verdi’s separate private serialized operation representation, preserves required domain conversions and validators, and returns a deletion inventory with each removed check mapped to its new function, invocation point, and rejecting mutation."
  falsifier: "Transport-only delivery is called complete, a private semantic wire codec remains independently maintained, a producer check moves solely to the consumer, or consolidation creates a second policy owner or a forbidden package import."
  scope: "Verdi contextowner and sealedexec seams, ATC independent public validator, standalone compatibility verbs, retained domain/artifact validators, and the completed two-flight release evidence."
  producer: { kind: checker, ref: "public-execution-contract:ac-5:behavioral" }
  authoritative_source: { kind: ci-job, ref: "public-execution-contract-release" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun public-execution-contract:ac-5:behavioral in CI job public-execution-contract-release for the exact paired source commits, executable SHA-256 values, contract bytes and governing specification after a change to either repository or contract. Refuse missing, stale, skipped or unproven required evidence."
links:
  - { type: verifies, ref: "spec/public-execution-contract" }
frozen: { at: 2026-09-10, commit: f36be8e33f3cebea81b7c9690b5f2b8aeeeee320 }
---
# One public operation codec per repository — behavioral evidence

The completed change removes Verdi’s separate private serialized operation representation, preserves required domain conversions and validators, and returns a deletion inventory with each removed check mapped to its new function, invocation point, and rejecting mutation.

Falsifier: Transport-only delivery is called complete, a private semantic wire codec remains independently maintained, a producer check moves solely to the consumer, or consolidation creates a second policy owner or a forbidden package import.

Scope: Verdi contextowner and sealedexec seams, ATC independent public validator, standalone compatibility verbs, retained domain/artifact validators, and the completed two-flight release evidence.

Producer `public-execution-contract:ac-5:behavioral` must report through CI job
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

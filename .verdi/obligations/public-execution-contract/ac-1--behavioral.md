---
id: obligation/public-execution-contract--ac-1--behavioral
kind: obligation
title: "Direct public transport — behavioral evidence"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The matched Verdi and ATC pair exchanges the 22 public owner arms and the fixed local claims exception over strict FD3 v2, with zero production owner decode/encode subprocess launches and unchanged standalone bridge commands."
  falsifier: "A wrong arm, version, union, sequence, or operation is accepted, any execution consumer remains on v1, or a built production execution launches an owner translation command."
  scope: "FD3 codecs, all openSealedController consumers, ATC routing, standalone bridge compatibility, literal fixtures for all 23 operations, and built-binary launch traces."
  producer: { kind: checker, ref: "public-execution-contract:ac-1:behavioral" }
  authoritative_source: { kind: ci-job, ref: "public-execution-contract-release" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun public-execution-contract:ac-1:behavioral in CI job public-execution-contract-release for the exact paired source commits, executable SHA-256 values, contract bytes and governing specification after a change to either repository or contract. Refuse missing, stale, skipped or unproven required evidence."
links:
  - { type: verifies, ref: "spec/public-execution-contract" }
frozen: { at: 2026-09-10, commit: f36be8e33f3cebea81b7c9690b5f2b8aeeeee320 }
---
# Direct public transport — behavioral evidence

The matched Verdi and ATC pair exchanges the 22 public owner arms and the fixed local claims exception over strict FD3 v2, with zero production owner decode/encode subprocess launches and unchanged standalone bridge commands.

Falsifier: A wrong arm, version, union, sequence, or operation is accepted, any execution consumer remains on v1, or a built production execution launches an owner translation command.

Scope: FD3 codecs, all openSealedController consumers, ATC routing, standalone bridge compatibility, literal fixtures for all 23 operations, and built-binary launch traces.

Producer `public-execution-contract:ac-1:behavioral` must report through CI job
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

---
id: obligation/public-execution-contract--ac-1--static
kind: obligation
title: "Direct public transport — static evidence"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The paired source inventory declares exactly the ratified v2 envelopes, 22 public owner arms, local claims exception, and all controller consumers, with no production bridge invocation path."
  falsifier: "A registry or consumer is missing, literal schema differs, or production routing still reaches a translation subprocess."
  scope: "FD3 codecs, all openSealedController consumers, ATC routing, standalone bridge compatibility, literal fixtures for all 23 operations, and built-binary launch traces."
  producer: { kind: checker, ref: "public-execution-contract:ac-1:static" }
  authoritative_source: { kind: ci-job, ref: "public-execution-contract-release" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun public-execution-contract:ac-1:static in CI job public-execution-contract-release for the exact paired source commits, executable SHA-256 values, contract bytes and governing specification after a change to either repository or contract. Refuse missing, stale, skipped or unproven required evidence."
links:
  - { type: verifies, ref: "spec/public-execution-contract" }
frozen: { at: 2026-09-10, commit: f36be8e33f3cebea81b7c9690b5f2b8aeeeee320 }
---
# Direct public transport — static evidence

The paired source inventory declares exactly the ratified v2 envelopes, 22 public owner arms, local claims exception, and all controller consumers, with no production bridge invocation path.

Falsifier: A registry or consumer is missing, literal schema differs, or production routing still reaches a translation subprocess.

Scope: FD3 codecs, all openSealedController consumers, ATC routing, standalone bridge compatibility, literal fixtures for all 23 operations, and built-binary launch traces.

Producer `public-execution-contract:ac-1:static` must report through CI job
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

---
id: obligation/public-execution-contract--ac-3--behavioral
kind: obligation
title: "Stable identities and durable recovery — behavioral evidence"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The legacy controller request digest preimage for operations 1–22, every durable format, receipt event/control-triple ordering, recovery authority outcome, and copied-store replay in both directions remain as fixed in contract sections 2.2 and 4–5."
  falsifier: "A public arm or v2 envelope replaces the legacy preimage, an acknowledgment precedes its required commit, a receipt alone authorizes candidate replay, a crash cut loses evidence or changes authority outcome, or either rollback direction needs historical-store rewriting."
  scope: "Golden preimages, dispatch and receipt identities, two receipt writes, all five deterministic process-crash cuts under baseline and candidate, actual re-entry classifiers/exits/rows/launches, and two-way copied-store decode/replay."
  producer: { kind: checker, ref: "public-execution-contract:ac-3:behavioral" }
  authoritative_source: { kind: ci-job, ref: "public-execution-contract-release" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun public-execution-contract:ac-3:behavioral in CI job public-execution-contract-release for the exact paired source commits, executable SHA-256 values, contract bytes and governing specification after a change to either repository or contract. Refuse missing, stale, skipped or unproven required evidence."
links:
  - { type: verifies, ref: "spec/public-execution-contract" }
frozen: { at: 2026-09-10, commit: f36be8e33f3cebea81b7c9690b5f2b8aeeeee320 }
---
# Stable identities and durable recovery — behavioral evidence

The legacy controller request digest preimage for operations 1–22, every durable format, receipt event/control-triple ordering, recovery authority outcome, and copied-store replay in both directions remain as fixed in contract sections 2.2 and 4–5.

Falsifier: A public arm or v2 envelope replaces the legacy preimage, an acknowledgment precedes its required commit, a receipt alone authorizes candidate replay, a crash cut loses evidence or changes authority outcome, or either rollback direction needs historical-store rewriting.

Scope: Golden preimages, dispatch and receipt identities, two receipt writes, all five deterministic process-crash cuts under baseline and candidate, actual re-entry classifiers/exits/rows/launches, and two-way copied-store decode/replay.

Producer `public-execution-contract:ac-3:behavioral` must report through CI job
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

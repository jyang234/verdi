---
id: obligation/public-execution-contract--ac-4--static
kind: obligation
title: "Matched-pair negotiation and rollout — static evidence"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The release inventory pins both source commits, executable hashes and contract bytes, maps the actual-run query before every named flight effect, and records the full quiescent upgrade and rollback constraints."
  falsifier: "An identity is unbound, the query exists only in dry run, an effect precedes it, or mixed-version in-flight recovery is inferred from schemas."
  scope: "Actual-run and dry-run version matrix, all Verdi controller hosts, pre-effect refusal evidence, baseline stored-plan limitation, copied stores, exact source commits/binary hashes/contract bytes, and offline rollout instructions."
  producer: { kind: checker, ref: "public-execution-contract:ac-4:static" }
  authoritative_source: { kind: ci-job, ref: "public-execution-contract-release" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun public-execution-contract:ac-4:static in CI job public-execution-contract-release for the exact paired source commits, executable SHA-256 values, contract bytes and governing specification after a change to either repository or contract. Refuse missing, stale, skipped or unproven required evidence."
links:
  - { type: verifies, ref: "spec/public-execution-contract" }
frozen: { at: 2026-09-10, commit: f36be8e33f3cebea81b7c9690b5f2b8aeeeee320 }
---
# Matched-pair negotiation and rollout — static evidence

The release inventory pins both source commits, executable hashes and contract bytes, maps the actual-run query before every named flight effect, and records the full quiescent upgrade and rollback constraints.

Falsifier: An identity is unbound, the query exists only in dry run, an effect precedes it, or mixed-version in-flight recovery is inferred from schemas.

Scope: Actual-run and dry-run version matrix, all Verdi controller hosts, pre-effect refusal evidence, baseline stored-plan limitation, copied stores, exact source commits/binary hashes/contract bytes, and offline rollout instructions.

Producer `public-execution-contract:ac-4:static` must report through CI job
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

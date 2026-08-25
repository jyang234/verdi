---
id: spec/vatc-forge-countersign
kind: spec
title: "Authenticated forge approvals as countersigns"
owners: [platform-team]
class: story
story: jira:VERDI-ATC-2
problem: { text: "story and feature closure require countersigns, but Verdi cannot yet map a live forge approval to that obligation without either trusting unauthenticated metadata or writing a vouch into the reviewed candidate tree", anchor: problem }
outcome: { text: "Verdi resolves current GitHub and GitLab approvals into strict candidate-bound countersign witnesses, uses them in story gate and story or feature closure, and leaves the reviewed tree byte-identical", anchor: outcome }
acceptance_criteria:
  - id: ac-1
    text: "the forge consumer port and both GitHub and GitLab adapters return strict approval facts bound to repository, change, immutable approval identity, exact current candidate SHA, forge state, authenticated principal evidence, and forge freshness witnesses"
    evidence: [static, behavioral]
    anchor: ac-1
  - id: ac-2
    text: "the countersign resolver emits canonical verdi.countersign-witness/v1 and satisfies an attestation/countersign obligation only when the required story-review or feature-UAT role, authenticated principal, active approval state, candidate SHA, freshness policy, count, and configured separation of duties are proven"
    evidence: [static, behavioral]
    anchor: ac-2
  - id: ac-3
    text: "build gate and story or feature close preflight consume the same resolver, preserve the approval reference and principal in canonical journey or closure evidence, reject revoked, dismissed, stale, wrong-head, duplicated, self-approved, unconfigured, or unreachable cases honestly, and write no countersign file"
    evidence: [behavioral]
    anchor: ac-3
links:
  - { type: implements, ref: "spec/verdi-atc-prerequisites#ac-2" }
decisions:
  - id: dc-1
    text: "U2 adds no standalone CLI or MCP operation: existing verdi gate, verdi close --preflight, verdi close --prepare, and close publication consume the forge-backed resolver; machine consumers read its witness through the canonical journey or closure evidence they already consume"
    anchor: dc-1
  - id: dc-2
    text: "verdi.countersign-witness/v1 contains schema, repository, forge, change_id, approval_id, approval_ref, candidate_sha, approval_state, approved_at, observed_at, principal_resolution, obligation, freshness, verdict, and witnesses; digests cover normalized values and never credential material"
    anchor: dc-2
  - id: dc-3
    text: "the current candidate is the forge-reported change head and must equal the locally evaluated full commit SHA; branch names, tree equality alone, display names, author claims, comments, reactions, and historical approvals cannot substitute"
    anchor: dc-3
  - id: dc-4
    text: "a configured but unreachable forge, an approval whose identity or freshness cannot be proven, and absent forge configuration are disclosed or blocking according to the consuming transition; none are interpreted as approval"
    anchor: dc-4
constraints:
  - id: co-1
    text: "forge response decoders reject unknown fields where the provider contract is closed, trailing data, unknown states, missing IDs, ambiguous pagination, and duplicate approval identities; adapters normalize provider-specific shapes behind the existing consumer-defined forge port"
    anchor: co-1
  - id: co-2
    text: "tests use httptest and the shared forge contract fake only, include GitHub and GitLab pagination and revocation cases, and make no network calls"
    anchor: co-2
  - id: co-3
    text: "the resolver is read-only with respect to the candidate Git tree and cannot create, edit, stage, commit, or request an approval"
    anchor: co-3
---
# Authenticated forge approvals as countersigns

## Problem

The approval that satisfies a countersign lives at the forge, not in the
candidate. A local file would mutate the bytes after review, while a bare
approval label would not prove actor, state, freshness, or candidate binding.

## Outcome

One resolver translates provider facts into Verdi's three-valued authority
model. Story review approvals satisfy the story countersign; the owner's G3
approval satisfies the feature-UAT countersign. The witness remains evidence,
not a second lifecycle state.

## AC-1

The port returns provider facts rather than provider judgments. Principal
authentication remains the governance kernel's responsibility.

## AC-2

The witness names every operand needed to reproduce the countersign verdict.
`proven` is reachable only when all operands prove the required obligation;
violations carry witnesses and unavailable facts remain disclosed as unproven.

## AC-3

Gate and closure share the resolver. Their adverse exit behavior follows the
existing transition contract, and neither path changes candidate bytes.

## Decisions

### DC-1

Existing gate, close, journey, and closure evidence carry the resolver; no new
standalone command exists.

### DC-2

The witness schema contains the closed identity, approval, obligation,
freshness, verdict, and witness fields declared above.

### DC-3

Only the full forge change-head SHA matched to local HEAD binds a candidate.

### DC-4

Unavailable or unauthenticated forge facts are blocking or disclosed-unproven,
never approval.

## Constraints

### CO-1

Both provider adapters strict-decode and reject ambiguous or duplicate facts.

### CO-2

Forge contract tests use hermetic HTTP and in-memory fakes without network.

### CO-3

Countersign resolution is read-only and cannot request approval or mutate Git.

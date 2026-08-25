---
id: spec/verdi-atc-prerequisites
kind: spec
title: "Verdi-ATC prerequisite surfaces"
owners: [platform-team]
class: feature
problem:
  text: "the ratified Verdi-ATC orchestrator cannot safely automate Verdi's build lifecycle because several required authority boundaries are absent or human-text-only: matrix output lacks a canonical machine projection, forge approvals do not satisfy countersign obligations, build commands are not contract-tested from ATC-owned worktrees, and sealed Codex and Claude execution cannot yet prove scoped context, recorder continuity, or authenticated builder and fresh-review receipts"
  anchor: problem
outcome:
  text: "Verdi exposes six strict, deterministic, harness-neutral prerequisite surfaces that let Verdi-ATC consume lifecycle truth, authenticate out-of-tree countersigns, operate only in its runway worktrees, dispatch sealed Codex and Claude sessions, stream all observable execution activity to VATC, and verify builder and fresh-review receipts without importing Verdi packages or treating ambient context as authority"
  anchor: outcome
acceptance_criteria:
  - id: ac-1
    text: "matrix and journey have explicit canonical JSON modes, with matrix CLI and MCP consuming one shared projection and preserving Verdi's report-versus-verdict exit semantics"
    evidence: [static, behavioral, attestation]
    anchor: ac-1
  - id: ac-2
    text: "authenticated, current forge review approvals bind an approver principal and exact candidate commit to story-review or feature-UAT countersign obligations used by gate and closure, without writing a countersign artifact into the reviewed tree"
    evidence: [static, behavioral, attestation]
    anchor: ac-2
  - id: ac-3
    text: "build start, align, disposition, gate, and close preparation are proven to discover and mutate only the invoking non-primary ATC runway worktree, with mismatched branch bases and operational Git failures refused with witnesses"
    evidence: [behavioral, attestation]
    anchor: ac-3
  - id: ac-4
    text: "an authoritative Codex execution uses a sealed minimum-context manifest and immutable instruction projection, proves project-profile isolation plus a detached execution-workspace child of the clean ATC runway, records classified context expansions and a canonical cross-revision continuity record, and refuses authority on invalidation, discontinuity, unverifiable resume, or unsafe handback"
    evidence: [static, behavioral, attestation]
    anchor: ac-4
  - id: ac-5
    text: "builder and fresh-review outputs close only through canonical receipts authenticated by a trusted runner and bound to exact manifests, dispatches, repository states, event roots, obligations, expansions, evidence, and review inputs; unsigned or unverifiable receipts remain advisory"
    evidence: [static, behavioral, attestation]
    anchor: ac-5
  - id: ac-6
    text: "Claude Code has the same sealed-execution invariants as Codex and both adapters emit the complete provider-observable activity stream through one strict redacted VATC event profile with a closed per-kind payload union whose gaps, unavailable telemetry, and segment boundaries are explicit and receipt-bound"
    evidence: [static, behavioral, attestation]
    anchor: ac-6
stubs:
  - { slug: vatc-machine-projections, acceptance_criteria: [ac-1] }
  - { slug: vatc-forge-countersign, acceptance_criteria: [ac-2] }
  - { slug: vatc-build-worktree-contract, acceptance_criteria: [ac-3] }
  - { slug: sealed-codex-execution, acceptance_criteria: [ac-4] }
  - { slug: context-receipts-review, acceptance_criteria: [ac-5] }
  - { slug: sealed-claude-vatc-events, acceptance_criteria: [ac-6] }
links:
  - { type: depends-on, ref: spec/context-integrity-v2 }
  - { type: depends-on, ref: spec/guided-lifecycle-governance-v3 }
  - { type: depends-on, ref: spec/execution-workspace }
decisions:
  - id: dc-1
    text: "Verdi owns lifecycle projections, context compilation, instruction authority, conflict evaluation, isolated execution, event normalization, principal resolution, countersign evaluation, and receipt semantics; Verdi-ATC invokes pinned Verdi CLI or MCP surfaces and strict-decodes their results, never importing Verdi packages or reproducing those decisions"
    anchor: dc-1
  - id: dc-2
    text: "this feature is the canonical Verdi home for the six externally ratified Stage 0.5 prerequisites; sealed-codex-execution also implements Context Integrity v2 AC-4 and context-receipts-review also implements Context Integrity v2 AC-5, while the other four stories are not misclassified as Context Integrity delivery units"
    anchor: dc-2
  - id: dc-3
    text: "the accepted child stories own exact public command, MCP, schema, and witness names; an implementation flight may not add, rename, or reinterpret a public wire without a new recorded authority decision"
    anchor: dc-3
  - id: dc-4
    text: "a forge approval is the countersign itself only after Verdi proves its authenticated principal, required role, current active state, freshness, separation of duties, and binding to the exact candidate commit; no file-based countersign mode exists"
    anchor: dc-4
  - id: dc-5
    text: "Verdi certifies only project-controlled context and provider-observable activity: vendor-controlled inputs stay disclosed as opaque, provider summaries are operator telemetry rather than gate evidence, and neither an adapter nor VATC claims access to hidden reasoning"
    anchor: dc-5
  - id: dc-6
    text: "all authoritative execution events are acknowledged only after VATC has strictly decoded, identity-bound, sequence-checked, redacted, durably appended, and projected them; recorder loss, a sequence gap, failed redaction, or an invalid event stops or suspends the producer and prevents an authoritative receipt"
    anchor: dc-6
constraints:
  - id: co-1
    text: "all new YAML and JSON strict-decode through Verdi's existing seams; unknown fields, schemas, enum values, event kinds, capability kinds, and trailing data fail closed"
    anchor: co-1
  - id: co-2
    text: "canonical JSON is sorted, uses empty arrays rather than null, disables HTML escaping, and ends with one newline; no wall-clock or random value enters a deterministic identity or digest except a declared provenance stamp"
    anchor: co-2
  - id: co-3
    text: "every public verb preserves exit 0 for a clean result, 1 for a verdict failure, and 2 for an operational failure; report-only projections still exit 0 when they successfully report adverse facts"
    anchor: co-3
  - id: co-4
    text: "secrets and credential values never enter manifests, projections, prompts, events, receipts, proofs, logs, fixtures, or user interfaces; tests use hermetic Git, forge, process, and recorder fakes with no network"
    anchor: co-4
  - id: co-5
    text: "the adapters are official-CLI process adapters behind consumer-defined ports, use no cgo or in-process provider SDK, and cannot weaken isolation, capability, expansion, event, or receipt requirements for a particular harness"
    anchor: co-5
  - id: co-6
    text: "each child story is accepted before implementation, follows failing-test then minimal-code delivery, includes happy and negative unit, integration, and built-binary behavior coverage, and closes only with clean make verify and go test -race ./... output for its exact candidate"
    anchor: co-6
---
# Verdi-ATC prerequisite surfaces

## Problem

Verdi-ATC is a separate orchestrator. It may schedule work, own runway
worktrees, and project committed events, but it must not become a second owner
of Verdi lifecycle or context semantics. The ratified ATC design therefore
depends on machine surfaces that Verdi does not yet provide completely.
Without those surfaces, the daemon would have to scrape human text, accept an
unverified approval, trust primary-checkout behavior, or reconstruct execution
authority from agent conversation. Each fallback would defeat Verdi's
three-valued honesty and the sealed Flight Plan boundary.

## Outcome

Six independently landable stories provide the complete Stage 0.5 contract.
The boundary remains harness-neutral: Verdi compiles and verifies authority;
official Codex and Claude Code process adapters expose only what their
documented interfaces make observable; VATC durably records and projects those
events; and the external orchestrator consumes strict bytes rather than Go
packages or informal transcripts.

## AC-1

`vatc-machine-projections` supplies canonical matrix and explicit journey JSON.

## AC-2

`vatc-forge-countersign` maps current authenticated forge approvals to the
existing countersign obligation without changing the candidate tree.

## AC-3

`vatc-build-worktree-contract` proves the existing build lifecycle from an
ATC-owned non-primary worktree and repairs only a witnessed gap.

## AC-4

`sealed-codex-execution` supplies the first authoritative sealed execution
adapter and the scoped context boundary. It also implements
`spec/context-integrity-v2#ac-4`.

## AC-5

`context-receipts-review` authenticates builder and fresh-review receipts. It
also implements `spec/context-integrity-v2#ac-5`.

## AC-6

`sealed-claude-vatc-events` adds Claude Code parity and fixes the shared VATC
event profile for both adapters.

## Decisions

### DC-1

Verdi owns and verifies the authority semantics; ATC consumes strict process
surfaces and does not import or reproduce the Verdi implementation.

### DC-2

This narrow feature owns all six prerequisites. U4 and U5 additionally fulfill
the existing Context Integrity v2 execution and receipt stubs.

### DC-3

Each accepted story fixes its exact public wires. Implementation cannot invent
another name or interpretation.

### DC-4

Only a live, authenticated, role-correct, fresh, exact-candidate forge approval
satisfies a countersign; the reviewed tree receives no countersign file.

### DC-5

The proof boundary ends at project-controlled context and provider-observable
activity. Opaque vendor inputs and hidden reasoning remain outside the claim.

### DC-6

VATC acknowledgment follows strict decode, identity and sequence validation,
redaction, durable append, and projection. A broken chain denies authority.

## Constraints

### CO-1

All human and machine inputs strict-decode and unknown vocabulary fails closed.

### CO-2

Deterministic proof outputs use canonical JSON and declared inputs only.

### CO-3

Every public verb preserves Verdi's 0 clean, 1 verdict, 2 operational exits.

### CO-4

Secrets never enter durable artifacts or fixtures; all integration tests are
hermetic and network-free.

### CO-5

Official-CLI adapters sit behind consumer ports and cannot weaken the shared
authority contract.

### CO-6

Each accepted child story follows TDD and supplies exact-candidate full-gate
and race evidence.

## Authority boundary

The source promotion from the ratified Verdi-ATC repository is documented in
`docs/superpowers/reports/2026-08-25-verdi-atc-prerequisites-source-coverage.md`.
That report is a coverage witness, not an alternate source of implementation
semantics. If it disagrees with this accepted feature or a child story, the
canonical Verdi spec wins and the discrepancy is a blocking losslessness
failure.

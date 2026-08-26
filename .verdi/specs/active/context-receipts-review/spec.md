---
id: spec/context-receipts-review
kind: spec
title: "Authenticated context receipts and fresh review"
owners: [platform-team]
class: story
story: jira:VERDI-ATC-5
problem: { text: "a sealed run still cannot support authoritative closure unless Verdi can authenticate who ran it, bind its outputs and evidence to the complete context and event history, and prove that R0 and R2 were fresh reviews rather than continuations of the builder conversation", anchor: problem }
outcome: { text: "Verdi emits and verifies canonical builder and reviewer receipts from trusted managed-runner evidence and launches fresh sealed reviews whose packets are exact, minimal, builder-chat-free, and independently receipt-bound", anchor: outcome }
acceptance_criteria:
  - id: ac-1
    text: "verdi.context-receipt/v1 strict-decodes and canonically binds receipt role and authority, manifest and dispatch digests, ATC runway and execution-workspace identities, input and output commits and trees, worktree cleanliness, the ordered revision-segment execution-event-chain root through the acknowledged execution-result plus execution-terminal manifest/source/VATC-global sequences, expansion ledger, obligations, evidence commands and results, runner principal resolution, adapter identity, review inputs, review-of link, and its own digest; its subsequently acknowledged receipt event is outside that self-digest boundary"
    evidence: [static, behavioral]
    anchor: ac-1
  - id: ac-2
    text: "verdi context receipt verify recomputes every available digest and continuity fact, authenticates the configured trusted managed-runner principal, rejects stale, incomplete, wrong-tree, unsigned, untrusted, discontinuous, or malformed receipts, and renders unsigned local receipts only as visibly advisory"
    evidence: [static, behavioral]
    anchor: ac-2
  - id: ac-3
    text: "R0 and R2 each run through the sealed execution service with a newly compiled minimum review packet containing the accepted spec, exact current diff, evidence bundle, builder receipt, and review policy but excluding builder conversation, personal or global memory, unrelated context, and prior reviewer conversation"
    evidence: [behavioral]
    anchor: ac-3
  - id: ac-4
    text: "R0 and R2 bind the configured reviewer lane, adapter, model, version, and isolated profile identity while using distinct sessions and fresh packets; R2 additionally receives the accepted adjudication record and current candidate evidence, and neither review can inherit unrecorded R0 or builder context"
    evidence: [behavioral]
    anchor: ac-4
links:
  - { type: implements, ref: "spec/verdi-atc-prerequisites#ac-5" }
  - { type: implements, ref: "spec/context-integrity-v2#ac-5" }
decisions:
  - id: dc-1
    text: "receipt production is an internal terminal operation of sealed execution and sealed review, not a public mint command; the only public receipt operation is verdi context receipt verify --request <path|-> [--out <path>] over strict verdi.context-receipt-verify-request/v1"
    anchor: dc-1
  - id: dc-2
    text: "verification returns canonical verdi.context-receipt-verdict/v1 with proven, violated, or unproven authority, stable findings and witnesses, the recomputed receipt digest, and the exact repository, manifest, dispatch, event, runner, and review operands evaluated"
    anchor: dc-2
  - id: dc-3
    text: "hashes prove byte integrity but not runner identity or isolation; only the configured governance-principal resolver over trusted managed-runner or CI evidence can make a receipt authoritative"
    anchor: dc-3
  - id: dc-4
    text: "a review receipt names exactly one builder receipt digest in review_of and binds the complete review packet inventory; every receipt binds the canonical ordered revision_segments array and its event_chain_root through execution-result, including terminal_manifest_revision, terminal_source_sequence, and terminal_global_sequence; after finalization VATC durably acknowledges a receipt event carrying that receipt digest, and automated gates require the separate receipt_event_ack without including it in the receipt it acknowledges; builder and reviewer receipts form a chain, not a shared session transcript"
    anchor: dc-4
  - id: dc-5
    text: "R0 and R2 may retain the same configured reviewer identity but are separate executions with separate manifests, dispatches, sessions, event roots, and receipts; freshness is proven from those bytes rather than asserted by the orchestrator"
    anchor: dc-5
constraints:
  - id: co-1
    text: "unknown or duplicate fields, trailing data, null collections, unknown receipt roles or authority states, unordered or duplicated evidence and expansion identities, and self-digest mismatch fail closed"
    anchor: co-1
  - id: co-2
    text: "receipt verification is read-only, deterministic over declared inputs, and cannot contact a live provider; authentication adapters use hermetic trusted-runner evidence in tests"
    anchor: co-2
  - id: co-3
    text: "an absent authenticated signer, opaque isolation fact, unavailable event, or telemetry gap is never inferred from a successful process exit and can never yield an authoritative receipt"
    anchor: co-3
---
# Authenticated context receipts and fresh review

## Problem

Content hashes alone cannot say who ran a command, whether its environment was
isolated, or whether a reviewer saw the builder's conversation. Closure needs a
chain that binds both bytes and authenticated execution facts.

## Outcome

Builder and reviewer receipts share one strict schema and verifier. Fresh
review is an execution property proven by new sealed inputs, sessions, events,
and trusted-runner identity, not a claim made in review prose.

## AC-1

The receipt inventory is closed and digest-bound. Missing expansions, evidence,
events, or repository identities are visible failures rather than optional
fields that a favorable run may omit.

The `event_chain_root` is the canonical digest of the complete ordered
`revision_segments` execution array defined by `spec/sealed-codex-execution`,
through the acknowledged `execution-result`; it is not the final revision's
event root. `terminal_manifest_revision` selects the last array entry, while
`terminal_source_sequence` and `terminal_global_sequence` equal its execution
boundary acknowledgments. Verification recomputes the array root and proves
every predecessor bridge before considering the receipt internally complete.

Receipt finalization is deliberately acyclic. The receipt self-digest covers
the execution array but not the later event that announces that digest. The
sealed producer emits that `receipt` event only after finalization, carrying
the canonical receipt bytes through the strict inline-or-segment detail union.
VATC resolves, strict-decodes, canonicalizes, digest-checks, and atomically
persists those bytes with the event before returning a strict
`receipt_event_ack` that binds the receipt digest and exact source/global
recorder position. Automated ATC authority requires both a proven receipt and
this durable acknowledgment. Missing, inaccessible, non-canonical, or
mismatched event detail cannot be acknowledged; failure cannot alter the
receipt bytes and leaves the run non-authoritative for automation.

## AC-2

The verifier recomputes content claims and delegates actor authentication to
the shared governance kernel. Advisory receipts remain useful diagnostics but
cannot satisfy an authoritative closure obligation.

## AC-3

Each review packet is compiled from canonical sources at the candidate under
review. It never imports a conversation transcript.

## AC-4

Reviewer identity continuity and context freshness are both required. Reusing
the intended reviewer configuration does not permit reusing a session or
unrecorded context.

## Decisions

### DC-1

Trusted terminal paths produce receipts; public callers can verify but cannot
mint them.

### DC-2

Verification returns one canonical three-valued verdict with recomputed
operands and witnesses.

### DC-3

Hashes prove bytes, while the governance kernel proves trusted runner identity.

### DC-4

A review receipt binds exactly one builder receipt, its complete packet, and
the full ordered execution-event chain through `execution-result`, rather than
only the final manifest subchain. Its receipt event is separately acknowledged
after the immutable receipt exists.

### DC-5

R0 and R2 retain reviewer configuration but use distinct sealed executions.

## Constraints

### CO-1

The receipt and verdict schemas are closed, non-null, ordered, and self-digest
checked.

### CO-2

Verification is read-only and deterministic; authentication tests are
hermetic.

### CO-3

Successful provider exit cannot substitute for runner, isolation, event, or
expansion proof.

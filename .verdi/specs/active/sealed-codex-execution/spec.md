---
id: spec/sealed-codex-execution
kind: spec
title: "Sealed Codex execution"
owners: [platform-team]
class: story
story: jira:VERDI-ATC-4
problem: { text: "the context compiler can describe project authority but Verdi cannot yet launch Codex so that the compiled minimum context, isolated project profile, worktree, capabilities, expansions, and observable event sequence remain one enforceable and resumable authority boundary", anchor: problem }
outcome: { text: "Verdi launches or resumes Codex through a harness-neutral sealed-execution core, exposes a flight-scoped context MCP server, streams every provider-observable action to the required recorder, and marks any execution with unproven isolation, invalidated authority, or discontinuous context or telemetry as non-authoritative", anchor: outcome }
acceptance_criteria:
  - id: ac-1
    text: "verdi context execution strict-decodes a sealed request, proves the exact accepted authority and clean ATC-runway input commit and tree, materializes the provider's detached child through the execution-workspace component under data/execution, proves the project-only profile, immutable instruction projection, approved grants, conflict verdict, recorder binding, and declared opaque vendor boundary before an authoritative Codex process starts"
    evidence: [static, behavioral]
    anchor: ac-1
  - id: ac-2
    text: "the flight-scoped context MCP server exposes only get_flight_plan and request_context; an approved in-scope request emits a digest-bound child manifest and expansion event, an out-of-scope read is denied and recorded, and any authority, capability, profile, or declared-scope change invalidates the epoch rather than expanding it"
    evidence: [static, behavioral]
    anchor: ac-2
  - id: ac-3
    text: "Codex observable messages, summaries, tool calls and results, reads, writes, denials, commands, tests, Git changes, context requests, adapter lifecycle, execution-result, and post-finalization receipt events are normalized and streamed with complete identity and monotonic sequence; recorder loss, rejection, redaction failure, a gap, or missing receipt-event acknowledgment prevents automated authority"
    evidence: [behavioral]
    anchor: ac-3
  - id: ac-4
    text: "resume is allowed only through the resume arm's canonical continuity record when manifest revision, profile, ATC runway, execution-workspace identity, candidate chain, capabilities, authority, ordered revision-event chain, VATC acknowledgment, expansion ledger, and prior adapter session all reverify; otherwise partial output remains inspectable but non-authoritative, and resume and replacement dispatch cannot coexist for one epoch"
    evidence: [behavioral]
    anchor: ac-4
links:
  - { type: implements, ref: "spec/verdi-atc-prerequisites#ac-4" }
  - { type: implements, ref: "spec/context-integrity-v2#ac-4" }
decisions:
  - id: dc-1
    text: "the public execution invocation is verdi context execution --request <path|-> [--out <path>]; strict verdi.context-execution-request/v1 has a common envelope plus exactly one start or resume arm, the resume arm embeds and digest-binds one canonical verdi.execution-continuity/v1 record, stdout or --out receives one canonical verdi.context-execution-result/v1 only after the execution-result and receipt events are durably acknowledged and includes the execution receipt plus receipt_event_ack, and an interrupt requests a normalized stop rather than minting a separate stop command"
    anchor: dc-1
  - id: dc-2
    text: "the scoped MCP invocation is verdi context mcp --request <path|-> and exposes exactly get_flight_plan with an empty argument object and request_context with a required ref plus purpose; MCP responses are inspection data, never a second instruction channel, while the adapter injects the sealed instruction projection through its one declared authority channel; the server has no generic artifact-read, shell, mutation, or receipt-minting tool"
    anchor: dc-2
  - id: dc-3
    text: "the deterministic Verdi manifest and instruction projection remain separate from the runtime dispatch identity; the execution request binds flight, lane, epoch, manifest revision, session, ATC runway, input commit and tree, execution-workspace request, adapter name and version, grants, recorder endpoint, and both projection digests"
    anchor: dc-3
  - id: dc-4
    text: "the first adapter value is codex; it invokes the official pinned Codex CLI as an argument vector behind a consumer-defined process port, supplies only the sealed projection and classified data, uses an isolated project profile, and does not import provider packages or trust provider self-attestation"
    anchor: dc-4
  - id: dc-5
    text: "an advisory execution uses the same schema and records authority: advisory plus explicit unproven prerequisites; no fallback, flag, adapter response, or caller assertion can upgrade it to authoritative"
    anchor: dc-5
  - id: dc-6
    text: "the ATC runway is the clean controller checkout and execution-workspace is the sole provider materializer: it creates a detached data/execution child at the runway's exact input commit, the provider commits only in that child, and an authoritative result returns a clean descendant output commit and tree; after receipt durability ATC may fast-forward its still-clean still-at-input feature branch to that output, must quarantine any mismatch instead of overwriting, and requests execution-workspace release only after handback is durably acknowledged"
    anchor: dc-6
  - id: dc-7
    text: "verdi.execution-continuity/v1 binds execution identity, ATC runway input, execution-workspace request and id, profile/grant/authority digests, current candidate, current manifest and projection, ordered revision-event segments, event-chain and expansion roots, terminal source and VATC-global acknowledgments, recorder checkpoint, adapter session reference, and its own digest; resume re-queries the recorder and repository and accepts no caller summary in place of those facts"
    anchor: dc-7
constraints:
  - id: co-1
    text: "personal and global configuration, memory, prior conversations, unrelated specifications, design history, and unratified documents are excluded and not read or hashed merely to describe their exclusion; unavoidable vendor inputs are identity-only opaque rows"
    anchor: co-1
  - id: co-2
    text: "the instruction projection is the only project-authority channel; repository and corpus content is provenance-wrapped data, and imperative text found there cannot become an instruction"
    anchor: co-2
  - id: co-3
    text: "authoritative prerequisite failure exits 1, malformed requests and process or storage failures exit 2, and every result binds the exact output commit, tree, clean status, execution-event-chain root, final manifest revision, receipt digest, receipt-event acknowledgment, and authority state"
    anchor: co-3
  - id: co-4
    text: "process, MCP, recorder, isolation, capability, expansion, invalidation, interruption, and resume tests are hermetic and include negative paths; no test contacts Codex or the network"
    anchor: co-4
---
# Sealed Codex execution

## Problem

A deterministic manifest is necessary but not sufficient. If Codex starts with
ambient configuration, can read beyond declared context, writes without a
grant, or loses recorder continuity, the resulting code cannot honestly be
bound to the accepted specification.

## Outcome

One harness-neutral service owns preparation, start, resume, interruption, and
continuity. The Codex process adapter is replaceable, but it cannot change the
authority model. VATC receives the complete activity that the adapter can
observe before an authoritative result is accepted.

## AC-1

Preparation fails closed before the provider starts. The result distinguishes
proven authoritative execution, violated execution with witnesses, and
advisory execution whose missing proofs are explicit.

## AC-2

The two scoped tools are the only context entry points available to the flight.
Expansion is a manifest transition, not an invisible read.

## AC-3

Events are not considered recorded when merely emitted. The configured sink
must acknowledge the bound sequence after durable projection, or authoritative
execution stops.

## AC-4

Resume reconstructs from canonical continuity records and fresh Verdi queries,
never from chat history. Any missing invariant prevents authoritative resume.

## Decisions

### DC-1

One strict `context execution` request union owns start and resume; process
interruption requests the normalized stop path.

### DC-2

The scoped MCP server exposes exactly `get_flight_plan` and `request_context`.
Its response is inspection data; the adapter separately places the already
sealed projection in the one declared authority channel.

### DC-3

Deterministic context and runtime dispatch remain separate digest-bound layers.

### DC-4

The official pinned Codex CLI is the first process adapter to the shared core.

### DC-5

Advisory runs are explicit and cannot be upgraded by a caller assertion.

### DC-6

The `.vatc` runway is the controller checkout; the accepted
execution-workspace component exclusively materializes the detached provider
child under `data/execution`. Clean descendant commits return by a guarded ATC
fast-forward after receipt durability.

### DC-7

Resume embeds one canonical continuity record and reverifies it against the
recorder, repository, isolated profile, and execution-workspace state.

## Constraints

### CO-1

Ambient and excluded context is not read merely to inventory its exclusion.

### CO-2

Only the projection is project authority; repository content is wrapped data.

### CO-3

Results preserve exit classes and bind exact repository, event, and authority
facts.

### CO-4

Provider, isolation, MCP, recorder, expansion, and resume tests are hermetic.

## Execution request union

The common `verdi.context-execution-request/v1` envelope binds:

```text
action, flight, lane, epoch, manifest_revision, session
atc_runway, input_commit, input_tree
manifest, manifest_digest, instruction_projection, projection_digest
execution_workspace_request, adapter, adapter_version
profile, grants, authority_verdict, recorder_endpoint
start: StartArm | absent
resume: ResumeArm | absent
```

`action: start` requires `start: { expected_source_sequence: 1 }` and forbids
`resume`. `action: resume` requires `resume: { continuity,
continuity_digest }` and forbids `start`. A different or missing arm, a digest
mismatch, or an unknown field is operational failure before provider launch.

`verdi.execution-continuity/v1` contains:

```text
schema, flight, lane, epoch, session, adapter, adapter_version
atc_runway, input_commit, input_tree, current_commit, current_tree
execution_workspace_id, execution_workspace_request_digest
profile_digest, grant_digest, authority_verdict_digest
current_manifest_revision, current_manifest_digest, projection_digest
revision_segments[], event_chain_root, expansion_ledger_root
terminal_source_sequence, terminal_global_sequence
recorder_checkpoint_digest, adapter_session_ref, digest
```

`revision_segments` has the exact shape defined by
the Event continuity contract below. The adapter session reference is a non-secret
identity whose corresponding provider state remains inside the isolated
profile; no credential or resume token enters this record. Resume recomputes
the record digest, verifies the recorder checkpoint and every segment/global
acknowledgment, verifies the workspace sidecars and current Git facts, and
recompiles authority before reconnecting the official CLI session.

## Event continuity contract

U4 owns the base `verdi.context-event/v1` envelope used by all sealed adapters:

```text
schema, source_sequence, flight, lane, epoch
manifest_revision, manifest_digest, session
atc_runway, execution_workspace_id, candidate_commit, candidate_tree
adapter, adapter_version, occurred_at, kind, payload_schema, payload
prior_event_digest, prior_revision, event_digest
```

`occurred_at` is a declared UTC RFC3339Nano provenance stamp. The event digest
is computed with `event_digest` blank over canonical envelope bytes, including
the already-redacted payload and predecessor fields. Payload schemas remain a
closed registry; U6 fixes the complete VATC registry and Claude parity without
changing this envelope or its ordering semantics.

Within one manifest revision, `source_sequence` starts at one. Sequence one
has empty `prior_event_digest`; later events require the immediately preceding
event digest. Revision zero has no `prior_revision`. When an approved expansion
creates revision N+1, `child-manifest` is the last event using revision N and
sequence one of revision N+1 requires:

```text
prior_revision: {
  manifest_revision, manifest_digest, event_root,
  terminal_source_sequence, terminal_global_sequence
}
```

Those values must equal revision N's acknowledged `child-manifest`. VATC's
acknowledgment supplies the separate global sequence after durable projection;
that number is not encoded into the event digest being acknowledged.

`child-manifest` closes only an expansion revision: it is the last event under
revision N, and sequence one under revision N+1 carries its acknowledged
bridge. A final revision with no further expansion has no synthetic
`child-manifest`. Its execution boundary is the acknowledged
`execution-result` event, emitted after provider and adapter-stop activity.

One canonical `verdi.context-event-revision/v1` record contains manifest
revision/digest, first global sequence, execution-terminal global sequence,
execution-terminal source sequence, terminal kind (`child-manifest` or
`execution-result`), and that terminal event digest as `event_root`. Non-final
records must terminate in `child-manifest`; the final record must terminate in
`execution-result`. Revision records sort by revision and must form the
predecessor chain. `event_chain_root` is the canonical digest of this complete
ordered execution array through `execution-result`.

The `execution-result` payload binds the result facts but never
`event_chain_root` or a digest of the finalized result object. After VATC
acknowledges that event, Verdi may construct the canonical execution result
and receipt that bind the now-final execution array and root. Only then does
the producer emit `receipt` with the receipt digest as the next source event.
That post-finalization event is deliberately outside the root carried by the
receipt, so no object includes its own digest transitively. Its closed
`verdi.receipt-event-ack/v1` binds schema, flight/lane/epoch/session, manifest
revision, event kind `receipt`, source sequence, event digest, VATC global
sequence, and receipt digest.
VATC persists the receipt and acknowledgment before returning it; an
unacknowledged receipt event leaves the receipt inspectable but cannot satisfy
an automated ATC gate. Later orchestration or closure records bind that
post-receipt acknowledgment rather than rewriting the immutable receipt.

Continuity records used before finalization bind the complete acknowledged
revision array available at their checkpoint. Final receipts bind the array
through `execution-result`, its root, and its execution-terminal source/global
values; neither a final-revision truncation nor a post-receipt acknowledgment
may be silently substituted.

## Runway and execution-workspace handback

The `execution_workspace_request` is the component's exact-commit detached
shape keyed by the flight/lane/epoch/session run identity and `input_commit`.
The provider never edits the `.vatc` runway. A successful execution result
binds the child workspace id/request digest, input commit/tree, clean output
commit/tree, and proof that output is a descendant of input.

ATC accepts handback only after the result, authoritative receipt, and matching
receipt-event acknowledgment are durable. It verifies that its feature-branch runway is clean and still exactly
at the input commit, then performs a Git fast-forward-only merge to the output
commit and verifies HEAD/tree. A changed runway, non-descendant result,
protected-spec change, dirty child, or verification mismatch is quarantined
and routed to the owner decision path; no reset, force update, patch copy, or
silent reconciliation is permitted. Execution-workspace release occurs only
after VATC durably records successful handback, or after an explicit abort
disposition that preserves the inspectable partial result.

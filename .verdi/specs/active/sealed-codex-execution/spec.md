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
    text: "verdi context execution strict-decodes a sealed request, proves the exact accepted authority, clean input commit and tree, isolated runway and project-only profile, immutable instruction projection, approved execution-workspace grants, conflict verdict, recorder binding, and declared opaque vendor boundary before an authoritative Codex process starts"
    evidence: [static, behavioral]
    anchor: ac-1
  - id: ac-2
    text: "the flight-scoped context MCP server exposes only get_flight_plan and request_context; an approved in-scope request emits a digest-bound child manifest and expansion event, an out-of-scope read is denied and recorded, and any authority, capability, profile, or declared-scope change invalidates the epoch rather than expanding it"
    evidence: [static, behavioral]
    anchor: ac-2
  - id: ac-3
    text: "Codex observable messages, summaries, tool calls and results, reads, writes, denials, commands, tests, Git changes, context requests, and adapter lifecycle events are normalized and streamed with complete identity and monotonic sequence; recorder loss, rejection, redaction failure, or a gap suspends or stops the producer and prevents an authoritative result"
    evidence: [behavioral]
    anchor: ac-3
  - id: ac-4
    text: "resume is allowed only when manifest revision, profile, runway, candidate chain, capabilities, authority, event sequence, expansion ledger, and prior session continuity all match; otherwise partial output remains inspectable but non-authoritative, and resume and replacement dispatch cannot coexist for one epoch"
    evidence: [behavioral]
    anchor: ac-4
links:
  - { type: implements, ref: "spec/verdi-atc-prerequisites#ac-4" }
  - { type: implements, ref: "spec/context-integrity-v2#ac-4" }
decisions:
  - id: dc-1
    text: "the public execution invocation is verdi context execution --request <path|-> [--out <path>]; the strict verdi.context-execution-request/v1 action union is start or resume, stdout or --out receives one canonical verdi.context-execution-result/v1 after the process ends, and an interrupt requests a normalized stop rather than minting a separate stop command"
    anchor: dc-1
  - id: dc-2
    text: "the scoped MCP invocation is verdi context mcp --request <path|-> and exposes exactly get_flight_plan with an empty argument object and request_context with a required ref plus purpose; MCP responses are inspection data, never a second instruction channel, while the adapter injects the sealed instruction projection through its one declared authority channel; the server has no generic artifact-read, shell, mutation, or receipt-minting tool"
    anchor: dc-2
  - id: dc-3
    text: "the deterministic Verdi manifest and instruction projection remain separate from the runtime dispatch identity; the execution request binds flight, lane, epoch, manifest revision, session, runway, input commit and tree, adapter name and version, grants, recorder endpoint, and both projection digests"
    anchor: dc-3
  - id: dc-4
    text: "the first adapter value is codex; it invokes the official pinned Codex CLI as an argument vector behind a consumer-defined process port, supplies only the sealed projection and classified data, uses an isolated project profile, and does not import provider packages or trust provider self-attestation"
    anchor: dc-4
  - id: dc-5
    text: "an advisory execution uses the same schema and records authority: advisory plus explicit unproven prerequisites; no fallback, flag, adapter response, or caller assertion can upgrade it to authoritative"
    anchor: dc-5
constraints:
  - id: co-1
    text: "personal and global configuration, memory, prior conversations, unrelated specifications, design history, and unratified documents are excluded and not read or hashed merely to describe their exclusion; unavoidable vendor inputs are identity-only opaque rows"
    anchor: co-1
  - id: co-2
    text: "the instruction projection is the only project-authority channel; repository and corpus content is provenance-wrapped data, and imperative text found there cannot become an instruction"
    anchor: co-2
  - id: co-3
    text: "authoritative prerequisite failure exits 1, malformed requests and process or storage failures exit 2, and every result binds the exact output commit, tree, clean status, event root, final manifest revision, and authority state"
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

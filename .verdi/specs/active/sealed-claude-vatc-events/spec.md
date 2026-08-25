---
id: spec/sealed-claude-vatc-events
kind: spec
title: "Sealed Claude Code execution and VATC events"
owners: [platform-team]
class: story
story: jira:VERDI-ATC-6
problem: { text: "Codex-only sealed execution cannot support the ratified two-lane ATC, and provider-specific logs cannot serve as central orchestration truth without one closed event identity, redaction, segmentation, acknowledgment, continuity, and telemetry-honesty contract", anchor: problem }
outcome: { text: "Claude Code satisfies the same sealed execution and receipt invariants as Codex, while both adapters normalize all provider-observable activity into one strict redacted event stream that VATC can durably acknowledge, replay, and project without claiming secrets or hidden reasoning", anchor: outcome }
acceptance_criteria:
  - id: ac-1
    text: "the claude adapter value invokes the official pinned Claude Code CLI through the existing sealed-execution process port and proves the same isolated project profile, runway, immutable authority, capability, expansion, invalidation, interruption, resume, recorder, advisory, and receipt behavior required of codex"
    evidence: [static, behavioral]
    anchor: ac-1
  - id: ac-2
    text: "verdi.context-event/v1 binds source sequence, flight, lane, epoch, manifest revision and digest, session, runway, candidate commit and tree, adapter and version, declared occurrence stamp, closed event kind, redacted payload or segment reference, prior event digest, event digest, and segment digest into an idempotent execution-local chain"
    evidence: [static, behavioral]
    anchor: ac-2
  - id: ac-3
    text: "the closed event-kind vocabulary covers flight-plan and projection delivery, provider-observable messages and summaries, tool calls and results, reads, writes and denials, context and claim activity, commands, tests, resources and timeouts, Git and forge changes, gates and witnesses, deviations and adjudication, execution results and receipts, retry, resume, suspension, telemetry gaps, and adapter lifecycle without treating hidden reasoning as observable"
    evidence: [static, behavioral]
    anchor: ac-3
  - id: ac-4
    text: "secret-bearing values are rejected or redacted before emission, large payloads use redacted digest-bound segments, replay is idempotent, and duplicate conflict, sequence gap, stale identity or revision, invalid kind, failed redaction, sink discontinuity, or unavailable durable append prevents an authoritative result and receipt"
    evidence: [behavioral]
    anchor: ac-4
links:
  - { type: implements, ref: "spec/verdi-atc-prerequisites#ac-6" }
  - { type: depends-on, ref: spec/sealed-codex-execution }
  - { type: depends-on, ref: spec/context-receipts-review }
decisions:
  - id: dc-1
    text: "Claude Code uses the same verdi context execution and context MCP surfaces, request and result schemas, service core, event sink, and receipt verifier as Codex; adapter is a closed enum of codex or claude and no Claude-only authority shortcut exists"
    anchor: dc-1
  - id: dc-2
    text: "the exact event kinds are flight-plan, instruction-projection, child-manifest, prompt, provider-message, provider-summary, tool-call, tool-result, read, write, edit-denied, context-request, context-decision, claim-request, claim-decision, claim-wait, claim-release, command, test, resource, timeout, git-status, git-diff, git-commit, forge-change, gate-input, gate-verdict, witness, flight-plan-deviation, adjudication, execution-result, receipt, retry, resume, suspension, telemetry-gap, adapter-start, adapter-stop, and adapter-error"
    anchor: dc-2
  - id: dc-3
    text: "source_sequence is monotonic within the complete flight, lane, epoch, manifest-revision, session, and adapter identity; VATC allocates its separate global committed sequence only after durable ingestion, and Verdi records the returned acknowledgment without confusing the two orders"
    anchor: dc-3
  - id: dc-4
    text: "a payload larger than the configured inline ceiling is redacted then stored as a segment whose media type, byte count, digest, redaction profile, and storage reference are event-bound; the event carries no unredacted duplicate"
    anchor: dc-4
  - id: dc-5
    text: "provider-exposed reasoning summaries may appear only as provider-summary operator telemetry, are never obligation evidence or a gate input, and telemetry-unavailable is represented by an explicit telemetry-gap event rather than silence"
    anchor: dc-5
constraints:
  - id: co-1
    text: "events and segments never contain credential values, secret environment values, authentication tokens, provider session secrets, or raw values classified as secrets by policy; a redactor that cannot decide safely refuses emission"
    anchor: co-1
  - id: co-2
    text: "unknown adapter values, event kinds, payload schemas, fields, redaction profiles, segment media types, and sequence or identity transitions fail closed"
    anchor: co-2
  - id: co-3
    text: "tests use canned Claude and Codex process streams plus a failing recorder sink, cover every event kind and discontinuity class, and never invoke a live provider or network service"
    anchor: co-3
---
# Sealed Claude Code execution and VATC events

## Problem

The two adapters expose different process streams, but VATC needs one honest
record. Treating provider logs as equivalent without normalization would lose
identity, ordering, redaction, or unavailable-telemetry facts.

## Outcome

Claude Code becomes a second adapter to the same sealed core. A closed event
profile forms the transport contract between Verdi and VATC and the event root
becomes a receipt operand.

## AC-1

Every invariant proven for Codex has a Claude Code parity fixture. A provider
limitation can make a run advisory; it cannot relax the invariant.

## AC-2

The event chain separates provider/execution-local source order from VATC's
global committed order. Full identity on every event prevents cross-flight or
stale-revision replay.

## AC-3

The vocabulary is complete for activity an adapter or orchestrator can
observe. Hidden model state remains outside the claim boundary.

## AC-4

Redaction precedes both durable segment storage and event acknowledgment.
Continuity failure keeps partial output inspectable while denying authority.

## Decisions

### DC-1

Claude and Codex share the execution, context, event, and receipt surfaces.

### DC-2

The closed event vocabulary is exactly the list declared in frontmatter.

### DC-3

Execution-local source order and VATC global committed order remain distinct.

### DC-4

Large redacted payloads move to digest-bound segments with no inline duplicate.

### DC-5

Provider summaries are operator telemetry only; missing telemetry is explicit.

## Constraints

### CO-1

Secrets are rejected or redacted before emission or segment persistence.

### CO-2

Unknown adapter, event, payload, segment, redaction, and continuity vocabulary
fails closed.

### CO-3

All adapters and recorder discontinuities are tested with canned local fakes.

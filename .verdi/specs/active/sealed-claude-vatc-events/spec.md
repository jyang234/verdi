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
    text: "the U4-owned verdi.context-event/v1 envelope binds source sequence, flight, lane, epoch, manifest revision and digest, session, ATC runway, execution-workspace identity, candidate commit and tree, adapter and version, declared occurrence stamp, closed payload discriminator and body, prior-event digest, optional prior-revision bridge, event digest, and VATC acknowledgment into an idempotent revision-local chain and one complete cross-revision execution chain"
    evidence: [static, behavioral]
    anchor: ac-2
  - id: ac-3
    text: "the closed event-kind vocabulary covers flight-plan and projection delivery, provider-observable messages and summaries, tool calls and results, reads, writes and denials, context and claim activity, commands, tests, resources and timeouts, Git and forge changes, gates and witnesses, deviations and adjudication, execution results and receipts, retry, resume, suspension, telemetry gaps, and adapter lifecycle without treating hidden reasoning as observable"
    evidence: [static, behavioral]
    anchor: ac-3
  - id: ac-4
    text: "secret-bearing values are rejected or redacted before emission, each event kind strict-decodes through its fixed verdi.context-event-payload/<kind>/v1 body, large variable detail uses the closed redacted inline-or-segment union, replay is idempotent, and duplicate conflict, sequence or revision-bridge gap, stale identity, invalid kind or payload, failed redaction, sink discontinuity, or unavailable durable append prevents an authoritative result and receipt"
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
    text: "the U4 source-sequence, expansion-revision bridge, final execution-result boundary, and post-finalization receipt-event contract remain unchanged for Claude: source_sequence starts at one and is monotonic within one flight/lane/epoch/session/adapter/manifest-revision identity; child-manifest closes only a revision followed by an approved expansion; VATC allocates its separate never-resetting global committed sequence only after durable ingestion; receipts digest the execution revision segments through execution-result; and the separately required receipt_event_ack cannot enter the receipt it acknowledges"
    anchor: dc-3
  - id: dc-4
    text: "each payload contains fixed routing and verdict fields plus only where declared an optional detail union; detail is exactly inline redacted canonical JSON or a segment reference, and detail larger than the configured inline ceiling is redacted then stored as a segment whose media type, byte count, digest, redaction profile, and storage reference are event-bound; fixed fields always remain inline and the event carries no unredacted duplicate"
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

Source order resets only at a manifest revision and bridges explicitly to the
prior revision; VATC global committed order never resets. Receipts bind both.

### DC-4

Only the declared variable `detail` moves to a digest-bound segment. Fixed
strict fields remain inline and no unredacted duplicate exists.

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

## Inherited event envelope

U6 consumes the exact `verdi.context-event/v1` envelope, sequence rules,
revision bridge, revision-segment record, and `event_chain_root` definition
owned by `spec/sealed-codex-execution`. It neither redefines nor resets them.
Every discriminator, payload schema, and payload body below occupies the U4
envelope's `kind`, `payload_schema`, and `payload` fields. This complete VATC
registry is a serial extension of U4's initially closed registry; an accepted
U6 replaces that registry as a whole without changing U4's identity or
continuity semantics.

## Payload and segment union

Every payload is a strict object whose required `schema` literal is
`verdi.context-event-payload/<kind>/v1`. The following table is exhaustive;
the listed fields are required and unknown fields fail closed. Arrays are
canonically sorted where the field is set-like and otherwise retain observed
order. `principal_resolution` and `verdict` reuse their existing closed Verdi
types.

| Kind | Required payload fields besides `schema` |
|---|---|
| `flight-plan` | `manifest_digest, projection_digest, dispatch_digest, detail` |
| `instruction-projection` | `manifest_digest, projection_digest, detail` |
| `child-manifest` | `request_id, parent_revision, parent_manifest_digest, child_revision, child_manifest_digest, expansion_digest` |
| `prompt` | `prompt_digest, detail` |
| `provider-message` | `message_id, role, message_digest, detail` |
| `provider-summary` | `summary_id, summary_digest, authority, detail` |
| `tool-call` | `call_id, tool_name, arguments_digest, detail` |
| `tool-result` | `call_id, tool_name, status, output_digest, detail` |
| `read` | `resource, classification, decision, content_digest, detail` |
| `write` | `path, claim_id, before_digest, after_digest, byte_count` |
| `edit-denied` | `operation, path, reason_code, witnesses` |
| `context-request` | `request_id, ref, purpose` |
| `context-decision` | `request_id, verdict, reason_code, parent_manifest_digest, child_manifest_digest, witnesses` |
| `claim-request` | `claim_id, paths, shared_resources` |
| `claim-decision` | `claim_id, verdict, reason_code, witnesses` |
| `claim-wait` | `claim_id, queue_position` |
| `claim-release` | `claim_id, paths, shared_resources` |
| `command` | `command_id, argv, working_directory, declared_environment_names, timeout_milliseconds` |
| `test` | `command_id, suite, exit_code, duration_milliseconds, verdict, output_digest, detail` |
| `resource` | `operation_id, cpu_milliseconds, peak_rss_bytes, read_bytes, write_bytes, availability` |
| `timeout` | `operation_id, timeout_milliseconds, reason_code` |
| `git-status` | `head, tree, branch, clean, entries_digest, detail` |
| `git-diff` | `base_commit, target_commit, diff_digest, detail` |
| `git-commit` | `commit, tree, parents, message_digest` |
| `forge-change` | `forge, repository, change_id, operation, subject_ref, candidate_sha, principal_resolution` |
| `gate-input` | `gate, subject, input_digests` |
| `gate-verdict` | `gate, subject, verdict, witnesses` |
| `witness` | `witness_kind, witness_digest, authority, detail` |
| `flight-plan-deviation` | `deviation_id, plan_digest, rule_id, operation, observed_digest, verdict, witnesses, detail` |
| `adjudication` | `finding_or_deviation_id, principal_resolution, decision, reason_digest, detail` |
| `execution-result` | `authority, input_commit, output_commit, output_tree, clean, manifest_digest, result_facts_digest` |
| `receipt` | `role, receipt_digest, authority, execution_event_chain_root` |
| `retry` | `reason_code, prior_session, next_session, continuity_digest` |
| `resume` | `continuity_digest, prior_session, current_session, manifest_digest, event_chain_root` |
| `suspension` | `reason_code, continuity_digest, event_chain_root` |
| `telemetry-gap` | `source, from_sequence, to_sequence, reason_code, availability` |
| `adapter-start` | `adapter, adapter_version, session, profile_digest, workspace_request_digest` |
| `adapter-stop` | `adapter, adapter_version, session, exit_code, reason_code` |
| `adapter-error` | `adapter, adapter_version, session, operation, reason_code, error_digest, detail` |

`result_facts_digest` is the digest of a strict
`verdi.context-execution-result-facts/v1` object containing the
`execution-result` row's authority, repository, cleanliness, and manifest
facts, with no event-chain root, finalized result digest, receipt digest, or
receipt acknowledgment. The event envelope and its acknowledgment establish
the execution cutoff; the later canonical execution result binds that cutoff.
The `receipt` row may therefore bind both the finalized receipt digest and the
already-fixed execution root without either value depending on the receipt
event's own digest.

`detail` is required only in the rows that list it and is exactly one of:

```text
{ mode: "inline", media_type, digest, redaction_profile, redacted_json }
{ mode: "segment", media_type, digest, redaction_profile, byte_count, reference }
```

The adapter first normalizes provider data, applies the named redaction
profile, and canonicalizes the redacted detail. If its byte length is at or
below the bound inline ceiling it uses `inline`; otherwise it durably stores
those same redacted canonical bytes and uses `segment`. The digest always
covers the redacted canonical bytes. `inline` forbids segment fields;
`segment` forbids `redacted_json`; fixed payload fields never migrate into the
segment. A secret-classified value that cannot be safely normalized causes an
`adapter-error` only if that error can itself be emitted without the value;
otherwise the producer stops and no authoritative receipt is possible.

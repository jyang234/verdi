---
id: obligation/sealed-codex-execution--ac-3--behavioral
kind: obligation
title: "Acknowledged execution completes through guarded runway handback"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Provider-observable messages, summaries, tool calls and results, reads, writes, denials, commands, tests, Git changes, context requests, adapter lifecycle, execution-result, and receipt activity form one complete, redacted, acknowledged event chain; only after the authoritative result, receipt, and matching receipt-event acknowledgment are durable may ATC reverify a clean child and clean runway still at the exact input, reject protected-spec changes, fast-forward only to a clean descendant, verify the resulting runway HEAD and tree, durably acknowledge handback, and request execution-workspace release."
  falsifier: "An observable activity class is omitted, secret-bearing detail survives redaction, source order or a revision bridge has a gap, VATC rejects or cannot durably append an event, receipt-event acknowledgment is absent, the runway is reset, forced, patched, advanced while dirty or changed, or advanced to a non-descendant, a protected-spec change, dirty child, or failed post-fast-forward HEAD or tree verification is not quarantined, workspace release precedes durable successful handback, or an abort disposition discards the inspectable partial result."
  scope: "Hermetic end-to-end completion over canned Codex streams, normalized U4 events, recorder acknowledgments, redaction, expansion revisions, execution-result and receipt finalization, fixture Git handback states including protected-spec changes, late child dirtiness, and post-fast-forward verification failure, quarantine, durable handback acknowledgment, abort disposition preserving inspectable partial output, and execution-workspace release ordering."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestContextExecutionCompletionContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestContextExecutionCompletionContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-codex-execution" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Acknowledged execution completes through guarded runway handback

CI job `verify` must record producer `go-test:cmd/verdi:TestContextExecutionCompletionContract_Behavioral` at the exact candidate commit.

The evidence must prove: Provider-observable messages, summaries, tool calls and results, reads, writes, denials, commands, tests, Git changes, context requests, adapter lifecycle, `execution-result`, and `receipt` activity form one complete, redacted, acknowledged event chain; only after the authoritative result, receipt, and matching receipt-event acknowledgment are durable may ATC reverify a clean child and clean runway still at the exact input, reject protected-spec changes, fast-forward only to a clean descendant, verify the resulting runway HEAD and tree, durably acknowledge handback, and request execution-workspace release.

It is falsified when: An observable activity class is omitted, secret-bearing detail survives redaction, source order or a revision bridge has a gap, VATC rejects or cannot durably append an event, receipt-event acknowledgment is absent, the runway is reset, forced, patched, advanced while dirty or changed, or advanced to a non-descendant, a protected-spec change, dirty child, or failed post-fast-forward HEAD or tree verification is not quarantined, workspace release precedes durable successful handback, or an abort disposition discards the inspectable partial result.

Scope: Hermetic end-to-end completion over canned Codex streams, normalized U4 events, recorder acknowledgments, redaction, expansion revisions, execution-result and receipt finalization, fixture Git handback states including protected-spec changes, late child dirtiness, and post-fast-forward verification failure, quarantine, durable handback acknowledgment, abort disposition preserving inspectable partial output, and execution-workspace release ordering.

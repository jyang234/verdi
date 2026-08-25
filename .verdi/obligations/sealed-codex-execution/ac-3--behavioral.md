---
id: obligation/sealed-codex-execution--ac-3--behavioral
kind: obligation
title: "Observable execution activity forms one acknowledged event chain"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "All provider-observable messages, summaries, tool activity, reads, writes, denials, commands, tests, Git changes, context activity, adapter lifecycle, execution-result, and receipt emission are normalized with complete identity and monotonic revision-local ordering before authoritative completion."
  falsifier: "An observable activity class is omitted, secret-bearing detail survives redaction, source order or a revision bridge has a gap, VATC rejects or cannot durably append an event, receipt-event acknowledgment is absent, or authority still resolves proven."
  scope: "Canned Codex process streams, normalized U4 event envelope, recorder acknowledgments, redaction outcomes, expansion revisions, execution-result cutoff, and post-finalization receipt event."
  producer: { kind: test, ref: "go-test:internal/sealedexec:TestContextEventContinuityContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/sealedexec:TestContextEventContinuityContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-codex-execution" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Observable execution activity forms one acknowledged event chain

CI job `verify` must record producer `go-test:internal/sealedexec:TestContextEventContinuityContract_Behavioral` at the exact candidate commit.

The evidence must prove: All provider-observable messages, summaries, tool activity, reads, writes, denials, commands, tests, Git changes, context activity, adapter lifecycle, execution-result, and receipt emission are normalized with complete identity and monotonic revision-local ordering before authoritative completion.

It is falsified when: An observable activity class is omitted, secret-bearing detail survives redaction, source order or a revision bridge has a gap, VATC rejects or cannot durably append an event, receipt-event acknowledgment is absent, or authority still resolves proven.

Scope: Canned Codex process streams, normalized U4 event envelope, recorder acknowledgments, redaction outcomes, expansion revisions, execution-result cutoff, and post-finalization receipt event.

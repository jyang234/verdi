---
id: obligation/sealed-claude-vatc-events--ac-2--static
kind: obligation
title: "The VATC event envelope binds complete identity and continuity"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "verdi.context-event/v1 strictly binds source order, flight, lane, epoch, manifest revision and digest, session, runway, execution workspace, candidate commit and tree, adapter and version, occurrence stamp, payload discriminator and body, predecessor event, optional prior revision, and self-digest, with acknowledgment modeled separately."
  falsifier: "An identity or predecessor operand is optional, source and VATC-global order are conflated, an event digest includes its later acknowledgment, a revision bridge can be malformed, or unknown identity transitions decode."
  scope: "Event envelope and acknowledgment types, strict codec, canonical digest calculation, revision-local sequence, prior-revision bridge, revision-segment array, and complete event-chain root."
  producer: { kind: test, ref: "go-test:internal/contextevent:TestContextEventEnvelopeContract_Static" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/contextevent:TestContextEventEnvelopeContract_Static in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-claude-vatc-events" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# The VATC event envelope binds complete identity and continuity

CI job `verify` must record producer `go-test:internal/contextevent:TestContextEventEnvelopeContract_Static` at the exact candidate commit.

The evidence must prove: verdi.context-event/v1 strictly binds source order, flight, lane, epoch, manifest revision and digest, session, runway, execution workspace, candidate commit and tree, adapter and version, occurrence stamp, payload discriminator and body, predecessor event, optional prior revision, and self-digest, with acknowledgment modeled separately.

It is falsified when: An identity or predecessor operand is optional, source and VATC-global order are conflated, an event digest includes its later acknowledgment, a revision bridge can be malformed, or unknown identity transitions decode.

Scope: Event envelope and acknowledgment types, strict codec, canonical digest calculation, revision-local sequence, prior-revision bridge, revision-segment array, and complete event-chain root.

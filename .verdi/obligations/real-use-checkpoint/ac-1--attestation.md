---
id: obligation/real-use-checkpoint--ac-1--attestation
kind: obligation
title: "The protocol was fixed before the first journey"
owners: [platform-team]
for_kind: attestation
quality:
  state: elaborated
  claim: "The owner attests that the committed protocol fixing each journey's start, end, permitted assistance, and duration measure, and naming the two changes, predates the first journey."
  falsifier: "The protocol was committed after the first journey began, or omits one of those fixed points."
  scope: "The protocol document and its commit, and the first journey's start."
  producer: { kind: authenticated-human, ref: "role:owner" }
  authoritative_source: { kind: governed-attestation, ref: "approval:owner" }
  freshness:
    invalidated_by: [spec, code]
    rule: "The owner re-attests on a running verdi serve at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/real-use-checkpoint" }
frozen: { at: 2026-09-29, commit: 7abe7c1f76958e2b0241c6ec5466f61e765264ee }
---
# The protocol was fixed before the first journey

The owner attests the protocol's commit and its contents before the first journey starts.

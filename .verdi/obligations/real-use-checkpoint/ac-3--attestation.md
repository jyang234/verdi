---
id: obligation/real-use-checkpoint--ac-3--attestation
kind: obligation
title: "The records report duration, independence, and every assist"
owners: [platform-team]
for_kind: attestation
quality:
  state: elaborated
  claim: "The owner attests that each journey's committed record reports its duration with no threshold chosen afterwards, whether the person finished independently, and every assist, and that the second journey finished without coaching from the development agent."
  falsifier: "A record omits its duration, independence, or an assist, applies a threshold chosen afterwards, or the second journey needed the development agent's coaching."
  scope: "The two journey records and their commits."
  producer: { kind: authenticated-human, ref: "role:owner" }
  authoritative_source: { kind: governed-attestation, ref: "approval:owner" }
  freshness:
    invalidated_by: [spec, code]
    rule: "The owner re-attests on a running verdi serve at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/real-use-checkpoint" }
frozen: { at: 2026-09-29, commit: 0ae55e5c2ce00a3348c60561c6adf470841f241f }
---
# The records report duration, independence, and every assist

The owner attests the records once both are committed.

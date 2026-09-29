---
id: obligation/readiness-page--ac-1--attestation
kind: obligation
title: "The owner accepts the readiness page's visual fidelity"
owners: [platform-team]
for_kind: attestation
quality:
  state: elaborated
  claim: "The owner judges, on a running verdi serve at the candidate commit, that the readiness page matches the handoff, except where the redesign's decisions differ."
  falsifier: "The owner finds a part of the page that departs from the handoff without a recorded decision, or the attestation is not bound to the candidate commit."
  scope: "The readiness page at each step, light and dark, at 1440 px and 320 px."
  producer: { kind: authenticated-human, ref: "role:owner" }
  authoritative_source: { kind: governed-attestation, ref: "approval:owner" }
  freshness:
    invalidated_by: [spec, code]
    rule: "The owner re-attests on a running verdi serve at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/readiness-page" }
frozen: { at: 2026-09-29, commit: 01c200e50eacb38505ba12850d187be361d1301a }
---
# The owner accepts the readiness page's visual fidelity

The owner inspects it on a running `verdi serve` at the candidate commit (plan: visual acceptance at the risk gate).
Until authenticated attestation lands (verification program W4), the evidence layer reports this obligation's source as
missing, which is disclosed, never a pass.

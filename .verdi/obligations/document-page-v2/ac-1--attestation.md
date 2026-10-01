---
id: obligation/document-page-v2--ac-1--attestation
kind: obligation
title: "The owner accepts the Document page's visual fidelity"
owners: [platform-team]
for_kind: attestation
quality:
  state: elaborated
  claim: "The owner judges, on a running verdi serve at the candidate commit, that the Document page matches the handoff, except where the redesign's decisions differ."
  falsifier: "The owner finds a part of the page that departs from the handoff without a recorded decision, or the attestation is not bound to the candidate commit."
  scope: "The Document page for a proposed and an accepted spec, light and dark, at 1440 px and 320 px."
  producer: { kind: authenticated-human, ref: "role:owner" }
  authoritative_source: { kind: governed-attestation, ref: "approval:owner" }
  freshness:
    invalidated_by: [spec, code]
    rule: "The owner re-attests on a running verdi serve at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/document-page-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# The owner accepts the Document page's visual fidelity

The owner inspects it on a running `verdi serve` at the candidate commit (plan: visual acceptance at the risk gate).
Until authenticated attestation lands (verification program W4), the evidence layer reports this obligation's source as
missing, which is disclosed, never a pass.

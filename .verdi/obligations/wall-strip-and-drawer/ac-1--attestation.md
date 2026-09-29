---
id: obligation/wall-strip-and-drawer--ac-1--attestation
kind: obligation
title: "The owner accepts the strip, Commit and push, and drawer's visual fidelity"
owners: [platform-team]
for_kind: attestation
quality:
  state: elaborated
  claim: "The owner judges, on a running verdi serve at the candidate commit, that the case-file strip, Commit and push with its popover, the readiness pill, the menu, and every drawer tab match the handoff, except where the redesign's decisions differ."
  falsifier: "The owner finds one of these elements departing from the handoff without a recorded decision, or the attestation is not bound to the candidate commit."
  scope: "The wall in authoring, review, and sealed modes, light and dark, at 1440 px and 320 px, every drawer tab."
  producer: { kind: authenticated-human, ref: "role:owner" }
  authoritative_source: { kind: governed-attestation, ref: "approval:owner" }
  freshness:
    invalidated_by: [spec, code]
    rule: "The owner re-attests on a running verdi serve at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer" }
frozen: { at: 2026-09-29, commit: d64d2d637fda0b28635c0760ac23a8d7093638fb }
---
# The owner accepts the strip, Commit and push, and drawer's visual fidelity

The owner inspects it on a running `verdi serve` at the candidate commit (plan: visual acceptance at the risk gate).
Until authenticated attestation lands (verification program W4), the evidence layer reports this obligation's source as
missing, which is disclosed, never a pass.

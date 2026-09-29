---
id: obligation/chrome-and-tokens--ac-1--attestation
kind: obligation
title: "The owner accepts the top bar's visual fidelity on a running workbench"
owners: [platform-team]
for_kind: attestation
quality:
  state: elaborated
  claim: "The owner judges, on a running verdi serve at the candidate commit, that the top bar on every workbench page matches the design handoff's layout, spacing, type, colour, and states, except where the redesign's decisions differ."
  falsifier: "The owner finds a workbench page whose top bar departs from the handoff without a recorded decision, or the attestation is not bound to the candidate commit."
  scope: "Every workbench page listed in ac-1, in light and dark mode, at 1440 px and 320 px."
  producer: { kind: authenticated-human, ref: "role:owner" }
  authoritative_source: { kind: governed-attestation, ref: "approval:owner" }
  freshness:
    invalidated_by: [spec, code]
    rule: "The owner re-attests on a running verdi serve at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/chrome-and-tokens" }
frozen: { at: 2026-09-29, commit: b9f225e713231477b90e4c86cdd3f038e16fb249 }
---
# The owner accepts the top bar's visual fidelity on a running workbench

The owner inspects the bar on a running `verdi serve` at the candidate commit (plan: visual acceptance at the risk
gate). Until authenticated attestation lands (verification program W4), the evidence layer reports this obligation's
source as missing, which is disclosed, never a pass.

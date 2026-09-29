---
id: obligation/real-use-checkpoint--ac-2--attestation
kind: obligation
title: "Two comparable real changes, the second new"
owners: [platform-team]
for_kind: attestation
quality:
  state: elaborated
  claim: "The owner attests that a person new to the store worked two comparable real changes on the redesigned workbench, each defining, agreeing on, and following a change, and that the second was a new change."
  falsifier: "Either journey was not a real change, skipped a phase, or the second replayed the first."
  scope: "The two journeys and the changes they carried."
  producer: { kind: authenticated-human, ref: "role:owner" }
  authoritative_source: { kind: governed-attestation, ref: "approval:owner" }
  freshness:
    invalidated_by: [spec, code]
    rule: "The owner re-attests on a running verdi serve at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/real-use-checkpoint" }
frozen: { at: 2026-09-29, commit: 7abe7c1f76958e2b0241c6ec5466f61e765264ee }
---
# Two comparable real changes, the second new

The owner attests the two journeys after the second ends.

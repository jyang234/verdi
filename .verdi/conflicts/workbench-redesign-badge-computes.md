---
id: conflict/workbench-redesign-badge-computes
kind: conflict
title: "The redesign draws case-file badges as chips in the case-file strip, not stamps on the lockup"
owners: [platform-team]
status: superseded
resolved_by: spec/workbench-redesign
links:
  - { type: challenges, ref: "spec/badge-computes#ac-5" }
  - { type: challenges, ref: "spec/badge-computes#dc-4" }
frozen: { at: 2026-09-29, commit: 6e571cc388f7c6ba31b7dc7590ca478e7d78e4cd }
---
# Conflict: The redesign draws case-file badges as chips in the case-file strip, not stamps on the lockup

## What is disputed

`spec/badge-computes` ac-5 and dc-4 draw case-file badges as stamps on the case-file lockup beside the class tag. The
adopted design removes the lockup and its stamps; badges and disclosures become chips in the case-file strip.

## Witness

The owner's workbench redesign handoff of 2026-09-18 (archive sha256
966ebcee92ec1cbf04243c26500fb132716a3b2b158fdd4451a2cd6f9d2978d1; per-file manifest in
`docs/superpowers/plans/2026-09-24-workbench-redesign.md`), which the owner adopted as the workbench's design on
2026-09-24 (decisions D-WR-1 to D-WR-12 in that plan), after the wave-3.5 pilot found readers could not tell what to do
next, why the steps come in their order, or which work was theirs (F-03 to F-05).

## Resolution

Closed-spec object supersession (`spec/verdi-evidence-model` §Challenging closed decisions). `spec/workbench-redesign`
dc-6 carries the `supersedes` edges to ac-5 and dc-4 and states the replacement. Kept and restated in dc-6: card badges
as compact chips in the receipt-row vocabulary, badges in every board mode, badges never blocking a write, and every
badge a button carrying `data-badge-source` and its derivation record. The closed spec, its closure record, and its
evidence are unchanged. This store has one maintainer, so the two-approval quorum is waived (03 step 3); the conflict is
still filed.

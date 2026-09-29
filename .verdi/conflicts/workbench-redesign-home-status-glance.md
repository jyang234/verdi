---
id: conflict/workbench-redesign-home-status-glance
kind: conflict
title: "The redesign merges home-status-glance's separate three-bucket glance into the index's four columns"
owners: [platform-team]
status: superseded
resolved_by: spec/workbench-redesign
links:
  - { type: challenges, ref: "spec/home-status-glance#ac-1" }
  - { type: challenges, ref: "spec/home-status-glance#ac-2" }
  - { type: challenges, ref: "spec/home-status-glance#ac-3" }
  - { type: challenges, ref: "spec/home-status-glance#dc-1" }
  - { type: challenges, ref: "spec/home-status-glance#dc-2" }
  - { type: challenges, ref: "spec/home-status-glance#dc-3" }
  - { type: challenges, ref: "spec/home-status-glance#dc-4" }
  - { type: challenges, ref: "spec/home-status-glance#dc-5" }
frozen: { at: 2026-09-29, commit: f5baa9c023861056ba2ea05fff17f64761a8897a }
---
# Conflict: The redesign merges home-status-glance's separate three-bucket glance into the index's four columns

## What is disputed

`spec/home-status-glance` adds a separate three-bucket glance (on the desk, in flight, settling) above an unchanged
Directory section, with lean entries (no source or in-review chip), its own `glance-group-*` and `glance-entry-*` test
ids, and the directory kept in the same place (ac-1 to ac-3, dc-1 to dc-5). The adopted design has no separate glance:
the glance and the directory are one set of four status columns whose cards carry the directory's chips, with the other
records in a strip below.

## Witness

The owner's workbench redesign handoff of 2026-09-18 (archive sha256
966ebcee92ec1cbf04243c26500fb132716a3b2b158fdd4451a2cd6f9d2978d1; per-file manifest in
`docs/superpowers/plans/2026-09-24-workbench-redesign.md`), which the owner adopted as the workbench's design on
2026-09-24 (decisions D-WR-1 to D-WR-12 in that plan), after the wave-3.5 pilot found readers could not tell what to do
next, why the steps come in their order, or which work was theirs (F-03 to F-05).

## Resolution

Closed-spec object supersession (`spec/verdi-evidence-model` §Challenging closed decisions). `spec/workbench-redesign`
dc-12 carries the `supersedes` edges to ac-1, ac-2, ac-3, and dc-1 to dc-5 and states the replacement. The glance's
intent is kept and restated in dc-12 and ac-7: actionable-first order, archived specs never leading the page (On the
shelf folds them into a collapsed list), every column always rendering its count and an explicit empty state, one index
computation per render, and nothing persisted. co-1 and co-2 are not challenged and stay in force. The closed spec, its
closure record, and its evidence are unchanged. This store has one maintainer, so the two-approval quorum is waived (03
step 3); the conflict is still filed.

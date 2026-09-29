---
id: conflict/workbench-redesign-workbench-legibility
kind: conflict
title: "The redesign's four index columns replace workbench-legibility dc-4's single trailing settling group"
owners: [platform-team]
status: superseded
resolved_by: spec/workbench-redesign
links:
  - { type: challenges, ref: "spec/workbench-legibility#dc-4" }
frozen: { at: 2026-09-29, commit: f5baa9c023861056ba2ea05fff17f64761a8897a }
---
# Conflict: The redesign's four index columns replace workbench-legibility dc-4's single trailing settling group

## What is disputed

`spec/workbench-legibility` dc-4 fixes the home page's leading taxonomy as three groups: drafts ('on the desk'), then
accepted-pending-build ('in flight'), then every remaining active status as one trailing 'settling' group. The adopted
design shows every spec in four status columns, so active components and terminal specs are separate columns, and the
index is no longer a list of groups above a directory.

## Witness

The owner's workbench redesign handoff of 2026-09-18 (archive sha256
966ebcee92ec1cbf04243c26500fb132716a3b2b158fdd4451a2cd6f9d2978d1; per-file manifest in
`docs/superpowers/plans/2026-09-24-workbench-redesign.md`), which the owner adopted as the workbench's design on
2026-09-24 (decisions D-WR-1 to D-WR-12 in that plan), after the wave-3.5 pilot found readers could not tell what to do
next, why the steps come in their order, or which work was theirs (F-03 to F-05).

## Resolution

Closed-spec object supersession (`spec/verdi-evidence-model` §Challenging closed decisions). `spec/workbench-redesign`
dc-12 carries the `supersedes` edge to dc-4 and states the replacement: the settling group becomes the Active components
and On the shelf columns, which are workbench-directory dc-2's own groups. dc-4's order, its status-only grouping, each
entry's status badge and working links, and its bar on evidence-bearing state are kept and restated in dc-12 and dc-5.
The closed spec, its closure record, and its evidence are unchanged. This store has one maintainer, so the two-approval
quorum is waived (03 step 3); the conflict is still filed.

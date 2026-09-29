---
id: conflict/workbench-redesign-case-file-flags
kind: conflict
title: "The redesign draws the case-file flags as chips in the case-file strip, not stamps on the case file"
owners: [platform-team]
status: superseded
resolved_by: spec/workbench-redesign
links:
  - { type: challenges, ref: "spec/case-file-flags#ac-1" }
  - { type: challenges, ref: "spec/case-file-flags#dc-3" }
  - { type: challenges, ref: "spec/case-file-flags#dc-4" }
frozen: { at: 2026-09-29, commit: 6e571cc388f7c6ba31b7dc7590ca478e7d78e4cd }
---
# Conflict: The redesign draws the case-file flags as chips in the case-file strip, not stamps on the case file

## What is disputed

`spec/case-file-flags` ac-1, dc-3, and dc-4 render spec-stale and pending-supersession as stamps on the case file, fix
their placement and register, and render a disclosed-unproven value as a notice line. The adopted design removes the
case-file stamps; flags and disclosures become chips in the case-file strip.

## Witness

The owner's workbench redesign handoff of 2026-09-18 (archive sha256
966ebcee92ec1cbf04243c26500fb132716a3b2b158fdd4451a2cd6f9d2978d1; per-file manifest in
`docs/superpowers/plans/2026-09-24-workbench-redesign.md`), which the owner adopted as the workbench's design on
2026-09-24 (decisions D-WR-1 to D-WR-12 in that plan), after the wave-3.5 pilot found readers could not tell what to do
next, why the steps come in their order, or which work was theirs (F-03 to F-05).

## Resolution

Closed-spec object supersession (`spec/verdi-evidence-model` §Challenging closed decisions). `spec/workbench-redesign`
dc-6 carries the `supersedes` edges to ac-1, dc-3, and dc-4 and states the replacement. Kept and restated in dc-6: the
same exported entry points through the badge compute layer, the three-valued result, which walls carry which flags, one
flag vocabulary across surfaces, and an unproven value drawn in the disclosure style, distinct from a flag, never
dressed as a verdict. The closed spec, its closure record, and its evidence are unchanged. This store has one
maintainer, so the two-approval quorum is waived (03 step 3); the conflict is still filed.

---
id: conflict/new-story-dialog-playwright-producers
kind: conflict
title: "new-story-dialog declared its Playwright-backed obligations as design debt"
owners: [platform-team]
status: superseded
links:
  - { type: challenges, ref: spec/new-story-dialog }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# Conflict: new-story-dialog cannot cross verdi build start

## What is disputed

spec/new-story-dialog declared its 4 Playwright-backed behavioral obligations (ac-1 behavioral, ac-2 behavioral, ac-3
behavioral, ac-4 behavioral) as unresolved design debt (SI-288), because no producer grammar existed for a Playwright
test. GLG v3 keeps design debt from crossing verdi build start, and frozen obligations cannot be edited (VL-010; 03:
amendment is always forward), so the story as accepted can never be built. The dispute is wrong-for-this-story (03 §The
amendment ladder, rung 3): the parent feature's criteria still stand.

## Witness

`verdi build start spec/new-story-dialog` exits 1 on main, refusing each of those obligations as unresolved design debt
(recorded 2026-09-29 for the redesign's seven frontend stories).

## Resolution

spec/new-story-dialog-v2 supersedes spec/new-story-dialog with v1's criteria, constraints, and decisions unchanged, and
each of those obligations names its Playwright test as an elaborated producer (playwright:<file>:<title path>, SI-292).
The conflict is resolved superseded with v2 on the same branch, so the merge lands both rung-3 records together
(story-supersession design §5).

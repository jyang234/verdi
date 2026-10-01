---
id: conflict/readiness-page-playwright-producers
kind: conflict
title: "readiness-page declared its Playwright-backed obligations as design debt"
owners: [platform-team]
status: superseded
links:
  - { type: challenges, ref: spec/readiness-page }
frozen: { at: 2026-09-30, commit: dbea2070d3c3ec5f5599f20d10dda4eedb218582 }
---
# Conflict: readiness-page cannot cross verdi build start

## What is disputed

spec/readiness-page declared its 3 Playwright-backed behavioral obligations (ac-1 behavioral, ac-2 behavioral, ac-3
behavioral) as unresolved design debt (SI-288), because no producer grammar existed for a Playwright test. GLG v3 keeps
design debt from crossing verdi build start, and frozen obligations cannot be edited (VL-010; 03: amendment is always
forward), so the story as accepted can never be built. The dispute is wrong-for-this-story (03 §The amendment ladder,
rung 3): the parent feature's criteria still stand.

## Witness

`verdi build start spec/readiness-page` exits 1 on main, refusing each of those obligations as unresolved design debt
(recorded 2026-09-29 for the redesign's seven frontend stories).

## Resolution

spec/readiness-page-v2 supersedes spec/readiness-page with v1's criteria, constraints, and decisions unchanged, and each
of those obligations names its Playwright test as an elaborated producer (playwright:<file>:<title path>, SI-292). The
conflict is resolved superseded with v2 on the same branch, so the merge lands both rung-3 records together
(story-supersession design §5).

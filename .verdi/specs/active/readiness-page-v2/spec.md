---
id: spec/readiness-page-v2
kind: spec
title: "Readiness page"
owners: [platform-team]
class: story
story: jira:VERDI-WR-5
problem: { text: "The readiness page is a three-row preview of a queue. The pilot's readers could not tell what to do next, why the steps come in their order, or which work was theirs.", anchor: problem }
outcome: { text: "The readiness page shows where the design stands in four steps, what to do next in ranked order with guidance first, the known problems of later steps, and the checks already complete, stamped with the per-request derivation.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "The readiness page follows the design's layout: where you are (the spec, its class, and its branch); the four-step stepper, each step with its state and one line of count and reason; a sentence explaining why the steps run in their order; Focus next; Known problems in later steps; and Completed checks.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "Focus next ranks every concern of the current step first, then the concerns that wait on it, each marked with its step and whether it is now or later. Each item's guidance sentence is its primary line, with its fact, timing, and blocking flag in the technical disclosure (spec-documents ac-12). The waiting concerns expand inline, and no concern is left off the page.", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "Human-review work is labeled plainly as human review, with the formal obligation as secondary text; every state reads as proven, violated with a witness, or disclosed as unproven; and the solo-author language stays.", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "The page carries the per-request derivation stamp, never the design's startup-snapshot copy, and renders from the same readiness facts as the drawer's Readiness tab, with no second derivation.", evidence: [static], anchor: ac-4 }
constraints:
  - { id: co-1, text: "Wave 6 §3.1 binds as amended (SI-287): the four ordered areas, one current focus, immediate before downstream concerns with timing stated, source-derived corrective guidance, the plain explanation of the order, human review labeled plainly, the complete concern set, three-valued language, and solo-author language.", anchor: co-1 }
decisions:
  - { id: dc-1, text: "The four steps are the four ordered areas (shape-proposal, show-success, check-context, and request-review) under the design's plain labels: define the work, define success, check constraints, and get approval. The formal area ids stay as secondary text.", anchor: dc-1 }
  - { id: dc-2, text: "The readiness page and the drawer's Readiness tab render the same facts, the page in the full layout and the tab in the compact one; neither derives readiness on its own.", anchor: dc-2 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-4" }
  - { type: supersedes, ref: "spec/readiness-page" }
---
# Readiness page

## Problem

The readiness page is a three-row preview of a queue. The pilot's readers could not tell what to do next, why the steps
come in their order, or which work was theirs.

## Outcome

The readiness page shows where the design stands in four steps, what to do next in ranked order with guidance first, the
known problems of later steps, and the checks already complete, stamped with the per-request derivation.

## What changed from v1

v2 revises spec/readiness-page through rung 3 of the amendment ladder (conflict/readiness-page-playwright-producers). v1
declared its 3 Playwright-backed behavioral obligations as unresolved design debt (SI-288), because no producer grammar
existed for a Playwright test, and GLG v3 keeps design debt from crossing verdi build start. The Playwright producer now
exists (SI-292 to SI-294), so each of those obligations names its test as an elaborated producer, titled `readiness-page
› <obligation title>` in the file v1 named. The criteria, constraints, and decisions are v1's, unchanged.

## ac-1

The readiness page follows the design's layout: where you are (the spec, its class, and its branch); the four-step
stepper, each step with its state and one line of count and reason; a sentence explaining why the steps run in their
order; Focus next; Known problems in later steps; and Completed checks.

## ac-2

Focus next ranks every concern of the current step first, then the concerns that wait on it, each marked with its step
and whether it is now or later. Each item's guidance sentence is its primary line, with its fact, timing, and blocking
flag in the technical disclosure (spec-documents ac-12). The waiting concerns expand inline, and no concern is left off
the page.

## ac-3

Human-review work is labeled plainly as human review, with the formal obligation as secondary text; every state reads as
proven, violated with a witness, or disclosed as unproven; and the solo-author language stays.

## ac-4

The page carries the per-request derivation stamp, never the design's startup-snapshot copy, and renders from the same
readiness facts as the drawer's Readiness tab, with no second derivation.

## co-1

Wave 6 §3.1 binds as amended (SI-287): the four ordered areas, one current focus, immediate before downstream concerns
with timing stated, source-derived corrective guidance, the plain explanation of the order, human review labeled
plainly, the complete concern set, three-valued language, and solo-author language.

## dc-1

The four steps are the four ordered areas (shape-proposal, show-success, check-context, and request-review) under the
design's plain labels: define the work, define success, check constraints, and get approval. The formal area ids stay as
secondary text.

## dc-2

The readiness page and the drawer's Readiness tab render the same facts, the page in the full layout and the tab in the
compact one; neither derives readiness on its own.

---
id: spec/wall-receipts
kind: spec
title: "Wall Receipts"
owners: [platform-team]
class: feature
status: draft
problem: { text: "the wall computes nothing it knows: lint findings, ladder flags, and evidence folds all exist, but no card carries them, so readiness is discovered at MR time and an unexplained verdict trains authors to game the badge rather than fix the cause", anchor: problem }
outcome: { text: "every computed claim on the wall is a badge that opens its derivation — the rule, the pinned inputs, the firing records — and readiness is ambient during authoring, never a surprise at review", anchor: outcome }
acceptance_criteria:
  - { id: ac-2, text: "every wall badge opens a derivation drawer naming the rule, the pinned inputs with their revisions, and the records that fired it", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "an acceptance criterion card renders its declared evidence kinds, and an empty evidence slot badges — disclosed, never blocking", evidence: [behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "spec-stale and pending-supersession render on the case file, computed identically to the dex story-lens flags", evidence: [behavioral], anchor: "#ac-4" }
  - { id: ac-5, text: "an acceptance-criteria column exceeding the viewport raises a size-smell badge on the case file — an observation, never a rule", evidence: [behavioral], anchor: "#ac-5" }
  - { id: ac-6, text: "judged findings on the wall display their sweep provenance, so a stale or partial sweep looks stale", evidence: [behavioral], anchor: "#ac-6" }
stubs:
  - { slug: badge-computes, acceptance_criteria: [ac-2, ac-4] }
  - { slug: derivation-drawer, acceptance_criteria: [ac-2, ac-6] }
  - { slug: evidence-slot, acceptance_criteria: [ac-3] }
  - { slug: case-file-flags, acceptance_criteria: [ac-4, ac-5] }
constraints:
  - { id: co-1, text: "badges compute with no LLM and read only pinned inputs; the drawer cites input revisions, never wall-clock time", anchor: "#co-1" }
  - { id: co-2, text: "badges never block authoring: disclosure, not refusal — readiness is ambient chrome, enforced only at MR time", anchor: "#co-2" }
decisions:
  - { id: dc-1, text: "the v1 badge set is existing computes only — VL lint findings scoped to this spec, spec-stale, pending-supersession, empty evidence slots, size-smell; no computation is invented for a badge", anchor: "#dc-1" }
  - { id: dc-2, text: "a derivation drawer names the rule id, the pinned inputs with revisions, and the firing records — receipts, not verdicts; an unexplained badge trains authors to game it", anchor: "#dc-2" }
open_questions:
  - { id: oq-1, text: "which VL rules are wall-relevant per spec versus store-wide noise the wall should not carry?", anchor: "#oq-1" }
  - { id: oq-2, text: "does the ADR exemption count join the reference-card peek in this spec or ride a later context-rail pass?", anchor: "#oq-2" }
---
# Wall Receipts

## Problem

The store already computes the truths an author needs while authoring —
VL lint findings, spec-stale and pending-supersession ladder flags,
evidence-fold status — and the wall renders none of them. Readiness is
discovered at MR time, as a surprise. And a verdict without a receipt is
worse than none: an unexplained red dot trains authors to game the badge
rather than fix the cause, which is how disclosure regimes rot.

## Outcome

Every computed claim on the wall is a badge that opens its derivation —
the rule that fired, the pinned inputs with their revisions, the specific
records behind it. Readiness becomes ambient chrome during authoring and
is enforced only at MR time. Receipts, not verdicts.

## ac-2

every wall badge opens a derivation drawer naming the rule, the pinned inputs with their revisions, and the records that fired it

## ac-3

an acceptance criterion card renders its declared evidence kinds, and an empty evidence slot badges — disclosed, never blocking

## ac-4

spec-stale and pending-supersession render on the case file, computed identically to the dex story-lens flags

## ac-5

an acceptance-criteria column exceeding the viewport raises a size-smell badge on the case file — an observation, never a rule

## ac-6

judged findings on the wall display their sweep provenance, so a stale or partial sweep looks stale

## co-1

badges compute with no LLM and read only pinned inputs; the drawer cites input revisions, never wall-clock time

## co-2

badges never block authoring: disclosure, not refusal — readiness is ambient chrome, enforced only at MR time

## dc-1

the v1 badge set is existing computes only — VL lint findings scoped to this spec, spec-stale, pending-supersession, empty evidence slots, size-smell; no computation is invented for a badge

## dc-2

a derivation drawer names the rule id, the pinned inputs with revisions, and the firing records — receipts, not verdicts; an unexplained badge trains authors to game it

## oq-1

which VL rules are wall-relevant per spec versus store-wide noise the wall should not carry?

## oq-2

does the ADR exemption count join the reference-card peek in this spec or ride a later context-rail pass?

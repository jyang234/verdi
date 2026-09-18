---
id: spec/sample
kind: spec
class: feature
title: Sample
owners: [platform-team]
problem: { text: "Nothing tracks which criteria are covered and which questions are still open.", anchor: "#problem" }
outcome: { text: "Every criterion is covered by a stub or honestly reported as not yet planned, and every question is claimed or honestly reported as unclaimed.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "The store proves coverage for a criterion a stub already names.", evidence: [static], anchor: "#ac-1" }
  - { id: ac-2, text: "The store honestly reports a criterion nothing covers yet.", evidence: [static], anchor: "#ac-2" }
open_questions:
  - { id: oq-1, text: "Which stub answers the first open question?", anchor: "#oq-1" }
  - { id: oq-2, text: "Which stub answers the second open question?", anchor: "#oq-2" }
stubs:
  - { slug: do-ac-1, acceptance_criteria: [ac-1] }
  - { slug: probe-oq-1, spike: true, resolves: [oq-1] }
---
# Sample

## Problem

Nothing tracks which criteria are covered and which questions are still open.

## Outcome

Every criterion is covered by a stub or honestly reported as not yet planned, and every question is claimed or honestly reported as unclaimed.

## ac-1

The store proves coverage for a criterion a stub already names.

## ac-2

The store honestly reports a criterion nothing covers yet.

## oq-1

Which stub answers the first open question?

## oq-2

Which stub answers the second open question?

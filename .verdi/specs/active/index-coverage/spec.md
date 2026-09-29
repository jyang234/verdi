---
id: spec/index-coverage
kind: spec
title: "Index coverage"
owners: [platform-team]
class: story
story: jira:VERDI-WR-9
problem: { text: "The index cannot say which accepted features still have criteria without a story. Coverage is computed only inside the wall's projection, so a second consumer would derive it again and could disagree.", anchor: problem }
outcome: { text: "Coverage per accepted feature is one pure function, extracted from the wall projection and shared by the wall, the index, and the New story dialog. The index also gets the count of disclosures for its top bar.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "For an accepted feature, one pure function returns each acceptance criterion's coverage from the feature's declared stubs and its stories' implements edges: the stubs and stories covering it, or none. It reads nothing else, and the wall's coverage chips come from the same function with their texts unchanged.", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "An input the function cannot read, such as a stub declaration or a story it cannot decode, is disclosed on the affected criterion, never counted as no coverage, and the function never reports anything as implemented or evidenced (parent dc-5; SI-283).", evidence: [static], anchor: ac-2 }
  - { id: ac-3, text: "The index gets the count of disclosures from the same enumeration the disclosures page shows, computed once per render, for the top bar's Disclosures toggle.", evidence: [static, behavioral], anchor: ac-3 }
constraints:
  - { id: co-1, text: "Coverage is family structure, never evidence-bearing state (workbench-legibility dc-4's bar, kept by parent dc-12), and nothing is persisted.", anchor: co-1 }
  - { id: co-2, text: "The function lives in one shared internal package that the wall, the index, and the dialog import; no consumer copies it.", anchor: co-2 }
decisions:
  - { id: dc-1, text: "The function is extracted from the wall projection, not written anew, so the wall's existing coverage chips are its first consumer and their texts do not change (parent ac-2).", anchor: dc-1 }
  - { id: dc-2, text: "A criterion counts as uncovered for the call to action only when no declared stub lists it and no story implements it (parent dc-5).", anchor: dc-2 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-7" }
---
# Index coverage

## Problem

The index cannot say which accepted features still have criteria without a story. Coverage is computed only inside the
wall's projection, so a second consumer would derive it again and could disagree.

## Outcome

Coverage per accepted feature is one pure function, extracted from the wall projection and shared by the wall, the
index, and the New story dialog. The index also gets the count of disclosures for its top bar.

## ac-1

For an accepted feature, one pure function returns each acceptance criterion's coverage from the feature's declared
stubs and its stories' implements edges: the stubs and stories covering it, or none. It reads nothing else, and the
wall's coverage chips come from the same function with their texts unchanged.

## ac-2

An input the function cannot read, such as a stub declaration or a story it cannot decode, is disclosed on the affected
criterion, never counted as no coverage, and the function never reports anything as implemented or evidenced (parent
dc-5; SI-283).

## ac-3

The index gets the count of disclosures from the same enumeration the disclosures page shows, computed once per render,
for the top bar's Disclosures toggle.

## co-1

Coverage is family structure, never evidence-bearing state (workbench-legibility dc-4's bar, kept by parent dc-12), and
nothing is persisted.

## co-2

The function lives in one shared internal package that the wall, the index, and the dialog import; no consumer copies
it.

## dc-1

The function is extracted from the wall projection, not written anew, so the wall's existing coverage chips are its
first consumer and their texts do not change (parent ac-2).

## dc-2

A criterion counts as uncovered for the call to action only when no declared stub lists it and no story implements it
(parent dc-5).

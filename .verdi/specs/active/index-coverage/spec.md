---
id: spec/index-coverage
kind: spec
title: "Index coverage"
owners: [platform-team]
class: story
story: jira:VERDI-WR-9
problem: { text: "The index cannot say which accepted features still have criteria without a stub or a story. Stub coverage is computed only inside the wall's projection and story coverage only in the board's family links, so a new consumer would derive them again and could disagree.", anchor: problem }
outcome: { text: "Coverage per accepted feature is one pure function, assembled from the projections the board already computes and shared by the wall, the index, and the New story dialog. The index also gets the count of disclosures for its top bar.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "For an accepted feature, one pure function returns each acceptance criterion's coverage in two halves: the declared stubs that list it and the stories whose implements edges target it, either possibly empty. It reads nothing else, and the wall's coverage chips come from the function's stub half with their texts unchanged.", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "An input the function cannot read, such as a stub declaration or a story it cannot decode, is disclosed on the affected criterion, never counted as no coverage, and the function never reports anything as implemented or evidenced (parent dc-5; SI-283).", evidence: [static], anchor: ac-2 }
  - { id: ac-3, text: "The index gets the count of disclosures from the same enumeration the disclosures page shows, computed once per render, for the top bar's Disclosures toggle.", evidence: [static, behavioral], anchor: ac-3 }
constraints:
  - { id: co-1, text: "Coverage is family structure, never evidence-bearing state (workbench-legibility dc-4's bar, kept by parent dc-12), and nothing is persisted.", anchor: co-1 }
  - { id: co-2, text: "The function lives in one shared internal package that the wall, the index, and the dialog import; no consumer copies it.", anchor: co-2 }
  - { id: co-3, text: "The function is pure over inputs its caller already loads once per render: the feature's decoded frontmatter and the implements backlinks of the corpus index the index page already builds once per render for its other-records listing. Computing coverage for every accepted feature adds no index computation, no wall or family projection, and nothing per feature, beside the one directory index the columns come from (parent dc-12).", anchor: co-3 }
decisions:
  - { id: dc-1, text: "The function is assembled from the two projections the board already computes, not written anew. The stub half is extracted from the wall projection's per-criterion stub counts (the scoping canvas's ACCoverage), so the wall's coverage chips keep their texts (parent ac-2). The story half is read from the corpus index's implements backlinks on each criterion (spec/<feature>#<ac>), the same edges workbench-legibility ac-2's family projection renders; it is not read through the stubs' criterion lists, so a story that implements an unstubbed criterion still covers it. This is the reading of parent dc-5 that SI-283 records: its test is that no declared stub lists the criterion and no story implements it, so every implements edge counts, including one on a criterion no stub lists, which the board's stub-card family view does not show.", anchor: dc-1 }
  - { id: dc-2, text: "A criterion counts as uncovered for the call to action only when no declared stub lists it and no story implements it (parent dc-5).", anchor: dc-2 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-7" }
---
# Index coverage

## Problem

The index cannot say which accepted features still have criteria without a stub or a story. Stub coverage is computed
only inside the wall's projection and story coverage only in the board's family links, so a new consumer would derive
them again and could disagree.

## Outcome

Coverage per accepted feature is one pure function, assembled from the projections the board already computes and shared
by the wall, the index, and the New story dialog. The index also gets the count of disclosures for its top bar.

## ac-1

For an accepted feature, one pure function returns each acceptance criterion's coverage in two halves: the declared
stubs that list it and the stories whose implements edges target it, either possibly empty. It reads nothing else, and
the wall's coverage chips come from the function's stub half with their texts unchanged.

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

## co-3

The function is pure over inputs its caller already loads once per render: the feature's decoded frontmatter and the
implements backlinks of the corpus index the index page already builds once per render for its other-records listing.
Computing coverage for every accepted feature adds no index computation, no wall or family projection, and nothing per
feature, beside the one directory index the columns come from (parent dc-12).

## dc-1

The function is assembled from the two projections the board already computes, not written anew. The stub half is
extracted from the wall projection's per-criterion stub counts (the scoping canvas's ACCoverage), so the wall's coverage
chips keep their texts (parent ac-2). The story half is read from the corpus index's implements backlinks on each
criterion (spec/<feature>#<ac>), the same edges workbench-legibility ac-2's family projection renders; it is not read
through the stubs' criterion lists, so a story that implements an unstubbed criterion still covers it. This is the
reading of parent dc-5 that SI-283 records: its test is that no declared stub lists the criterion and no story
implements it, so every implements edge counts, including one on a criterion no stub lists, which the board's stub-card
family view does not show.

## dc-2

A criterion counts as uncovered for the call to action only when no declared stub lists it and no story implements it
(parent dc-5).

---
id: spec/new-story-dialog
kind: spec
title: "New story dialog"
owners: [platform-team]
class: story
story: jira:VERDI-WR-7
problem: { text: "Starting a story from a feature asks for a name without showing the branch it will cut or which criteria already have stories, and an index reader who sees an uncovered criterion has to find the dialog and select that criterion again.", anchor: problem }
outcome: { text: "The New story dialog shows the branch it will cut as the name is typed, catches a bad name inline, lists each criterion with its coverage, starts with the uncovered criterion claimed when opened from the index, and writes nothing until Create.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "As the name is typed, the dialog shows the branch it will cut (design/<slug>) and reports a name that breaks the grammar inline. Create stays disabled until the name is valid and at least one acceptance criterion is claimed, and the status line says what is missing next.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "The dialog lists each acceptance criterion of the feature with its existing coverage text, from the same coverage function the wall and the index use, and each criterion can be claimed or unclaimed.", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "Opened from an index card's call to action, the dialog starts with that uncovered criterion claimed.", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "Nothing is written until Create. Create cuts the branch and scaffolds the story through the existing creation path, and Cancel or Escape leaves no branch, file, or ref behind.", evidence: [behavioral], anchor: ac-4 }
constraints:
  - { id: co-1, text: "Labels use the store's vocabulary preset (planned story, research spike), never the design's hard-coded story or New story (parent co-4).", anchor: co-1 }
decisions:
  - { id: dc-1, text: "The dialog reuses the existing creation form's write path and its name grammar. It adds the branch preview, the inline grammar report, and the coverage list, and changes no write.", anchor: dc-1 }
  - { id: dc-2, text: "Coverage comes from index-coverage's shared function, so the dialog, the wall, and the index never disagree about which criteria have stories.", anchor: dc-2 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-6" }
---
# New story dialog

## Problem

Starting a story from a feature asks for a name without showing the branch it will cut or which criteria already have
stories, and an index reader who sees an uncovered criterion has to find the dialog and select that criterion again.

## Outcome

The New story dialog shows the branch it will cut as the name is typed, catches a bad name inline, lists each criterion
with its coverage, starts with the uncovered criterion claimed when opened from the index, and writes nothing until
Create.

## ac-1

As the name is typed, the dialog shows the branch it will cut (design/<slug>) and reports a name that breaks the grammar
inline. Create stays disabled until the name is valid and at least one acceptance criterion is claimed, and the status
line says what is missing next.

## ac-2

The dialog lists each acceptance criterion of the feature with its existing coverage text, from the same coverage
function the wall and the index use, and each criterion can be claimed or unclaimed.

## ac-3

Opened from an index card's call to action, the dialog starts with that uncovered criterion claimed.

## ac-4

Nothing is written until Create. Create cuts the branch and scaffolds the story through the existing creation path, and
Cancel or Escape leaves no branch, file, or ref behind.

## co-1

Labels use the store's vocabulary preset (planned story, research spike), never the design's hard-coded story or New
story (parent co-4).

## dc-1

The dialog reuses the existing creation form's write path and its name grammar. It adds the branch preview, the inline
grammar report, and the coverage list, and changes no write.

## dc-2

Coverage comes from index-coverage's shared function, so the dialog, the wall, and the index never disagree about which
criteria have stories.

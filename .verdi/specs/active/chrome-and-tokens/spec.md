---
id: spec/chrome-and-tokens
kind: spec
title: "Chrome and tokens"
owners: [platform-team]
class: story
story: jira:VERDI-WR-1
problem: { text: "Every workbench page stacks up to three header rows before its content: the site header, the board header, and the posture row. The wall also draws stamps, tilted cards, and handwriting that read as gimmicks. A reader cannot tell at a glance which page, spec, and branch they are on, or whether their work is saved.", anchor: problem }
outcome: { text: "Every workbench page opens with one top bar that names the page, the spec with its class and mode, the branch, and whether the working tree is clean, with the full posture one action away. Nothing in the workbench is rotated or stamped, and the docs site is unchanged.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Every workbench page renders exactly one top bar (data-testid topbar) and none of today's site header, board header, or posture row: the wall on the default branch and on a design branch, the Document page, the diagram editor, the readiness page, the index, and each page on the shared layout (matrix, verdict, disclosures, spec import, corpus, not found, and error). The bar carries the wordmark linking to the index, the page title, the class and mode chips on a page about one spec, the branch and posture text, and a slot for the page's own controls.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "On a page about one spec, the bar shows the displayed bytes (proposed or accepted) and the working tree's clean or dirty state at every width from 320 px. One action on the posture text opens the full posture (checkout, branch, worktree head, accepted head, and ahead and behind) with the same facts and the same proven, violated-with-witness, or disclosed-unproven states today's posture row shows, and it opens without JavaScript.", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "No workbench element is rotated, and stamps are drawn as chips: status, class, and mode marks render as chips, and the hand font appears only on a story or spike sticky parked in the stubs band. Every other card, badge, and label uses the body or monospace face.", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "The tokens --wall-edge and --scrim are defined with dark-mode overrides; new rules use tokens only, except the pushpin highlights and shadows; and the workbench chrome's rules are scoped so that a docs-site build of a fixture store is byte-identical before and after this story.", evidence: [static], anchor: ac-4 }
  - { id: ac-5, text: "The bar works at 320 px width and at 200 % zoom without horizontal scrolling, is present in the initial server response before any JavaScript runs, keeps every control reachable by keyboard in reading order, passes the existing accessibility checks, and keeps each page within the Wave 6 page budget.", evidence: [behavioral], anchor: ac-5 }
constraints:
  - { id: co-1, text: "The parent's constraints co-1 to co-8 bind this story unchanged: responsiveness, no-JS use and the page budget, keyboard and accessibility, three-valued honesty, the vocabulary preset, tokens and scoping, evidence files never renamed, kept semantics, and Fable-built frontend with Playwright proof and no network.", anchor: co-1 }
decisions:
  - { id: dc-1, text: "Workbench pages are the pages verdi serve renders; the docs site is the static site built from the default branch. The corpus pages the workbench serves under /a/ are workbench pages and get the bar. The docs site keeps its own header, and its bytes do not change (parent ac-1 and co-5).", anchor: dc-1 }
  - { id: dc-2, text: "The full posture opens as a disclosure from the bar that needs no script (a native details element or equivalent). Its facts come from the posture model today's row renders; nothing is recomputed for the bar.", anchor: dc-2 }
  - { id: dc-3, text: "The controls in today's board header move into the bar's controls slot unchanged, so no page loses a control before the stories that restyle them land: the readiness pill and Commit and push (wall-strip-and-drawer) and the Wall and Document switch (document-page).", anchor: dc-3 }
  - { id: dc-4, text: "The posture's existing test ids move into the bar unchanged, and no Playwright file that an obligation names as evidence is renamed (parent co-6); where such a file asserts the old rows, only that assertion changes.", anchor: dc-4 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-1" }
---
# Chrome and tokens

## Problem

Every workbench page stacks up to three header rows before its content: the site header, the board header, and the
posture row. The wall also draws stamps, tilted cards, and handwriting that read as gimmicks. A reader cannot tell at a
glance which page, spec, and branch they are on, or whether their work is saved.

## Outcome

Every workbench page opens with one top bar that names the page, the spec with its class and mode, the branch, and
whether the working tree is clean, with the full posture one action away. Nothing in the workbench is rotated or
stamped, and the docs site is unchanged.

## ac-1

Every workbench page renders exactly one top bar (data-testid topbar) and none of today's site header, board header, or
posture row: the wall on the default branch and on a design branch, the Document page, the diagram editor, the readiness
page, the index, and each page on the shared layout (matrix, verdict, disclosures, spec import, corpus, not found, and
error). The bar carries the wordmark linking to the index, the page title, the class and mode chips on a page about one
spec, the branch and posture text, and a slot for the page's own controls.

## ac-2

On a page about one spec, the bar shows the displayed bytes (proposed or accepted) and the working tree's clean or dirty
state at every width from 320 px. One action on the posture text opens the full posture (checkout, branch, worktree
head, accepted head, and ahead and behind) with the same facts and the same proven, violated-with-witness, or
disclosed-unproven states today's posture row shows, and it opens without JavaScript.

## ac-3

No workbench element is rotated, and stamps are drawn as chips: status, class, and mode marks render as chips, and the
hand font appears only on a story or spike sticky parked in the stubs band. Every other card, badge, and label uses the
body or monospace face.

## ac-4

The tokens --wall-edge and --scrim are defined with dark-mode overrides; new rules use tokens only, except the pushpin
highlights and shadows; and the workbench chrome's rules are scoped so that a docs-site build of a fixture store is
byte-identical before and after this story.

## ac-5

The bar works at 320 px width and at 200 % zoom without horizontal scrolling, is present in the initial server response
before any JavaScript runs, keeps every control reachable by keyboard in reading order, passes the existing
accessibility checks, and keeps each page within the Wave 6 page budget.

## co-1

The parent's constraints co-1 to co-8 bind this story unchanged: responsiveness, no-JS use and the page budget, keyboard
and accessibility, three-valued honesty, the vocabulary preset, tokens and scoping, evidence files never renamed, kept
semantics, and Fable-built frontend with Playwright proof and no network.

## dc-1

Workbench pages are the pages verdi serve renders; the docs site is the static site built from the default branch. The
corpus pages the workbench serves under /a/ are workbench pages and get the bar. The docs site keeps its own header, and
its bytes do not change (parent ac-1 and co-5).

## dc-2

The full posture opens as a disclosure from the bar that needs no script (a native details element or equivalent). Its
facts come from the posture model today's row renders; nothing is recomputed for the bar.

## dc-3

The controls in today's board header move into the bar's controls slot unchanged, so no page loses a control before the
stories that restyle them land: the readiness pill and Commit and push (wall-strip-and-drawer) and the Wall and Document
switch (document-page).

## dc-4

The posture's existing test ids move into the bar unchanged, and no Playwright file that an obligation names as evidence
is renamed (parent co-6); where such a file asserts the old rows, only that assertion changes.

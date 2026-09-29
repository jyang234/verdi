---
id: spec/index
kind: spec
title: "Index"
owners: [platform-team]
class: story
story: jira:VERDI-WR-10
problem: { text: "The index is one column of lists, a separate glance above an exhaustive directory, with every record kind listed below. A reader cannot see at a glance which specs need a move, which wait on review, and what has settled.", anchor: problem }
outcome: { text: "The index shows every spec in four status columns with its age, source, review state, disclosures, and next move, with a list view of the same groups, filters, an other-records strip, and keyboard movement.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "The index shows every spec on the default branch and every draft on a design branch exactly once, in four columns in this order: On the desk, Accepted, Active components, and On the shelf (workbench-directory dc-2's groups). Each column renders its heading, its count, and an explicit empty state when it holds no card, and all four come from one directory index computation per render with nothing persisted.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "Each card shows its title, ref, status badge, working links (a board link where one can be served, and matrix and verdict for a default-branch feature with stories), age, source, the in-review chip for a draft whose branch has an open pull request, its next move, and any disclosure. Columns and cards carry the dir-group and dir-entry test ids.", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "The filter row selects everything, quiet drafts, drafts in review, and disclosed entries. When the forge cannot be reached, the in-review chip and filter say review status is unavailable, never zero, and every card stays in its column.", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "An accepted feature with an uncovered criterion offers the New story call to action naming that criterion, which opens the New story dialog with it claimed; the call to action is dropped if it breaks the page budget.", evidence: [behavioral], anchor: ac-4 }
  - { id: ac-5, text: "On the shelf shows terminal specs still in the active zone and folds archived specs into a collapsed list at its foot, so archived specs never lead the page. Below the columns, the other-records strip lists every other record kind, the services, and the boards, collapsed with counts, each expandable to its full listing without JavaScript. The disclosures pointer is the bar's Disclosures toggle, and the disclosed entry for a branch with no draft spec and the notice page for a deleted branch stay.", evidence: [behavioral], anchor: ac-5 }
  - { id: ac-6, text: "A list view shows the same four groups with the same entries and labels, and the keyboard moves between cards and opens one with Enter. When the index computation fails, the page discloses it once, and neither view renders partial or invented groups.", evidence: [behavioral], anchor: ac-6 }
constraints:
  - { id: co-1, text: "home-status-glance co-1 and co-2 and directory-home stay in force: every behavioral path is proven by Playwright against fixture stores, an index failure degrades every section identically, and the link grammar is today's.", anchor: co-1 }
  - { id: co-2, text: "fixtures.ts changes only in this story (plan step 2, shared files). The 37-directory-home and 43-home-status-glance files are never renamed, and their assertions change only as the superseded objects require (parent co-6).", anchor: co-2 }
decisions:
  - { id: dc-1, text: "This story carries, as its criteria, the kept properties of the superseded home-status-glance objects and workbench-legibility dc-4 (parent dc-12): actionable-first order, archived specs never leading, counts and empty states, one computation per render, nothing persisted, status badges and working links, and no evidence-bearing state.", anchor: dc-1 }
  - { id: dc-2, text: "The design's In review column is a chip and a filter, and its Built column is labelled Active components (parent dc-12).", anchor: dc-2 }
  - { id: dc-3, text: "Age and quiet come from index-data, and coverage and the disclosures count from index-coverage; this story reads them and derives neither.", anchor: dc-3 }
  - { id: dc-4, text: "The mine filter and the owner-identity pill are not built (parent dc-3).", anchor: dc-4 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-7" }
---
# Index

## Problem

The index is one column of lists, a separate glance above an exhaustive directory, with every record kind listed below.
A reader cannot see at a glance which specs need a move, which wait on review, and what has settled.

## Outcome

The index shows every spec in four status columns with its age, source, review state, disclosures, and next move, with a
list view of the same groups, filters, an other-records strip, and keyboard movement.

## ac-1

The index shows every spec on the default branch and every draft on a design branch exactly once, in four columns in
this order: On the desk, Accepted, Active components, and On the shelf (workbench-directory dc-2's groups). Each column
renders its heading, its count, and an explicit empty state when it holds no card, and all four come from one directory
index computation per render with nothing persisted.

## ac-2

Each card shows its title, ref, status badge, working links (a board link where one can be served, and matrix and
verdict for a default-branch feature with stories), age, source, the in-review chip for a draft whose branch has an open
pull request, its next move, and any disclosure. Columns and cards carry the dir-group and dir-entry test ids.

## ac-3

The filter row selects everything, quiet drafts, drafts in review, and disclosed entries. When the forge cannot be
reached, the in-review chip and filter say review status is unavailable, never zero, and every card stays in its column.

## ac-4

An accepted feature with an uncovered criterion offers the New story call to action naming that criterion, which opens
the New story dialog with it claimed; the call to action is dropped if it breaks the page budget.

## ac-5

On the shelf shows terminal specs still in the active zone and folds archived specs into a collapsed list at its foot,
so archived specs never lead the page. Below the columns, the other-records strip lists every other record kind, the
services, and the boards, collapsed with counts, each expandable to its full listing without JavaScript. The disclosures
pointer is the bar's Disclosures toggle, and the disclosed entry for a branch with no draft spec and the notice page for
a deleted branch stay.

## ac-6

A list view shows the same four groups with the same entries and labels, and the keyboard moves between cards and opens
one with Enter. When the index computation fails, the page discloses it once, and neither view renders partial or
invented groups.

## co-1

home-status-glance co-1 and co-2 and directory-home stay in force: every behavioral path is proven by Playwright against
fixture stores, an index failure degrades every section identically, and the link grammar is today's.

## co-2

fixtures.ts changes only in this story (plan step 2, shared files). The 37-directory-home and 43-home-status-glance
files are never renamed, and their assertions change only as the superseded objects require (parent co-6).

## dc-1

This story carries, as its criteria, the kept properties of the superseded home-status-glance objects and
workbench-legibility dc-4 (parent dc-12): actionable-first order, archived specs never leading, counts and empty states,
one computation per render, nothing persisted, status badges and working links, and no evidence-bearing state.

## dc-2

The design's In review column is a chip and a filter, and its Built column is labelled Active components (parent dc-12).

## dc-3

Age and quiet come from index-data, and coverage and the disclosures count from index-coverage; this story reads them
and derives neither.

## dc-4

The mine filter and the owner-identity pill are not built (parent dc-3).

---
id: spec/wall-changes
kind: spec
title: "Wall changes"
owners: [platform-team]
class: story
story: jira:VERDI-WR-3
problem: { text: "The wall knows only that the working tree is dirty. A reader cannot see what the uncommitted changes are, and a change the semantic diff does not recognize (prose, layout, another staged file) is easy to miss, or to be told there is nothing to commit when there is.", anchor: problem }
outcome: { text: "The wall's snapshot reports the uncommitted changes in three distinct states (typed changes, unclassified changes, and an unreadable comparison), so Commit and push can show what will be committed and never shows a clean tree while any change remains.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "The wall snapshot, in its page model and JSON, carries the spec's uncommitted changes against HEAD: typed changes, each a recognized operation from the semantic diff of HEAD's spec to the working tree's spec with the object it touches; unclassified changes, each a path with the reason the semantic diff does not classify it (prose or body text, layout, another staged path, or an untracked file); and an unreadable comparison, disclosed with its reason when HEAD's or the working tree's spec cannot be read or diffed, never reported as no changes.", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "A semantic diff with zero recognized operations never reports the tree clean while any other change remains: the dirty indicator stays set, and each remaining change is listed as unclassified.", evidence: [static, behavioral], anchor: ac-2 }
  - { id: ac-3, text: "The branch-switch guard reads the working tree's own git status, independent of the changes summary: a zero typed-change count never lets a switch proceed over uncommitted changes, and an unreadable comparison still blocks the switch while git reports the tree dirty.", evidence: [behavioral], anchor: ac-3 }
constraints:
  - { id: co-1, text: "The summary is computed per request from git and the working tree and is never persisted; the workbench performs no git write to compute it.", anchor: co-1 }
  - { id: co-2, text: "Every integration with git is proven against fixturegit repositories, never the live store, and no test uses the network.", anchor: co-2 }
decisions:
  - { id: dc-1, text: "Typed changes come from the existing semantic diff (internal/draftmutation's Diff), reused and never re-derived. This story adds the classification of what that diff does not recognize and the unreadable state.", anchor: dc-1 }
  - { id: dc-2, text: "A readable comparison reports its typed and unclassified changes side by side, so a tree can carry both at once. An unreadable comparison carries its reason instead of a typed list, and still lists what git status reports as unclassified changes.", anchor: dc-2 }
  - { id: dc-3, text: "This story changes the snapshot and its JSON only. How the changes are shown, the popover and its three states, belongs to wall-strip-and-drawer, which consumes this contract.", anchor: dc-3 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-3" }
---
# Wall changes

## Problem

The wall knows only that the working tree is dirty. A reader cannot see what the uncommitted changes are, and a change
the semantic diff does not recognize (prose, layout, another staged file) is easy to miss, or to be told there is
nothing to commit when there is.

## Outcome

The wall's snapshot reports the uncommitted changes in three distinct states (typed changes, unclassified changes, and
an unreadable comparison), so Commit and push can show what will be committed and never shows a clean tree while any
change remains.

## ac-1

The wall snapshot, in its page model and JSON, carries the spec's uncommitted changes against HEAD: typed changes, each
a recognized operation from the semantic diff of HEAD's spec to the working tree's spec with the object it touches;
unclassified changes, each a path with the reason the semantic diff does not classify it (prose or body text, layout,
another staged path, or an untracked file); and an unreadable comparison, disclosed with its reason when HEAD's or the
working tree's spec cannot be read or diffed, never reported as no changes.

## ac-2

A semantic diff with zero recognized operations never reports the tree clean while any other change remains: the dirty
indicator stays set, and each remaining change is listed as unclassified.

## ac-3

The branch-switch guard reads the working tree's own git status, independent of the changes summary: a zero typed-change
count never lets a switch proceed over uncommitted changes, and an unreadable comparison still blocks the switch while
git reports the tree dirty.

## co-1

The summary is computed per request from git and the working tree and is never persisted; the workbench performs no git
write to compute it.

## co-2

Every integration with git is proven against fixturegit repositories, never the live store, and no test uses the
network.

## dc-1

Typed changes come from the existing semantic diff (internal/draftmutation's Diff), reused and never re-derived. This
story adds the classification of what that diff does not recognize and the unreadable state.

## dc-2

A readable comparison reports its typed and unclassified changes side by side, so a tree can carry both at once. An
unreadable comparison carries its reason instead of a typed list, and still lists what git status reports as
unclassified changes.

## dc-3

This story changes the snapshot and its JSON only. How the changes are shown, the popover and its three states, belongs
to wall-strip-and-drawer, which consumes this contract.

---
id: spec/ritual-effect-witness
kind: spec
title: "Ritual effect witness"
owners: [platform-team]
class: story
story: jira:VERDI-PH-4
problem: { text: "A declaration is only a promise until something runs the ritual and compares what it did with what it declared. Six real findings (UAT-021 to UAT-036) were rituals sweeping operators' work into commits, and three of them, plus the carried rituals the census found, are still open.", anchor: problem }
outcome: { text: "A harness runs every declared ritual on seeded fixtures through its real entry point, observes refs, HEAD, the index, the working tree, linked worktrees, and every commit it creates, and fails on any effect outside the declaration; the open findings are fixed and pinned, so the class becomes a red gate.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A test harness runs a ritual through its real entry point against a fixturegit repository with a local bare remote, in two seeded states: one with an untracked file, a pre-staged unrelated index entry, and a dirty tracked file, and one with the same untracked and dirty tracked work and a clean index. It observes, before and after, the refs, HEAD, the index, the working tree, and the linked-worktree list, plus the file list of every commit the ritual creates and its git command log; it reports each effect as within the declaration, outside it, or unattributable, never counting an unattributable effect as within scope, and asserts the declared index carry exactly (a refusal exits 2 with no mutation, a scoped commit omits the foreign entry, no commit exists when none is declared).", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "Every declared ritual, along each publication path it supports, runs in both seeded states and every effect it has lies within its declaration; a ritual declared to complete completes in the clean-index state.", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "design start (including its --supersedes path), the commit-to-design ritual (verdi board commit and the workbench's commit), accept diagram, and constitution propose never commit a pre-staged entry outside their declared paths; close refuses a non-empty index before any mutation (exit 2, nothing changed); and the board's Commit and push is the one ritual declared carried (UAT-036).", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "build start and constitution propose cut their branches from the resolved default branch, not from HEAD (UAT-023), and build start refuses a branch name that already exists in the tree it cuts from (UAT-031); UAT-021, UAT-033, and UAT-034 stay covered by their existing tests.", evidence: [behavioral], anchor: ac-4 }
constraints:
  - { id: co-1, text: "The parent's constraints bind this story unchanged: no new git primitive and no widened declaration without an owner-visible decision (co-1), the declaration is data checked by the gate (co-2), every test is hermetic over fixturegit repositories with pushes only to a local bare remote (co-3), and recovery's semantics do not change (co-4).", anchor: co-1 }
decisions:
  - { id: dc-1, text: "The harness lives in internal/ritualwitness and reads each ritual's declaration from internal/writescope, so a ritual is checked against exactly what the registry declares. CLI verbs run as the built binary, workbench actions through the running server's handlers, and MCP tools through the MCP server, each with the provider and forge fakes the repository's tests already use and no network.", anchor: dc-1 }
  - { id: dc-2, text: "The fixes follow the owner's decisions of 2026-09-30: design start's --supersedes path, commit-to-design, accept diagram, and constitution propose become scoped commits; constitution propose cuts from the resolved default branch; the board's Commit and push stays carried and declared. Each fix narrows its ritual's declaration in the same change (parent co-1).", anchor: dc-2 }
  - { id: dc-3, text: "A publication path is each distinct way a ritual commits or publishes (for close: the CI path, --force-local, a feature close, and the failure unwind; for stub instantiation: the CLI and each board action). A path that cannot be run hermetically is reported unproven by name, never skipped silently.", anchor: dc-3 }
links:
  - { type: implements, ref: "spec/ritual-write-scope-v3#ac-2" }
  - { type: implements, ref: "spec/ritual-write-scope-v3#ac-4" }
---
# Ritual effect witness

## Problem

A declaration is only a promise until something runs the ritual and compares what it did with what it declared. Six real
findings (UAT-021 to UAT-036) were rituals sweeping operators' work into commits, and three of them, plus the carried
rituals the census found, are still open.

## Outcome

A harness runs every declared ritual on seeded fixtures through its real entry point, observes refs, HEAD, the index,
the working tree, linked worktrees, and every commit it creates, and fails on any effect outside the declaration; the
open findings are fixed and pinned, so the class becomes a red gate.

## ac-1

A test harness runs a ritual through its real entry point against a fixturegit repository with a local bare remote, in
two seeded states: one with an untracked file, a pre-staged unrelated index entry, and a dirty tracked file, and one
with the same untracked and dirty tracked work and a clean index. It observes, before and after, the refs, HEAD, the
index, the working tree, and the linked-worktree list, plus the file list of every commit the ritual creates and its git
command log; it reports each effect as within the declaration, outside it, or unattributable, never counting an
unattributable effect as within scope, and asserts the declared index carry exactly (a refusal exits 2 with no mutation,
a scoped commit omits the foreign entry, no commit exists when none is declared).

## ac-2

Every declared ritual, along each publication path it supports, runs in both seeded states and every effect it has lies
within its declaration; a ritual declared to complete completes in the clean-index state.

## ac-3

design start (including its --supersedes path), the commit-to-design ritual (verdi board commit and the workbench's
commit), accept diagram, and constitution propose never commit a pre-staged entry outside their declared paths; close
refuses a non-empty index before any mutation (exit 2, nothing changed); and the board's Commit and push is the one
ritual declared carried (UAT-036).

## ac-4

build start and constitution propose cut their branches from the resolved default branch, not from HEAD (UAT-023), and
build start refuses a branch name that already exists in the tree it cuts from (UAT-031); UAT-021, UAT-033, and UAT-034
stay covered by their existing tests.

## co-1

The parent's constraints bind this story unchanged: no new git primitive and no widened declaration without an
owner-visible decision (co-1), the declaration is data checked by the gate (co-2), every test is hermetic over
fixturegit repositories with pushes only to a local bare remote (co-3), and recovery's semantics do not change (co-4).

## dc-1

The harness lives in internal/ritualwitness and reads each ritual's declaration from internal/writescope, so a ritual is
checked against exactly what the registry declares. CLI verbs run as the built binary, workbench actions through the
running server's handlers, and MCP tools through the MCP server, each with the provider and forge fakes the repository's
tests already use and no network.

## dc-2

The fixes follow the owner's decisions of 2026-09-30: design start's --supersedes path, commit-to-design, accept
diagram, and constitution propose become scoped commits; constitution propose cuts from the resolved default branch; the
board's Commit and push stays carried and declared. Each fix narrows its ritual's declaration in the same change (parent
co-1).

## dc-3

A publication path is each distinct way a ritual commits or publishes (for close: the CI path, --force-local, a feature
close, and the failure unwind; for stub instantiation: the CLI and each board action). A path that cannot be run
hermetically is reported unproven by name, never skipped silently.

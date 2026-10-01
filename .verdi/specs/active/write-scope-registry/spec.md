---
id: spec/write-scope-registry
kind: spec
title: "Write-scope registry"
owners: [platform-team]
class: story
story: jira:VERDI-PH-3
problem: { text: "Nothing records what a verb may do to a repository. Nineteen entry groups mutate git state (the census of 2026-09-30), several reachable only from the workbench or MCP, and a new verb can branch, stage, commit, or push with no declaration anyone checks.", anchor: problem }
outcome: { text: "One registry declares, as data in source, the write scope of every verb that mutates a repository in a closed grammar, and a static witness fails when a verb that can reach a mutating gitx function has no declaration, or when gitx gains a function no one has classified.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "One internal package returns, from a function, the write-scope declaration of every verb that mutates a git repository, where a verb is a CLI verb, a workbench action, or an MCP tool (parent dc-6). Each declaration names its verbs and states, in a closed grammar, the refs it may create, the refs it may move, the refs it may delete, whether HEAD may switch, the linked worktrees it may add or remove, the paths it may stage, its index carry (refused, scoped, carried, or no_commit), whether untracked files may enter a commit, and whether it may push. Every declaration validates, and an unknown field or value fails closed. Every exported internal/gitx function, and every function outside gitx that writes under a repository's git directory, is classified as mutating or read-only in one list, and a static witness fails when such a function is unclassified, when a classified name no longer exists, when a verb that reaches a mutating gitx function has no declaration, or when a declaration names a verb that reaches none. The declarations whose ritual still awaits the fix that makes it conform are a named, counted list the witness reports, never hidden.", evidence: [static], anchor: ac-1 }
constraints:
  - { id: co-1, text: "The parent's constraints bind this story unchanged: no new git primitive and no widened declaration without an owner-visible decision (co-1), the declaration is data checked by the gate (co-2), every test is hermetic over fixturegit repositories with pushes only to a local bare remote (co-3), and recovery's semantics do not change (co-4).", anchor: co-1 }
decisions:
  - { id: dc-1, text: "The registry lives in internal/writescope and is returned by a function, never held in a package variable (docs/ground-rules.md), the same shape as the CLI-verb and MCP-tool inventories (parent dc-4). Its values are typed, so an unknown field cannot compile and an unknown enum value fails its Validate.", anchor: dc-1 }
  - { id: dc-2, text: "Reachability is computed statically from the module's source, with interface calls resolved to every implementation, over the entry points the CLI-verb inventory, the MCP-tool inventory, and the workbench's actions define, where a workbench action is any workbench request that can mutate a repository, including the branch routes that ensure a managed worktree, which parent dc-6 names among the workbench's mutating entry points, and it reaches the non-gitx writers too (the execution-workspace reconciler's and garbage collector's writes to worktree administrative entries), so a verb that changes git state only through them is still found (parent dc-2, dc-7). A new module dependency for this needs a ledger entry before it is added; the standard library's go/parser and go/types are enough.", anchor: dc-2 }
  - { id: dc-3, text: "The census of 2026-09-30 is the starting set of declarations, each read through parent dc-7's field readings. A declaration describes what its ritual does at this story's base, except where the parent has already ruled the behavior a defect to fix. accept diagram, design start --supersedes, and constitution propose are declared scoped, as parent dc-11 rules. Plain design start and the commit-to-design ritual must never carry a foreign entry (parent ac-4), which a refusal would also meet; this story chooses the scoped fix for them too, because a refusal would stop an operator whose unrelated work is staged, and one fix mode for all five rituals keeps parent dc-3's first-state rule simple. Only the board's Commit and push declares carried (parent dc-3). The five sit in the counted awaiting-fix list until spec/ritual-effect-witness fixes the rituals and empties it; its behavioral witness lands in the same change as those fixes, so no merged witness ever passes against a declaration the ritual does not meet. Nothing is widened to pass (parent co-1).", anchor: dc-3 }
links:
  - { type: implements, ref: "spec/ritual-write-scope-v3#ac-1" }
---
# Write-scope registry

## Problem

Nothing records what a verb may do to a repository. Nineteen entry groups mutate git state (the census of 2026-09-30),
several reachable only from the workbench or MCP, and a new verb can branch, stage, commit, or push with no declaration
anyone checks.

## Outcome

One registry declares, as data in source, the write scope of every verb that mutates a repository in a closed grammar,
and a static witness fails when a verb that can reach a mutating gitx function has no declaration, or when gitx gains a
function no one has classified.

## ac-1

One internal package returns, from a function, the write-scope declaration of every verb that mutates a git repository,
where a verb is a CLI verb, a workbench action, or an MCP tool (parent dc-6). Each declaration names its verbs and
states, in a closed grammar, the refs it may create, the refs it may move, the refs it may delete, whether HEAD may
switch, the linked worktrees it may add or remove, the paths it may stage, its index carry (refused, scoped, carried, or
no_commit), whether untracked files may enter a commit, and whether it may push. Every declaration validates, and an
unknown field or value fails closed. Every exported internal/gitx function, and every function outside gitx that writes
under a repository's git directory, is classified as mutating or read-only in one list, and a static witness fails when
such a function is unclassified, when a classified name no longer exists, when a verb that reaches a mutating gitx
function has no declaration, or when a declaration names a verb that reaches none. The declarations whose ritual still
awaits the fix that makes it conform are a named, counted list the witness reports, never hidden.

## co-1

The parent's constraints bind this story unchanged: no new git primitive and no widened declaration without an
owner-visible decision (co-1), the declaration is data checked by the gate (co-2), every test is hermetic over
fixturegit repositories with pushes only to a local bare remote (co-3), and recovery's semantics do not change (co-4).

## dc-1

The registry lives in internal/writescope and is returned by a function, never held in a package variable
(docs/ground-rules.md), the same shape as the CLI-verb and MCP-tool inventories (parent dc-4). Its values are typed, so
an unknown field cannot compile and an unknown enum value fails its Validate.

## dc-2

Reachability is computed statically from the module's source, with interface calls resolved to every implementation,
over the entry points the CLI-verb inventory, the MCP-tool inventory, and the workbench's actions define, where a
workbench action is any workbench request that can mutate a repository, including the branch routes that ensure a
managed worktree, which parent dc-6 names among the workbench's mutating entry points, and it reaches the non-gitx
writers too (the execution-workspace reconciler's and garbage collector's writes to worktree administrative entries), so
a verb that changes git state only through them is still found (parent dc-2, dc-7). A new module dependency for this
needs a ledger entry before it is added; the standard library's go/parser and go/types are enough.

## dc-3

The census of 2026-09-30 is the starting set of declarations, each read through parent dc-7's field readings. A
declaration describes what its ritual does at this story's base, except where the parent has already ruled the behavior
a defect to fix. accept diagram, design start --supersedes, and constitution propose are declared scoped, as parent
dc-11 rules. Plain design start and the commit-to-design ritual must never carry a foreign entry (parent ac-4), which a
refusal would also meet; this story chooses the scoped fix for them too, because a refusal would stop an operator whose
unrelated work is staged, and one fix mode for all five rituals keeps parent dc-3's first-state rule simple. Only the
board's Commit and push declares carried (parent dc-3). The five sit in the counted awaiting-fix list until
spec/ritual-effect-witness fixes the rituals and empties it; its behavioral witness lands in the same change as those
fixes, so no merged witness ever passes against a declaration the ritual does not meet. Nothing is widened to pass
(parent co-1).

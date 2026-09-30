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
  - { id: ac-1, text: "One internal package returns, from a function, the write-scope declaration of every verb that mutates a git repository, where a verb is a CLI verb, a workbench action, or an MCP tool (parent dc-6). Each declaration names its verbs and states, in a closed grammar, the refs it may create, the refs it may move, the refs it may delete, whether HEAD may switch, the linked worktrees it may add or remove, the paths it may stage, its index carry (refused, scoped, carried, or no_commit), whether untracked files may enter a commit, and whether it may push. Every declaration validates, and an unknown field or value fails closed. Every exported internal/gitx function is classified as mutating or read-only in one list, and a static witness fails when an exported gitx function is unclassified, when a classified name no longer exists, when a verb that reaches a mutating gitx function has no declaration, or when a declaration names a verb that reaches none.", evidence: [static], anchor: ac-1 }
constraints:
  - { id: co-1, text: "The parent's constraints bind this story unchanged: no new git primitive and no widened declaration without an owner-visible decision (co-1), the declaration is data checked by the gate (co-2), every test is hermetic over fixturegit repositories with pushes only to a local bare remote (co-3), and recovery's semantics do not change (co-4).", anchor: co-1 }
decisions:
  - { id: dc-1, text: "The registry lives in internal/writescope and is returned by a function, never held in a package variable (docs/ground-rules.md), the same shape as the CLI-verb and MCP-tool inventories (parent dc-4). Its values are typed, so an unknown field cannot compile and an unknown enum value fails its Validate.", anchor: dc-1 }
  - { id: dc-2, text: "Reachability is computed statically from the module's source, with interface calls resolved to every implementation, over the entry points the CLI-verb inventory, the MCP-tool inventory, and the workbench's action and route handlers define. A new module dependency for this needs a ledger entry before it is added; the standard library's go/parser and go/types are enough.", anchor: dc-2 }
  - { id: dc-3, text: "The census of 2026-09-30 is the starting set of declarations, and each one describes what its ritual does at this story's base, read through parent dc-7's field readings. A ritual whose observed behavior is a defect is declared as it is and fixed in spec/ritual-effect-witness; its declaration is narrowed in the same change as the fix, never widened to pass (parent co-1).", anchor: dc-3 }
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
unknown field or value fails closed. Every exported internal/gitx function is classified as mutating or read-only in one
list, and a static witness fails when an exported gitx function is unclassified, when a classified name no longer
exists, when a verb that reaches a mutating gitx function has no declaration, or when a declaration names a verb that
reaches none.

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
over the entry points the CLI-verb inventory, the MCP-tool inventory, and the workbench's action and route handlers
define. A new module dependency for this needs a ledger entry before it is added; the standard library's go/parser and
go/types are enough.

## dc-3

The census of 2026-09-30 is the starting set of declarations, and each one describes what its ritual does at this
story's base, read through parent dc-7's field readings. A ritual whose observed behavior is a defect is declared as it
is and fixed in spec/ritual-effect-witness; its declaration is narrowed in the same change as the fix, never widened to
pass (parent co-1).

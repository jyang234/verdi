---
id: spec/workbench-directory
kind: spec
title: "Workbench Directory"
owners: [platform-team]
class: feature
status: draft
problem: { text: "verdi serve is bound to a working tree, so it is bound to a branch. Every draft in progress therefore lives at its own port, invented ad hoc, while the main address's home page silently under-reports the store: an operator looking at the directory cannot see the work most in motion. The distinction the operator needs is status — draft, accepted, active, terminal — and the tool is expressing it as network addresses.", anchor: problem }
outcome: { text: "One address is the whole directory. The home page enumerates the default branch and every design branch, groups by status, and links every board. Drafts open as authoring walls backed by their own branch's working tree; accepted specs stay sealed records; nothing the operator clicks ever mutates the state under another tab. The port pattern retires.", anchor: outcome }
acceptance_criteria:
  - { id: ac-2, text: "the home directory lists every spec on the default branch and every draft on a design branch, grouped and status-chipped, computed deterministically from git refs", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "opening a draft board serves its design branch working tree in authoring mode without disturbing any other board or the serving checkout", evidence: [behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "the one-writer law holds: a single serve process owns every working tree it writes", evidence: [behavioral], anchor: "#ac-4" }
  - { id: ac-5, text: "a design branch with no draft spec, or a branch deleted mid-session, degrades to a disclosed notice — never a dead link, never a silent absence", evidence: [behavioral], anchor: "#ac-5" }
  - { id: ac-6, text: "the mode law is unchanged: the same spec renders as a sealed record from the default branch and as an authoring wall from its own design branch", evidence: [behavioral], anchor: "#ac-6" }
stubs:
  - { slug: ref-index, acceptance_criteria: [ac-2] }
  - { slug: worktree-manager, acceptance_criteria: [ac-3, ac-4] }
  - { slug: directory-home, acceptance_criteria: [ac-2, ac-5] }
  - { slug: draft-boards, acceptance_criteria: [ac-3, ac-6] }
constraints:
  - { id: co-1, text: "managed worktrees live under the data zone, never committed; index computation reads refs and never switches a checkout", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "serve-managed lazy worktrees over guarded checkout-switching: opening a directory entry must never mutate the state under another tab — shared-state surprise is the silent-loss family", anchor: "#dc-1" }
  - { id: dc-2, text: "the directory groups by status — drafts in progress, accepted-pending-build, active components, terminal — the status is the distinction, never the address", anchor: "#dc-2" }
  - { id: dc-3, text: "one address: the per-draft port pattern is retired the day this lands", anchor: "#dc-3" }
  - { id: dc-4, text: "managed worktrees are reclaimed by verdi gc on the ratified gc signals — a branch merged (tip is an ancestor of the default-branch tip) or deleted (absent) is reclaimable; directory reads never delete and there is no background daemon; a worktree with uncommitted changes is never reclaimed but disclosed and kept", anchor: "#dc-4" }
open_questions:
  - { id: oq-2, text: "once the PR flow lands a remote (round-6 work): do remote design branches join the directory enumeration, and how are they told apart from local drafts? Carried with a recommended shape (see body) pending the remote", anchor: "#oq-2" }
---
# Workbench Directory

## Problem

verdi serve is bound to a working tree, so it is bound to a branch. Every
draft in progress therefore lives at its own port, invented ad hoc, while
the main address's home page silently under-reports the store: an
operator looking at the directory cannot see the work most in motion.
The distinction the operator needs is status — draft, accepted, active,
terminal — and the tool is expressing it as network addresses.

## Outcome

One address is the whole directory. The home page enumerates the default
branch and every design branch, groups by status, and links every board.
Drafts open as authoring walls backed by their own branch's working tree;
accepted specs stay sealed records; nothing the operator clicks ever
mutates the state under another tab. The port pattern retires.

## ac-2

the home directory lists every spec on the default branch and every draft on a design branch, grouped and status-chipped, computed deterministically from git refs

## ac-3

opening a draft board serves its design branch working tree in authoring mode without disturbing any other board or the serving checkout

## ac-4

the one-writer law holds: a single serve process owns every working tree it writes

## ac-5

a design branch with no draft spec, or a branch deleted mid-session, degrades to a disclosed notice — never a dead link, never a silent absence

## ac-6

the mode law is unchanged: the same spec renders as a sealed record from the default branch and as an authoring wall from its own design branch

## co-1

managed worktrees live under the data zone, never committed; index computation reads refs and never switches a checkout

## dc-1

serve-managed lazy worktrees over guarded checkout-switching: opening a directory entry must never mutate the state under another tab — shared-state surprise is the silent-loss family

## dc-2

the directory groups by status — drafts in progress, accepted-pending-build, active components, terminal — the status is the distinction, never the address

## dc-3

one address: the per-draft port pattern is retired the day this lands

## dc-4

A managed worktree is derived state under the data zone, created lazily by
the serve process, so the serve process owns its reclamation — but reads
never delete (a worktree vanishing under an open tab is the surprise
mutation dc-1 forbids), and there is no background daemon. Reclamation
reuses verdi's ratified gc signals: a managed worktree whose branch is
merged (its tip is an ancestor of the default-branch tip) or deleted
(absent) is reclaimable, and `verdi gc` reaps it — the same explicit
reaper that already prunes the corpus cache. A worktree carrying
uncommitted changes is never reclaimed: it is disclosed in the directory
and kept until the human resolves it — three-valued honesty applied to
cleanup; clean-and-merged is safe to drop, dirty is disclosed and held.

## oq-2

Carried, not resolved: this binds only once the PR flow lands a remote,
which does not exist yet (round-6 work), and deciding against an unbuilt
mechanism is what the invention discipline forbids. Recommended shape to
inherit when the remote lands: remote design branches DO join the
directory, keyed by status per dc-2 — a local design branch with no MR is
a local-authoring draft; a remote branch with an open MR is "in review"
and renders as the review mirror (the mode law: an open MR wins as review
mode). Enumeration sources differ and are disclosed — local drafts from
local refs, remote ones from the forge port's ListOpenMRs — and a forge
that cannot be reached is disclosed, never silently dropped.

---
id: spec/residue-reclamation
kind: spec
title: "Residue Reclamation"
owners: [platform-team]
class: feature
status: draft
problem: { text: "spec/closure-hygiene shipped verdi audit a third report section (== Closure hygiene audit ==) that surveys workspace-wide residue — every local branch merged into the default branch and never deleted, and every git worktree registered against the repository outside the primary checkout, each named with its own merge/clean/managed state — but that story is DETECTION-ONLY: spec/assurance-integrity's own co-2 explicitly declined to carry reclamation (\"reclamation gets its own future AC once, and only once, a story is ready to build it\"), and closure-hygiene's own dc-5 recorded why nothing reclaims any of it yet — verdi-store-layout's Garbage collection section, read in full at the time, licensed only pruning inside .verdi/data/ (derived/, cache) and explicitly disclaimed the committed zone and mutable/; unmanaged verdi-wt/ worktrees are sibling directories entirely OUTSIDE the git repository, and local branches are git-ref state, never a store path — neither mentioned, licensed, nor forbidden, a genuine gap dc-5 held was not self-serve invention-ledger territory. As of 2026-07-20 (main/HEAD cda3ec625a276db2cc66aa0603b62c4e8649cd60), this repository's own verdi audit names exactly what that gap leaves stranded: 159 local branches whose tip is an ancestor of the default branch tip, never deleted, and 33 registered git worktrees outside the primary checkout — most already both merged and clean, several sitting on close/<name> branches whose own ritual finished the moment their build merged. verdi-store-layout's Garbage collection section has since gained a fourth bullet under the ratification flow dc-5 asked for (owner decision 2026-07-20, verbatim \"I ratify the amendment\"; 08-revision-notes.md §External-assessment round — gc unmanaged reclamation (2026-07-20); ledger R4-I-79): verdi gc may now, on explicit opt-in, prune a LOCAL branch and its worktree (if any) when the branch is fully merged, its worktree is clean, and the worktree is not the primary checkout, disclosing verbatim what it did and did not touch. The license exists. Nothing yet uses it.", anchor: problem }
outcome: { text: "verdi gc gains an opt-in reclamation mode that acts on exactly what the ratified amendment licenses and nothing more: for every LOCAL branch and its worktree (if any) that spec/closure-hygiene's own survey already computes as fully merged, clean, and not the primary checkout, the mode either prints the full plan (the default) or, given explicit further opt-in, executes it — worktree first, then its branch, each removal backed by git's own independent refusal as a second guard, each item's outcome (reclaimed, with its branch's tip commit for recovery, or kept, with its one-line reason) disclosed whether or not anything is actually removed. Nothing unmerged, dirty, or primary is ever touched, and no other verdi gc behavior changes. Delivered in full by this feature's one stub story, spec/gc-reclaim.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "verdi gc gains a reclamation mode, invoked on explicit opt-in, that acts within the newly-ratified license (verdi-store-layout §Garbage collection's fourth bullet, ledger R4-I-79) and only within it: every LOCAL branch and its worktree (if any) that is fully merged into the default branch, carries no uncommitted changes, and is neither the primary checkout nor the one running the sweep itself is eligible for removal; every other branch and worktree — unmerged, dirty, in a state that cannot be resolved, detached, already under verdi gc's existing managed-worktree jurisdiction, or the checkout running the sweep itself — is kept, and every kept item's reason is named, never omitted. By default the mode only prints this plan; a second, explicit opt-in is required before anything is actually touched, and even then every removal is independently re-checked by git's own refusal before it happens. Every branch actually removed prints its last commit, so what was reclaimed is always recoverable by SHA; every run, whether or not it removes anything, discloses exactly what it did and did not touch. Delivered in full by this feature's one stub story, spec/gc-reclaim.", evidence: [static, behavioral, attestation], anchor: ac-1 }
decisions:
  - { id: dc-1, text: "This feature exists solely to give the reclamation license 08-revision-notes.md's External-assessment round already ratified (R4-I-79) a resolving parent AC for the one story that fully implements it — not to reopen the external-assessment round or grow into a broader gc redesign. The nearest relative, spec/assurance-integrity (closed), considered and explicitly declined exactly this outcome at its own acceptance: its co-2 named the gap by number (\"reclamation gets its own future AC once, and only once, a story is ready to build it\") precisely so a future story would not have to reopen an already-closed feature to claim it. No other currently active feature covers gc or branch/worktree lifecycle by subject — a check of all five (code-health: forge/tracker transport hygiene; disclosure-legibility: claim-visibility vocabulary; public-showcase: showcase corpus and README; ritual-integrity: judge-backed verb ergonomics; scoping-canvas: the feature-wall scoping board) against their own declared outcomes turned up nothing adjacent, the same subject-unrelatedness check the prior parentless-story round (R4-I-58/R4-I-67/R4-I-69) ran before authoring spec/assurance-integrity itself. Scope is fixed at exactly the one stub's outcome: verdi-store-layout's other two Garbage-collection bullets are untouched by this feature (co-1). A future, genuinely new gap gets its own feature or a ratified amendment, never silently folded in here after acceptance. Keeping this feature small and closeable is the deliberate point, mirroring spec/assurance-integrity's own dc-1 (owner directive, 2026-07-19): it closes the moment its one story closes, not months later absorbing adjacent findings.", anchor: dc-1 }
constraints:
  - { id: co-1, text: "verdi-store-layout's other two Garbage-collection bullets — pruning derived/<ref>/ past derived.retention_days, and pruning stale cache entries — remain unimplemented and are out of this feature's scope; spec/worktree-manager's own dc-5 already recorded them as awaiting their own future story, and this feature does not claim them either. This feature's one AC covers only the fourth, newly-ratified bullet: unmanaged local-branch-and-worktree reclamation. verdi gc's scope-disclosure line continues to name both remaining gaps as not-run on every invocation (spec/gc-reclaim ac-3).", anchor: co-1 }
stubs:
  - { slug: gc-reclaim, acceptance_criteria: [ac-1] }
---

# Residue Reclamation

## Problem

`spec/closure-hygiene` shipped `verdi audit` a third report section
(`== Closure hygiene audit ==`) that surveys workspace-wide residue — every
local branch merged into the default branch and never deleted, and every
git worktree registered against the repository outside the primary
checkout, each named with its own merge/clean/managed state — but that
story is **detection-only**: `spec/assurance-integrity`'s own CO-2
explicitly declined to carry reclamation ("reclamation gets its own future
AC once, and only once, a story is ready to build it"), and
closure-hygiene's own DC-5 recorded why nothing reclaims any of it yet:

`verdi-store-layout`'s Garbage collection section, read in full at the
time, licensed only pruning inside `.verdi/data/` (`derived/`, cache) and
explicitly disclaimed the committed zone and `mutable/`. Unmanaged
`verdi-wt/` worktrees are sibling directories entirely OUTSIDE the git
repository, and local branches are git-ref state, never a store path —
neither mentioned, licensed, nor forbidden by that section: a genuine gap,
which DC-5 held was not self-serve invention-ledger territory.

As of 2026-07-20 (main/HEAD `cda3ec625a276db2cc66aa0603b62c4e8649cd60`),
this repository's own `verdi audit` names exactly what that gap leaves
stranded: 159 local branches whose tip is an ancestor of the default
branch tip, never deleted, and 33 registered git worktrees outside the
primary checkout — most already both merged and clean, several sitting on
`close/<name>` branches (e.g. `close/model-digest`, `close/model-schema`,
`close/scaffold-templates`, `close/vocabulary-surfaces`, each checked out
in its own now-idle `-build` worktree) whose own ritual finished the
moment their build merged. Four more `close/<name>` branches
(`close/attest-helper`, `close/close-preflight`, `close/disposition-verb`,
`close/home-status-glance`) remain in the survey too, but UNMERGED —
closure-hygiene's own `superseded-elsewhere` classification (their archive
move already landed some other way) says nothing about whether the branch
itself is merged, and none of these four is.

`verdi-store-layout`'s Garbage collection section has since gained a
fourth bullet under the ratification flow DC-5 asked for (owner decision
2026-07-20, verbatim "I ratify the amendment";
`08-revision-notes.md` §External-assessment round — gc unmanaged
reclamation (2026-07-20); ledger R4-I-79): `verdi gc` may now, on explicit
opt-in, prune a LOCAL branch and its worktree (if any) when the branch is
fully merged, its worktree is clean, and the worktree is not the primary
checkout, disclosing verbatim what it did and did not touch.

The license exists. Nothing yet uses it.

## Outcome

`verdi gc` gains an opt-in reclamation mode that acts on exactly what the
ratified amendment licenses and nothing more: for every LOCAL branch and
its worktree (if any) that `spec/closure-hygiene`'s own survey already
computes as fully merged, clean, and not the primary checkout, the mode
either prints the full plan (the default) or, given explicit further
opt-in, executes it — worktree first, then its branch, each removal backed
by git's own independent refusal as a second guard, each item's outcome
(reclaimed, with its branch's tip commit for recovery, or kept, with its
one-line reason) disclosed whether or not anything is actually removed.

Nothing unmerged, dirty, or primary is ever touched, and no other
`verdi gc` behavior changes.

Delivered in full by this feature's one stub story, `spec/gc-reclaim`.

## AC-1

`verdi gc` gains a reclamation mode, invoked on explicit opt-in, that acts
within the newly-ratified license (`verdi-store-layout` §Garbage
collection's fourth bullet, ledger R4-I-79) and only within it:

Every LOCAL branch and its worktree (if any) that is fully merged into the
default branch, carries no uncommitted changes, and is neither the primary
checkout nor the one running the sweep itself is eligible for removal.
Every other branch and worktree — unmerged, dirty, in a state that cannot
be resolved, detached, already under `verdi gc`'s existing
managed-worktree jurisdiction, or the checkout running the sweep itself —
is kept, and every kept item's reason is named, never omitted.

By default the mode only prints this plan; a second, explicit opt-in is
required before anything is actually touched, and even then every removal
is independently re-checked by git's own refusal before it happens. Every
branch actually removed prints its last commit, so what was reclaimed is
always recoverable by SHA; every run, whether or not it removes anything,
discloses exactly what it did and did not touch.

Delivered in full by this feature's one stub story, `spec/gc-reclaim`.

## DC-1

This feature exists solely to give the reclamation license
`08-revision-notes.md`'s External-assessment round already ratified
(R4-I-79) a resolving parent AC for the one story that fully implements
it — not to reopen the external-assessment round or grow into a broader
`gc` redesign.

The nearest relative, `spec/assurance-integrity` (closed), considered and
explicitly declined exactly this outcome at its own acceptance: its CO-2
named the gap by number ("reclamation gets its own future AC once, and
only once, a story is ready to build it") precisely so a future story
would not have to reopen an already-closed feature to claim it.

No other currently active feature covers `gc` or branch/worktree lifecycle
by subject. Checked against their own declared outcomes: `code-health`
(forge/tracker transport hygiene), `disclosure-legibility` (claim-
visibility vocabulary), `public-showcase` (showcase corpus and README),
`ritual-integrity` (judge-backed verb ergonomics), and `scoping-canvas`
(the feature-wall scoping board) — five active features, none adjacent by
subject. The same subject-unrelatedness check the prior parentless-story
round (R4-I-58/R4-I-67/R4-I-69) ran before authoring `spec/assurance-
integrity` itself.

Scope is fixed at exactly the one stub's outcome: `verdi-store-layout`'s
other two Garbage-collection bullets are untouched by this feature (CO-1).
A future, genuinely new gap gets its own feature or a ratified amendment,
never silently folded in here after acceptance.

Keeping this feature small and closeable is the deliberate point,
mirroring `spec/assurance-integrity`'s own DC-1 (owner directive,
2026-07-19): it closes the moment its one story closes, not months later
absorbing adjacent findings.

## CO-1

`verdi-store-layout`'s other two Garbage-collection bullets — pruning
`derived/<ref>/` past `derived.retention_days`, and pruning stale cache
entries — remain unimplemented and are out of this feature's scope;
`spec/worktree-manager`'s own DC-5 already recorded them as awaiting their
own future story, and this feature does not claim them either.

This feature's one AC covers only the fourth, newly-ratified bullet:
unmanaged local-branch-and-worktree reclamation. `verdi gc`'s
scope-disclosure line continues to name both remaining gaps as not-run on
every invocation (`spec/gc-reclaim` AC-3).

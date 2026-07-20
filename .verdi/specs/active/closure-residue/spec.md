---
id: spec/closure-residue
kind: spec
title: "Closure Residue"
owners: [platform-team]
class: feature
status: draft
problem: { text: "verdi's own store on main already carries the residue its closure ritual is meant to prevent. close/showcase-corpus-renovation's tip (24214fd) already moved spec/showcase-corpus-renovation to archive/, but that branch never merged, so the spec still reads accepted-pending-build in specs/active/ on main — a closure ritual that ran and never landed. Four more close/<name> branches (attest-helper, close-preflight, disposition-verb, home-status-glance) sit unmerged even though archive/<name> already exists on main for every one of them — the closure landed through a different commit history, and the branch is now pure leftover. spec/code-health sits accepted-pending-build though every story stub it declared (forge-transport, shared-homes, fail-loud, file-topics) is realized by a closed, merged story. And workspace-wide, 153 of 167 local branches are fully merged into main and were never deleted, spread across 28 registered git worktrees, most sitting on work long since merged or archived. `verdi close` (03 §Closure ritual) commits its archival output to whatever branch it runs on and stops there (spec/close-verb dc-3) — nothing checks whether that output ever reached main, and nothing distinguishes a spec genuinely awaiting build from one whose closure simply never landed. `verdi gc` (spec/worktree-manager) reclaims managed worktrees only (`.verdi/data/worktrees/`) — every worktree and branch outside that narrow slice accumulates forever, invisible to anyone who does not happen to run `git worktree list` or `git branch --merged` by hand.", anchor: problem }
outcome: { text: "verdi can honestly answer what closure work is actually done versus merely left over. A detection pass reports every git-reality-versus-spec-status contradiction and every stranded closure-ritual branch with a concrete, named witness — three-valued honest: where git state cannot decide a category, it says so, never guesses. Once licensed by a ratification amendment to `verdi-store-layout`'s Garbage collection section (this feature's own oq-1), a conservative reclamation pass mechanically removes only unmanaged worktrees and local branches that are provably dead — fully merged, clean, and never the primary checkout — leaving everything else named and untouched in its own report.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "verdi reports, for the current checkout, every active-zone spec whose declared status contradicts what git already shows happened — a stranded closure ritual (a close/<name> branch whose tip already moved <name> to archive/, unmerged into the default branch) and a feature whose every declared stub is realized by a closed, merged story though the feature itself is still accepted-pending-build — plus every close/<name> branch unmerged into the default branch, each finding naming the branch, its tip commit, and the exact contradiction; where git state cannot decide a category nothing is asserted", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "once licensed by a ratification amendment to verdi-store-layout's Garbage collection section, verdi mechanically reclaims only unmanaged worktrees and local branches that are provably dead: the branch is fully merged into the default branch, its worktree (if any) is clean, it is never the primary checkout, and reclamation runs only on explicit opt-in — every run naming verbatim what it did and did not touch, mirroring spec/worktree-manager's own dc-4/dc-5 disclosure idiom", evidence: [static, behavioral], anchor: ac-2 }
decisions:
  - { id: dc-1, text: "two-phase delivery behind the honest-now/licensed-later split ac-1/ac-2 already state. Detection (ac-1) is a pure read/report extension of an already-recognized verb (`verdi audit`, R4-I-10) and ships as this feature's one declared stub story, spec/closure-hygiene. Reclamation (ac-2) touches git state outside anything verdi-store-layout's ratified Garbage collection section currently licenses (quoted and argued in spec/closure-hygiene's own dc-5/oq-1) and is deliberately left UNSTUBBED here — stubbing a story against an AC this feature cannot yet license would misstate readiness rather than record it honestly. A later revision of this feature adds ac-2's stub once the amendment lands; 03 §Stub reconciliation's own withdrawn-with-note path keeps this feature closeable on ac-1 alone should ac-2 ever be declined instead of ratified", anchor: dc-1 }
constraints:
  - { id: co-1, text: "reclamation is conservative by construction, inherited by every implementing story (03 §Object model: a feature constraint inherits downward, checked wherever relevant). Never auto-archive a spec — archival is the closure ritual's own output; a stale status is either an unfinished ritual to surface or a defect to fix by hand, never a condition reclamation silently resolves. Never auto-merge a stranded close/<name> branch — merge is acceptance, an owner-gated act. Never delete anything unmerged or dirty. Detection never performs a git-mutating call", anchor: co-1 }
open_questions:
  - { id: oq-1, text: "does reclaiming unmanaged worktrees and merged local branches fall within any already-ratified authority, or does it require a ratification-flow amendment to verdi-store-layout's Garbage collection section (a component spec, status: active) before ac-2 can be built? spec/closure-hygiene's own dc-5 argues the latter, quotes the section verbatim, and drafts candidate amendment language", anchor: oq-1 }
stubs:
  - { slug: closure-hygiene, acceptance_criteria: [ac-1] }
---

# Closure Residue

## Problem

Verdi's own store on main already carries the residue its closure ritual is
meant to prevent.

`close/showcase-corpus-renovation`'s tip (`24214fd`, "close: archive
spec/showcase-corpus-renovation (jira:VERDI-22)") already moved
`spec/showcase-corpus-renovation` to `archive/`, but that branch never
merged into main. The spec still reads `accepted-pending-build` in
`specs/active/` on main today — a closure ritual that ran to completion and
never landed.

Four more `close/<name>` branches — `close/attest-helper`,
`close/close-preflight`, `close/disposition-verb`, `close/home-status-glance`
— sit unmerged into main right now, even though `archive/<name>` already
exists on main for every one of them. Their closures landed through a
different commit history (a squash, a manual replay, or a differently-named
merge); the branch itself is now pure leftover, structurally
indistinguishable from a genuinely stranded ritual without checking whether
the archive move it carries is already present elsewhere.

`spec/code-health` sits `accepted-pending-build` on main though every story
stub it declared at scaffold time — `forge-transport`, `shared-homes`,
`fail-loud`, `file-topics` — is realized by a closed, merged story
(`archive/forge-transport`, `archive/shared-homes`, `archive/fail-loud`,
`archive/file-topics` all exist on main). Stub reconciliation would likely
pass; nobody has run `verdi close spec/code-health`.

Workspace-wide, 153 of 167 local branches are fully merged into main and
were never deleted, spread across 28 git worktrees registered against this
repository (one primary checkout, 27 more under `verdi-wt/`) — most sitting
on design/feature/close branch trios for specs long since archived.

`verdi close` (03 §Closure ritual) commits its archival output to whatever
branch it runs on and stops there — spec/close-verb's own dc-3: "this verb
stops at the branch... opening the MR is the human's act." Nothing checks
whether that output ever reached main, and nothing distinguishes a spec
genuinely awaiting build from one whose closure simply never landed.
`verdi gc` (spec/worktree-manager) reclaims managed worktrees only, under
`.verdi/data/worktrees/` — every worktree and branch outside that narrow
slice accumulates forever, invisible to anyone who does not happen to run
`git worktree list` or `git branch --merged` by hand.

## Outcome

Verdi can honestly answer what closure work is actually done versus merely
left over.

A detection pass reports every git-reality-versus-spec-status contradiction
and every stranded closure-ritual branch with a concrete, named witness —
three-valued honest: where git state cannot decide a category, it says so,
never guesses.

Once licensed by a ratification amendment to `verdi-store-layout`'s
Garbage collection section (this feature's own OQ-1), a conservative
reclamation pass mechanically removes only unmanaged worktrees and local
branches that are provably dead — fully merged, clean, and never the
primary checkout — leaving everything else named and untouched in its own
report.

## AC-1

Verdi reports, for the current checkout, every active-zone spec whose
declared status contradicts what git already shows happened, plus every
stranded closure-ritual branch — the two residue patterns this problem
statement witnesses (a stranded ritual, a stub-complete-but-unclosed
feature) and every `close/<name>` branch unmerged into the default branch,
classified. Every finding names the branch, its tip commit, and the exact
contradiction. Where git state cannot decide a category — for instance, no
resolvable default branch — nothing is asserted for it.

Evidence: static + behavioral.

## AC-2

Once licensed by a ratification amendment to `verdi-store-layout`'s
Garbage collection section, verdi mechanically reclaims only unmanaged
worktrees and local branches that are provably dead: the branch is fully
merged into the default branch, its worktree (if any) is clean, it is
never the primary checkout, and reclamation runs only on explicit opt-in.
Every run names verbatim what it did and did not touch, mirroring
spec/worktree-manager's own dc-4/dc-5 disclosure idiom.

Evidence: static + behavioral.

## DC-1

Two-phase delivery behind the honest-now/licensed-later split AC-1/AC-2
already state.

Detection (AC-1) is a pure read/report extension of an already-recognized
verb (`verdi audit`, R4-I-10) and ships as this feature's one declared stub
story, `spec/closure-hygiene`.

Reclamation (AC-2) touches git state outside anything `verdi-store-layout`'s
ratified Garbage collection section currently licenses (quoted and argued
in `spec/closure-hygiene`'s own DC-5, its sibling story spec) and is deliberately left
UNSTUBBED here — stubbing a story against an AC this feature cannot yet
license would misstate readiness rather than record it honestly. A later
revision of this feature adds AC-2's stub once the amendment lands;
03 §Stub reconciliation's own withdrawn-with-note path keeps this feature
closeable on AC-1 alone should AC-2 ever be declined instead of ratified.

## CO-1

Reclamation is conservative by construction, inherited by every
implementing story (03 §Object model: a feature constraint inherits
downward, checked wherever relevant, never assigned to one).

Never auto-archive a spec — archival is the closure ritual's own output; a
stale status is either an unfinished ritual to surface or a defect to fix
by hand, never a condition reclamation silently resolves. Never auto-merge
a stranded `close/<name>` branch — merge is acceptance, an owner-gated act.
Never delete anything unmerged or dirty. Detection never performs a
git-mutating call.

## OQ-1

Does reclaiming unmanaged worktrees and merged local branches fall within
any already-ratified authority, or does it require a ratification-flow
amendment to `verdi-store-layout`'s Garbage collection section (a component
spec, status: active) before AC-2 can be built?

`spec/closure-hygiene`'s own DC-5 argues the latter, quotes the section
verbatim, and drafts candidate amendment language for the owner's review.

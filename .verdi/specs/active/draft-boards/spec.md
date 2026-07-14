---
id: spec/draft-boards
kind: spec
title: "Draft Boards"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-21
problem: { text: "the board serves exactly one working tree — boardSpecServer reads specs/active/ under the serving checkout's root — so a draft's authoring wall exists only in a checkout sitting on that design branch: the per-draft port pattern. The mode law already renders authoring vs read-only purely from branch state; what is missing is ROUTING — one address that reaches every draft's own branch tree without disturbing any other board or the serving checkout", anchor: "#problem" }
outcome: { text: "clicking a draft in the directory opens that spec's authoring wall served from its design branch's managed worktree, under one address grammar (/b/<branch-escaped>/board/spec/<name>): two boards from two branches are usable in two tabs simultaneously, nothing an operator does under one address mutates the tree under another, the mode law is unchanged — the same spec sealed from the default branch, authoring from its own design branch — and the per-draft port pattern retires", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "GET /b/<branch-escaped>/board/spec/<name> serves that spec's board from the managed worktree for local design branch <branch> (consumed from the worktree-manager story's seam) in authoring mode per the unchanged mode law, and the board's sub-routes (fragment, api actions, peek, pinsearch) work identically beneath the prefix — the existing board server rooted at the worktree, never a second board implementation", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "two draft boards from two design branches are open and usable in two tabs simultaneously: an authoring edit through one lands only in its own branch's managed worktree and never disturbs the other board or the serving checkout, whose working tree stays clean throughout", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "the mode law is unchanged and no new mode exists: the same spec renders as a sealed read-only record at its default-branch address and as an authoring wall at its design-branch address, both reachable at once — the authoring/read-only/review decision stays the existing branch-state computation, applied per branch instance", evidence: [static, behavioral], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/workbench-directory#ac-3" }
  - { type: implements, ref: "spec/workbench-directory#ac-6" }
decisions:
  - { id: dc-1, text: "URL grammar: a /b/{branch}/ prefix in front of the existing board addresses — the branch rides one path segment with its slashes percent-encoded (%2F, Go 1.22+ ServeMux segment semantics), and beneath the prefix the existing board route table is served rooted at the branch's tree, one board-server instance per branch — the smallest shape consistent with internal/workbench's routes: no query-param mode switch, no new port, no second route table", anchor: "#dc-1" }
  - { id: dc-2, text: "first open is lazy and synchronous: the managed worktree is cut on first request through the worktree-manager seam and the request blocks until the cut completes — no interstitial, no spinner infrastructure, no background daemon (feature dc-4); a failed cut renders a disclosed error page naming the failure, never a dead link; subsequent opens reuse the worktree", anchor: "#dc-2" }
  - { id: dc-3, text: "the port pattern retires (feature dc-3): unprefixed addresses keep serving the serving checkout's own tree, semantics unchanged — under the canonical default-branch deployment that IS the sealed-record view — while every draft is reached under /b/ from the same one address; serve gains no per-draft ports and no new port flags", anchor: "#dc-3" }
  - { id: dc-4, text: "feature dc-5 routed: a /b/ branch that resolves only to a remote-tracking ref renders sealed — read-only, remoteness disclosed, no worktree cut, no local branch minted; a /b/ branch that resolves to no ref at all renders the disclosed notice page (HTTP 404, a way back) — the same stale-entry surface the directory-home story discloses; neither case is a new mode, only routing to the modes that exist", anchor: "#dc-4" }
constraints:
  - { id: co-1, text: "the one-writer law is the worktree-manager story's contract (feature ac-4), consumed here: per-branch boards obtain worktrees only through that seam, introduce no second writer and no worktree lifecycle of their own, and every managed worktree lives under the data zone, never committed (feature co-1)", anchor: "#co-1" }
  - { id: co-2, text: "no surprise mutation (feature dc-1): serving a per-branch board never switches any checkout and never mutates the state under another tab; reads never delete (feature dc-4); no network in any test — fixturegit stores and hermetic doubles, with the Playwright suite under e2e/ as the behavioral evidence register", anchor: "#co-2" }
frozen: { at: 2026-07-14, commit: 26646b193ba1be5466d3a7158e56d203bb7a08d2, stub_matched: true }
---
# Draft Boards

## Problem

The board serves exactly one working tree. boardSpecServer reads
`specs/active/` under the serving checkout's root, and the mode law keys
authoring vs read-only purely off that checkout's branch state
(internal/workbench/handler.go: "the board renders authoring/read-only
purely from branch state"). So a draft's authoring wall exists only in a
checkout that is itself sitting on the draft's design branch — which is
exactly the per-draft port pattern: a second `verdi serve`, a second port,
per draft in motion (workbench-directory#problem). The projection, the
mode computation, and the authoring surface all already exist and are
correct; what is missing is ROUTING — one address that reaches every
draft's own branch tree without disturbing any other board or the serving
checkout.

## Outcome

Clicking a draft in the directory opens that spec's authoring wall served
from its design branch's managed worktree, under one address grammar:
`/b/<branch-escaped>/board/spec/<name>`. Two boards from two branches are
usable in two tabs simultaneously; an edit through one lands only in its
own branch's worktree and nothing an operator does under one address
mutates the tree under another. The mode law is unchanged — the same spec
renders as a sealed record from the default branch and as an authoring
wall from its own design branch — and the per-draft port pattern retires:
one serve, one address, every draft.

## AC-1

`GET /b/<branch-escaped>/board/spec/<name>` serves that spec's board from
the managed worktree for local design branch `<branch>`, obtained through
the worktree-manager story's seam ("a managed worktree for branch X" is
that story's contract; this story consumes it). The board renders in
authoring mode per the unchanged mode law — a draft spec on its own design
branch — and the board's sub-routes (`fragment`, `api/{action}`, `peek`,
`pinsearch`) work identically beneath the prefix, because what is mounted
there is the existing board server rooted at the worktree, never a second
board implementation (dc-1). Evidence: static (the prefix route constructs
the existing board server over the seam-obtained root; no duplicated
projection or mode logic) and behavioral (a Playwright e2e opens a draft
under /b/ and exercises the wall and its sub-routes).

## AC-2

The parent's ac-3, proven as a person would experience it: two draft
boards from two design branches open in two tabs and both stay usable —
simultaneously, not alternately. An authoring edit through one board lands
only in its own branch's managed worktree; the other board re-renders
unchanged, and the serving checkout's working tree stays clean throughout —
opening and editing drafts never disturbs it (feature dc-1's
no-surprise-mutation law). Evidence: behavioral (a Playwright e2e drives
two tabs against two fixture design branches and asserts edit isolation
plus a clean serving checkout).

## AC-3

The parent's ac-6, the mode law unchanged. The same spec renders as a
sealed read-only record at its default-branch address (the unprefixed
`/board/spec/<name>`, dc-3) and as an authoring wall at its design-branch
address (`/b/<branch-escaped>/board/spec/<name>`), and both are reachable
at once. No new mode exists: the authoring/read-only/review decision stays
the existing branch-state computation — status draft on a non-default
branch means authoring — now applied per branch instance because each
instance is rooted in its own tree. This story is routing, not a mode.
Evidence: static (no new mode value; the mode computation is the existing
one, consumed per instance) and behavioral (an e2e renders the same spec
sealed at its unprefixed address and authoring under /b/ in the same
session).

## DC-1

URL grammar: a `/b/{branch}/` prefix in front of the existing board
addresses. The branch rides one path segment with its slashes
percent-encoded (`design/foo` → `design%2Ffoo`, Go 1.22+ ServeMux segment
semantics decode it back), so `/b/design%2Ffoo/board/spec/foo` is the
draft's authoring address. Beneath the prefix the existing board route
table is served rooted at the branch's tree — one board-server instance
per branch, constructed over the seam-obtained root — which is why every
sub-route works unchanged. This is the smallest honest shape consistent
with internal/workbench's routes: the alternative query-param form
(`?branch=`) would have to be threaded through every fragment/api/asset
request the board page issues, and a per-draft port is the very pattern
this feature retires. Only the directory mints these links (directory-home
dc-3); humans never type them.

## DC-2

First open is lazy and synchronous. The managed worktree is cut on first
request through the worktree-manager seam, and that first request blocks
until the cut completes — a `git worktree add` at this store's scale, not
worth an interstitial, a spinner, or any background machinery (feature
dc-4: no background daemon). A failed cut renders a disclosed error page
naming the failure — never a dead link, never a silent 500. Subsequent
opens reuse the worktree and are as fast as the serving checkout's own
board. The directory shows nothing special for a not-yet-cut draft: the
latency is disclosed by this decision, not by a UI state.

## DC-3

The port pattern retires (feature dc-3). Unprefixed addresses keep serving
the serving checkout's own tree with semantics unchanged — the mode law
still reads that checkout's branch state, so under the canonical
deployment (one `verdi serve` on the default branch) the unprefixed
`/board/spec/<name>` IS the sealed-record view of every landed spec.
Every draft is reached under `/b/` from the same one address. No per-draft
ports, no new port flags on serve: the day this lands, the reason to run a
second serve per draft is gone.

## DC-4

Feature dc-5, routed. A `/b/` branch that resolves only to a
remote-tracking ref renders sealed: read-only, its remoteness disclosed in
the board chrome, no worktree cut and no local branch minted — managed
worktrees are cut from local branches only (feature dc-5 verbatim), and
silently minting a local branch on click would be the surprise mutation
feature dc-1 forbids. A `/b/` branch that resolves to no ref at all
renders the disclosed notice page — HTTP 404, a human-readable body, a
link back to the directory — the same stale-entry surface the
directory-home story discloses (its dc-5). Neither case is a new mode;
this decision only routes entries to the modes that already exist
(feature dc-5: "it only routes entries to the modes that already exist").

## CO-1

The one-writer law is the worktree-manager story's contract (feature
ac-4); this story consumes it and must not weaken it. Per-branch boards
obtain worktrees only through that seam — no second worktree lifecycle, no
second writer: the single serve process owns every working tree it writes,
and this routing adds instances of the existing board writer over
seam-owned trees, nothing else. Every managed worktree lives under the
data zone and is never committed (feature co-1).

## CO-2

No surprise mutation, hermetically proven. Serving a per-branch board
never switches any checkout and never mutates the state under another tab
(feature dc-1); reads never delete (feature dc-4). No network in any test:
per-branch routing and isolation are exercised over fixturegit stores and
hermetic doubles, and the Playwright suite under `e2e/` is the behavioral
evidence register — the two-tab isolation proof runs there, against the
real served binary.

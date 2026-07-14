---
id: spec/directory-home
kind: spec
title: "Directory Home"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-20
problem: { text: "the workbench home page is a single-checkout view: it lists what the serving working tree contains and nothing else, so every draft in progress on a design branch is invisible at the one address the operator actually visits — the work most in motion is exactly the work the directory under-reports (workbench-directory#problem), and the distinction the operator needs, status, is never the page's organizing structure", anchor: "#problem" }
outcome: { text: "GET / at the one serve address is the whole-store directory: it renders the computed directory index — every spec on the default branch and every draft on a design branch — grouped by status per the feature's dc-2, every entry status-chipped and linking to its board, disclosed by source, chipped in-review from the forge when an MR is open, and degrading every absence to a disclosed notice: never a dead link, never a silent absence", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "GET / renders the whole-store directory from the computed directory index (the ref-index story's seam, consumed — never re-derived): every spec on the default branch and every draft on a design branch appears exactly once, grouped by status per feature dc-2 (drafts in progress, accepted-pending-build, active components, terminal), every entry status-chipped and linking to its board address", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "every directory entry is disclosed by source — local design branch vs remote-tracking (feature dc-5); an entry whose branch carries an open MR is chipped in review from the forge port, and when the forge cannot be reached the chip degrades to a disclosed 'MR status unavailable' notice while the refs-computed directory still renders fully", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "a design branch with no draft spec renders as a disclosed notice entry naming the branch, and an entry whose branch was deleted mid-session resolves to a disclosed notice page with a way back — never a dead link, never a silent absence, never a directory-wide failure", evidence: [behavioral], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/workbench-directory#ac-2" }
  - { type: implements, ref: "spec/workbench-directory#ac-5" }
decisions:
  - { id: dc-1, text: "the directory IS the home page: GET / at the one serve address, replacing the current single-checkout active/archived listing in place — no new route, no second landing page (feature dc-3, one address); the surviving home affordances (disclosures pointer, services, grandfathered v0 boards) keep their sections", anchor: "#dc-1" }
  - { id: dc-2, text: "the page consumes the computed directory index through the sibling ref-index story's seam and performs no ref enumeration of its own — the handler renders index output; grouping keys off the index entry's status, never its address (feature dc-2)", anchor: "#dc-2" }
  - { id: dc-3, text: "link grammar consumed, not invented: default-branch entries keep today's unprefixed addresses (/board/spec/<name>, /a/spec/<name>); a design-branch entry links to its per-branch board address under the sibling draft-boards grammar (/b/<branch-escaped>/board/spec/<name>), behind which only a LOCAL design branch opens as an authoring wall while a remote-only branch renders sealed with its remoteness disclosed — feature dc-5's split is enforced by the routing story behind the one grammar, never by the directory minting different link shapes; the directory emits only addresses the routing story serves", anchor: "#dc-3" }
  - { id: dc-4, text: "the in-review chip is a per-render, non-blocking consultation of the forge port's ListOpenMRs — a second, non-ref source disclosed as such (feature dc-5); it never enters the deterministic index computation, and every failure degrades to the disclosed absence, never a blocked or delayed directory", anchor: "#dc-4" }
  - { id: dc-5, text: "error surfaces: an index-computation failure renders as a disclosed inline notice in a still-served page (the home page is never itself a dead end — the existing renderHome posture); a stale entry click resolves to a rendered disclosed notice page with HTTP 404 and a link back to the directory, never a bare NotFound", anchor: "#dc-5" }
constraints:
  - { id: co-1, text: "rendering the directory reads the computed index — refs only — and never switches a checkout, never cuts a worktree, never deletes one: a directory READ mutates nothing (feature co-1, dc-1, dc-4)", anchor: "#co-1" }
  - { id: co-2, text: "no network in any test: the in-review chip and its degradation are proven against hermetic doubles (httptest forge / canned fixtures), and the Playwright suite under e2e/ is the behavioral evidence register for every directory surface", anchor: "#co-2" }
frozen: { at: 2026-07-14, commit: 4a5a9e169801c3860ae1c5e90ac21512a6884f10, stub_matched: true }
---
# Directory Home

## Problem

The workbench home page (internal/workbench's `GET /`) is a single-checkout
view: it lists the active and archived specs the serving working tree
contains, and nothing else. A draft in progress on a design branch — the
work most in motion — is invisible at the one address the operator actually
visits; today it is reachable only by running a second `verdi serve` from a
second checkout on an ad-hoc port (workbench-directory#problem). And the
distinction the operator needs to navigate the store — draft, accepted,
active, terminal — appears only as a badge per row, never as the page's
organizing structure. The home page silently under-reports the store.

## Outcome

`GET /` at the one serve address is the whole-store directory. It renders
the computed directory index — every spec on the default branch and every
draft on a design branch — grouped by status per the feature's dc-2, every
entry status-chipped and linking to its board. Entries are disclosed by
source (local vs remote-tracking, feature dc-5), an open MR shows as an
in-review chip fed by the forge port, and every absence the page cannot
resolve — a branch with no draft, a branch deleted mid-session, an
unreachable forge — degrades to a disclosed notice. Never a dead link,
never a silent absence.

## AC-1

`GET /` renders the whole-store directory from the computed directory
index — the sibling ref-index story's seam, consumed as an input, never
re-derived (dc-2). Every spec on the default branch and every draft on a
design branch appears exactly once, grouped by status per feature dc-2:
drafts in progress, accepted-pending-build, active components, terminal.
The status is the distinction, never the address. Every entry carries a
status chip and links to its board address per dc-3. Evidence: static (the
home renderer consumes the index seam and contains no ref enumeration of
its own) and behavioral (a Playwright e2e over a fixture store spanning all
four groups sees the grouped, chipped, linked directory).

## AC-2

Every directory entry is disclosed by source: a local design branch and a
remote-tracking one are distinguishable on the page (feature dc-5). An
entry whose branch carries an open MR is chipped "in review" from the forge
port's ListOpenMRs — a second, non-ref source, disclosed as such — and when
the forge cannot be reached the chip degrades to a disclosed "MR status
unavailable" notice while the refs-computed directory still renders fully
(dc-4): the directory never depends on network reachability. Evidence:
behavioral (e2e/integration over a hermetic forge double shows the chip;
the same surface with the double unreachable shows the disclosed absence
and a complete directory).

## AC-3

The feature's ac-5 degradations, on this page. A design branch with no
draft spec renders as a disclosed notice entry naming the branch — listed,
explained, not linked as if a board existed. An entry whose branch is
deleted mid-session resolves, on click, to a rendered disclosed notice page
with a link back to the directory (dc-5) — never a dead link, never a
silent absence, and never a directory-wide failure: one unresolvable entry
cannot take down the page. Evidence: behavioral (e2e drives both shapes —
the empty branch and the deleted-after-render branch — and sees disclosed
notices, not broken responses).

## DC-1

The directory is the home page. `GET /` at the one serve address becomes
the whole-store directory, replacing the current single-checkout
active/archived listing in place — no new route, no second landing page,
because a second address for "the real directory" would re-create exactly
the fragmentation feature dc-3 retires. The surviving home affordances —
the disclosures pointer, services, and the grandfathered v0 boards
section — keep their sections beneath the directory.

## DC-2

Consume the seam, never re-derive. The page renders the computed directory
index the sibling ref-index story owns (workbench-directory stub ref-index,
ac-2's deterministic refs-only computation); the home handler performs no
git ref enumeration of its own and holds no second copy of the grouping
rules. Grouping keys off each index entry's status field — never its
address, never its on-disk path (feature dc-2). What the index computes is
that story's contract; what this page does with it is this story's.

## DC-3

Link grammar consumed, not invented. Default-branch entries keep today's
unprefixed addresses — `/board/spec/<name>` for the board, `/a/spec/<name>`
for the corpus page. A design-branch entry links to its per-branch board
address under the sibling draft-boards story's grammar,
`/b/<branch-escaped>/board/spec/<name>` — one grammar for local and
remote-tracking entries alike, behind which the routing story enforces
feature dc-5's split: only a local design branch opens as an authoring wall
(managed worktrees are cut from local branches only), while a remote-only
branch renders sealed, read-only, with its remoteness disclosed. The
directory never encodes that split as different link shapes and never mints
a third grammar; it emits only addresses the routing story serves, so a
link on this page is live by construction or disclosed per AC-3.

## DC-4

The in-review chip is a per-render, non-blocking consultation of the forge
port's ListOpenMRs — the second, non-ref enumeration source feature dc-5
names, disclosed as such on the page. It never enters the deterministic
index computation (co-1 stays refs-only), and every failure — unreachable
forge, misconfigured credentials, transport error — degrades to the
disclosed "MR status unavailable" absence. The refs-computed directory is
never blocked on the network and never rendered partially because of it.

## DC-5

Error surfaces. An index-computation failure renders as a disclosed inline
notice in a still-served page — the home page is the one landing surface
that must never itself be a dead end, the posture the existing renderHome
already takes for a half-initialised store. A stale entry click (the branch
vanished between render and click) resolves to a rendered disclosed notice
page: HTTP 404 as the honest status, a human-readable body naming what
vanished, and a link back to the directory — never a bare NotFound, never
a blank response.

## CO-1

A directory read mutates nothing. Rendering the page reads the computed
index — refs only — and never switches a checkout, never cuts a managed
worktree, never deletes one (feature co-1; dc-1's no-surprise-mutation law;
dc-4's reads-never-delete). Worktree creation belongs to the open-a-board
path (the draft-boards and worktree-manager stories), never to listing.

## CO-2

No network in any test. The in-review chip and its degradation are proven
against hermetic doubles — the httptest forge double and canned fixtures —
and the Playwright suite under `e2e/` is the behavioral evidence register
for every directory surface, driving the real served page over fixture
stores with deterministic refs.

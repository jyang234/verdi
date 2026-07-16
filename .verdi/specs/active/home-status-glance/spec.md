---
id: spec/home-status-glance
kind: spec
title: "Home Status Glance"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-27
problem: { text: "today's home directory (spec/directory-home) already groups every default-branch spec and design-branch draft into four status buckets — drafts in progress, accepted-pending-build, active components, terminal — but gives all four equal visual weight and equal position: an operator lands on GET / and must scan the whole exhaustive listing (plus the artifacts-by-kind, services, and boards sections beneath it) to find the handful of specs that actually need THEIR attention right now. Nothing on the page leads with what is actionable, and the one bucket that most needs a second look — terminal — even commingles a spec that is done and physically archived with one that is merely closed and still sitting in the active zone awaiting the archive move: two different next-actions, rendered identically (feature problem; feature dc-4's own 'closed awaiting archive' distinction)", anchor: "#problem" }
outcome: { text: "GET / leads with a new status-at-a-glance section, above the existing exhaustive Directory section (which keeps rendering exactly as it does today, in place, unchanged). Every active spec — default-branch and design-branch alike — regroups into three actionable-first buckets in fixed order: draft ('on the desk'), accepted-pending-build ('in flight'), then every other active status trailing as a settling group. Each entry carries only its status badge and its working links — a board link universally, matrix and verdict additionally for a feature — with no evidence-bearing state (feature dc-4). The glance is purely additive: every section and link the directory renders today is still there, byte-for-byte, further down the page", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "GET / renders a new leading section, above the existing Directory section, that re-groups every entry from the SAME computed directory index (internal/refindex, consumed once per render, never re-derived) into three buckets in fixed order — on-the-desk (status draft), in-flight (status accepted-pending-build), settling (every other status the index reports, ACTIVE-ZONE ONLY per dc-2) — each shown entry carrying its title (linked exactly as its source already links it today), its raw status badge, and its working links: a board link whenever the routing can actually serve one, plus matrix and verdict additionally for a default-branch class:feature entry; proven over a fixture store spanning every status value this store's schema legalizes AND both store zones — including an active-zone closed-or-superseded entry (which the glance shows in settling) and an archive-zone entry (which the glance EXCLUDES: asserted absent from the glance yet still present, unchanged, in the exhaustive Directory section — dc-2's zone rule, ac-2's no-loss), across both a story/feature entry and a component entry, and across both a default-branch entry and a design-branch draft", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "every section and every link the directory renders today — the four existing status groups with their entries, chips, and links; the store-root notice; the disclosures pointer; the other-artifacts-by-kind listing; the services listing; the boards listing — is still present, in the same place, carrying the same content, after this story lands: the glance section is additive only, never a replacement or a removal, proven against the identical fixture store ac-1 uses", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "a glance bucket with zero matching entries still renders its heading, its zero count, and an explicit empty-state notice, mirroring the existing Directory section's own empty-group precedent — never a silently omitted bucket; all three buckets are structurally present on every render of GET /, regardless of which are populated", evidence: [behavioral, attestation], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/workbench-legibility#ac-3" }
decisions:
  - { id: dc-1, text: "the glance is a second, additive rendering pass over the exact same home.Index(ctx) call renderHome already makes once per render (directory.go) — no second index computation, no new persisted artifact, no new frontmatter field (parent dc-1 upheld). Its population is every non-disclosed entry the index returns that dc-2's zone rule admits: every ACTIVE-ZONE default-branch spec entry plus every ordinary (non-degraded) design-branch draft entry (itself active-zone). An archive-zone entry, though the index returns it, is held to the exhaustive Directory section by dc-2 — the same no-loss posture as the Disclosed case below. A design branch with no draft spec yet (refindex's Disclosed case) carries no content to badge or link and is excluded from the glance — it remains fully rendered, unchanged, in the exhaustive Directory section below (co-2, ac-2's no-loss bar)", anchor: "#dc-1" }
  - { id: dc-2, text: "the three buckets re-group refindex's existing four-value StatusGroup vocabulary (parent dc-1: the grouping vocabulary is consumed, never re-derived): on-the-desk = StatusGroupDraftsInProgress, in-flight = StatusGroupAcceptedPendingBuild, settling = ACTIVE-ZONE entries whose group is StatusGroupActiveComponents or StatusGroupTerminal — a still-active component, or a closed-awaiting-archive / superseded spec that is STILL sitting in .verdi/specs/active/. ZONE-AWARE settling per ADJ-32 (judge finding f1 sustained): an archive-zone entry — one already moved to .verdi/specs/archive/ — is EXCLUDED from the glance entirely. Zone-agnostic settling would lead the home page with every archived spec ever: refindex walks both zones into StatusGroupTerminal, and this store's own e2e fixtures render TERMINAL_SPEC and ARCHIVED_SPEC side by side in that group (e2e/tests/37-directory-home.spec.ts), a graveyard atop a section whose whole purpose is actionable-first, contradicting parent dc-4's explicit 'active specs' population — the glance would get worse every time a spec closes. EXCLUDED means from the GLANCE ONLY: every archive-zone entry still renders unchanged in the exhaustive Directory section ac-2 pins (no loss). Mechanism: a computed, in-memory zone distinction derived from WHERE the index read the entry — the active/archive zone is already known at the point refindex reads each default-branch entry (computeDefaultBranchEntries walks both zones in one loop), surfaced as an additive field on refindex.Entry or an equivalent computed signal, never a persisted artifact or frontmatter field (parent dc-1 upheld) and never a gate/fold/lint/CLI change (refindex is none of these; parent dc-3 upheld). REGRESSION OBLIGATION on that additive seam: the new signal is read only by the glance; every other refindex consumer — the exhaustive directory render (internal/workbench/directory.go) and refindex's own tests — must behave byte-identically after it lands, proven, not assumed. Cites ADJ-32", anchor: "#dc-2" }
  - { id: dc-3, text: "a glance entry is deliberately leaner than its exhaustive-section counterpart: title, status badge, and working links only — no source chip (local/remote/both/default), no in-review chip, no receipts or gate state (parent dc-4's evidence-bearing-state bar). Link derivation mirrors directory.go's existing per-source rules exactly, never a third grammar (parent dc-1/dc-3): a default-branch entry's title links to its corpus page and its board link is the unprefixed /board/spec/<name>, present only when the active-zone working tree actually carries the file (today's boardServable gate, mirrored not re-derived. Two distinct truth sources are in play: the glance's zone signal is read from the DEFAULT-BRANCH tree — refindex.computeDefaultBranchEntries, never the working tree (co-1) — while boardServable is a SERVING-WORKING-TREE check (directory.go's specWorkingTreeMeta, an os.ReadFile of .verdi/specs/active/<name>/spec.md). dc-2's archive-zone exclusion moots the ORIGINAL archive-zone withheld-link tension (ADJ-32 f2). In the residual case where those two sources diverge — a glance-admitted active-zone entry whose file is absent from the serving checkout (the checkout is behind origin's default branch, or served from a worktree that dropped the file) — the board link is honestly withheld, exactly as the exhaustive section already degrades (a working-tree-absent spec gets no board link there either): parent dc-4's WORKING-links qualifier governs and co-2's honest degradation applies — a link that cannot work does not exist to give (the same reading that settled f3), never a broken link, and no supersedes/exempts edge (ADJ-26). This is Controller adjudication ADJ-35, which narrows ADJ-32's f2-mootness premise on the record); a design-branch entry's title IS its one link to the per-branch board address, exactly as writeDesignEntry renders it today. Matrix and verdict links appear only for a default-branch class:feature entry with a non-empty story field (today's exact condition) — a still-drafting feature on a design branch carries no built evidence for matrix/verdict to show, so it gets a board link only, mirroring writeDesignEntry's current behavior: not a link withheld, but a link that does not yet exist to give (parent dc-4 promises WORKING links; a link that cannot work does not exist to give — the judge's f3 pressure to synthesize matrix/verdict for a still-drafting feature is rejected per ADJ-32, this reasoning upheld)", anchor: "#dc-3" }
  - { id: dc-4, text: "an empty bucket always renders — heading, zero count, explicit empty-state text — the glance's three buckets are structurally fixed, never conditionally omitted. This mirrors the existing Directory section's own empty-group rendering (directory.go's None. shape) rather than introducing a second convention for 'nothing here'", anchor: "#dc-4" }
  - { id: dc-5, text: "fixed placement and a binding selector contract, mirroring dirEntryTestId/dirGroupTestId's own precedent (e2e/tests/fixtures.ts): the glance section (data-testid home-glance) renders immediately after the store-root/disclosures lines and immediately BEFORE the existing Directory section, which keeps its own markup, classes, and data-testids completely unchanged. Its three bucket sub-sections carry data-testid glance-group-<slug> for slug in the fixed order on-the-desk, in-flight, settling; each shown entry carries data-testid glance-entry-<name>. These are new, additional testids — they never replace or repurpose dir-group-*/dir-entry-*, which ac-2 requires unchanged", anchor: "#dc-5" }
constraints:
  - { id: co-1, text: "every behavioral path is Playwright-proven under e2e/ (parent co-1): the grouping/order/badges/links claim, the no-loss claim, and the empty-bucket claim are each driven against a fixture store, never live data; no network in any test", anchor: "#co-1" }
  - { id: co-2, text: "honest degradation, inherited from the parent (co-2) and from directory-home's own dc-5: an index-computation failure disclosed inline by the existing renderHome/writeDirectorySection (the SAME indexErr this story's glance section also receives from the one shared home.Index(ctx) call) degrades both the glance and the exhaustive section identically — the glance never renders a second, contradictory notice, and never renders partial or fabricated groups when the index itself failed", anchor: "#co-2" }
frozen: { at: 2026-07-16, commit: 28024ab6c2631d07f449df8178b6c26b70c14787, stub_matched: true }
---
# Home Status Glance

## Problem

Today's home directory (spec/directory-home) already groups every
default-branch spec and design-branch draft into four status buckets —
drafts in progress, accepted-pending-build, active components, terminal —
but gives all four equal visual weight and equal position. An operator
lands on `GET /` and must scan the whole exhaustive listing, plus the
artifacts-by-kind, services, and boards sections beneath it, to find the
handful of specs that actually need THEIR attention right now. Nothing on
the page leads with what is actionable, and the one bucket that most needs
a second look — terminal — even commingles a spec that is done and
physically archived with one that is merely closed and still sitting in
the active zone awaiting the archive move: two different next-actions,
rendered identically (feature problem; feature dc-4's own "closed awaiting
archive" distinction).

## Outcome

`GET /` leads with a new status-at-a-glance section, above the existing
exhaustive Directory section — which keeps rendering exactly as it does
today, in place, unchanged. Every active spec, default-branch and
design-branch alike, regroups into three actionable-first buckets in fixed
order: `draft` ("on the desk"), `accepted-pending-build` ("in flight"),
then every other active status trailing as a `settling` group. Each entry
carries only its status badge and its working links — a board link
universally, matrix and verdict additionally for a feature — with no
evidence-bearing state (feature dc-4). The glance is purely additive:
every section and link the directory renders today is still there,
byte-for-byte, further down the page.

## AC-1

`GET /` renders a new leading section, above the existing Directory
section, that re-groups every entry from the SAME computed directory index
(internal/refindex, consumed once per render, never re-derived) into three
buckets, in this fixed order: `on-the-desk` (status draft), `in-flight`
(status accepted-pending-build), `settling` (active-zone entries only, per
dc-2: a still-active component, or a closed-awaiting-archive / superseded
spec STILL sitting in `.verdi/specs/active/` — an archive-zone entry is
excluded). A disclosed no-draft-spec branch
entry (refindex's degraded case) carries no spec content and is left to
the exhaustive section below — the glance shows specs, not empty
branches (dc-1). Every entry the glance does show carries: its title,
linked exactly as its source already links it today (the unprefixed
corpus/board pair for a default-branch entry, the per-branch board grammar
for a design-branch entry); its raw status badge; and its working links —
a board link whenever the routing can actually serve one (mirroring
today's `boardServable`/design-branch rules exactly — a serving-working-tree
check; dc-2's archive-zone exclusion moots the original archive-zone
withheld-link case, and where a glance-admitted active-zone entry's file is
absent from the serving checkout the board link is honestly withheld,
exactly as the exhaustive section degrades — dc-3, per ADJ-35),
plus matrix and verdict additionally for a default-branch `class: feature`
entry (dc-3). Proven over a fixture store spanning every status value
this store's schema legalizes (draft, accepted-pending-build, active,
closed, superseded) AND both store zones: a `class: story`/`feature` entry
and a `class: component` entry, a default-branch entry and a design-branch
draft, an active-zone closed-or-superseded spec (which the glance shows in
`settling`) and an archive-zone spec — the archive-zone spec asserted
ABSENT from the glance while still present, unchanged, in the exhaustive
Directory section (dc-2's zone rule; ac-2's no-loss). Evidence: behavioral (Playwright drives `GET /` over
the fixture store and asserts bucket membership, order, badges, and link
targets) + attestation.

## AC-2

Every section and every link the directory renders today — the four
existing status groups (`drafts-in-progress`, `accepted-pending-build`,
`active-components`, `terminal`) with their entries, chips, and links; the
store-root notice; the disclosures pointer; the other-artifacts-by-kind
listing; the services listing; the boards listing — is still present, in
the same place, carrying the same content, after this story lands. The
glance section is additive only, never a replacement or a removal (dc-1).
Proven against the identical fixture store AC-1 uses: every pre-existing
`data-testid` this story does not itself introduce is asserted present,
visible, and carrying the content it carries today. Evidence: behavioral
(Playwright re-asserts every pre-existing surface unchanged on the same
render that also proves AC-1) + attestation.

## AC-3

A glance bucket with zero matching entries still renders its heading, its
zero count, and an explicit empty-state notice, exactly mirroring the
existing Directory section's own "None." precedent for an empty group
(internal/workbench/directory.go) — never a silently omitted bucket (dc-4).
All three buckets are structurally present on every render of `GET /`,
regardless of which are populated, so an operator reads absence-of-work as
an explicit, deliberate fact rather than wondering whether the section is
broken. Evidence: behavioral (Playwright drives a fixture store with at
least one empty bucket and asserts the heading, count, and empty-state
notice all render) + attestation.

## DC-1

The glance is a second, additive rendering pass over the exact same
`home.Index(ctx)` call `renderHome` already makes once per render
(directory.go) — no second index computation, no new persisted artifact,
no new frontmatter field (parent dc-1 upheld). Its population is every
non-disclosed entry the index returns that dc-2's zone rule admits: every
active-zone default-branch spec entry plus every ordinary (non-degraded)
design-branch draft entry (itself active-zone). An archive-zone entry,
though the index returns it, is held to the exhaustive Directory section by
dc-2 — the same no-loss posture as the `Disclosed` case below. A design
branch with no draft spec yet (refindex's `Disclosed` case) carries
no content to badge or link and is excluded from the glance — it remains
fully rendered, unchanged, in the exhaustive Directory section below
(co-2, AC-2's no-loss bar).

## DC-2

The three buckets re-group refindex's existing four-value `StatusGroup`
vocabulary (parent dc-1: the grouping vocabulary is consumed, never
re-derived): `on-the-desk` = `StatusGroupDraftsInProgress`, `in-flight` =
`StatusGroupAcceptedPendingBuild`, `settling` = the **active-zone** entries
whose group is `StatusGroupActiveComponents` or `StatusGroupTerminal` — a
still-active component, or a closed-awaiting-archive / superseded spec that
is STILL sitting in `.verdi/specs/active/`.

**Zone-aware settling (ADJ-32, judge finding f1 sustained).** An
archive-zone entry — one already moved to `.verdi/specs/archive/` — is
EXCLUDED from the glance entirely. Zone-agnostic settling would lead the
home page with every archived spec ever: refindex walks both zones into
`StatusGroupTerminal`, and this store's own e2e fixtures already render
TERMINAL_SPEC and ARCHIVED_SPEC side by side in that group
(`e2e/tests/37-directory-home.spec.ts`). That is a graveyard atop a
section whose whole purpose is actionable-first, and it contradicts parent
DC-4's explicit "active specs" population — the glance would get worse
every time a spec closes. Excluding archive-zone entries here means from
the GLANCE ONLY: every one of them still renders unchanged in the
exhaustive Directory section AC-2 pins, so nothing is lost.

**Mechanism.** A computed, in-memory zone distinction derived from WHERE
the index read the entry. The active/archive zone is already known at the
point `refindex` reads each default-branch entry
(`computeDefaultBranchEntries` walks `.verdi/specs/active/` and
`.verdi/specs/archive/` in one loop), so the distinction is surfaced as an
additive field on `refindex.Entry` (or an equivalent computed signal) — an
in-memory derivation, never a persisted artifact or frontmatter field
(parent dc-1 upheld: no new persisted state), and never a gate, fold,
lint, or CLI change (refindex is none of those; parent dc-3 upheld,
presentation only).

**Regression obligation on the additive seam.** The new zone signal is
read only by the glance. Every other `refindex` consumer — the exhaustive
directory render (`internal/workbench/directory.go`) and refindex's own
tests — MUST behave byte-identically after the field lands; the build
proves this, it is not assumed. (ADJ-32 flagged exactly this shared-seam
care as the one cost of sustaining f1.)

## DC-3

A glance entry is deliberately leaner than its exhaustive-section
counterpart: title, status badge, and working links only — no source chip
(local/remote/both/default), no in-review chip, no receipts or gate state
(parent DC-4's evidence-bearing-state bar). Link derivation mirrors
`directory.go`'s existing per-source rules exactly, never a third grammar
(parent DC-1/DC-3): a default-branch entry's title links to its corpus
page and its board link is the unprefixed `/board/spec/<name>`, present
only when the active-zone working tree actually carries the file — the
`boardServable` gate, mirrored not re-derived. Two distinct truth sources
are in play: the glance's zone signal is read from the **default-branch
tree** (`refindex.computeDefaultBranchEntries`, "never the working tree",
co-1), while `boardServable` is a **serving-working-tree** check
(`directory.go`'s `specWorkingTreeMeta`, an `os.ReadFile` of
`.verdi/specs/active/<name>/spec.md`). DC-2's archive-zone exclusion moots
the original archive-zone withheld-link tension (ADJ-32 f2). In the
residual case where those two sources diverge — a glance-admitted
active-zone entry whose file is absent from the serving checkout (the
checkout is behind origin's default branch, or is served from a worktree
that dropped the file) — the board link is honestly withheld, exactly as
the exhaustive section already degrades (a working-tree-absent spec gets no
board link there either). Parent DC-4's **working-links** qualifier governs
and CO-2's honest degradation applies: a link that cannot work does not
exist to give (the same reading that settled f3), never a broken link, and
no supersedes/exempts edge (ADJ-26). This is Controller adjudication
**ADJ-35**, which narrows ADJ-32's f2-mootness premise on the record. A
design-branch entry's title IS its one link to the per-branch board
address, exactly as `writeDesignEntry` renders it today. Matrix and verdict links appear only for a
default-branch `class: feature` entry with a non-empty `story` field
(today's exact condition) — a still-drafting feature on a design branch
carries no built evidence for matrix/verdict to show, so it gets a board
link only, mirroring `writeDesignEntry`'s current behavior: not a link
withheld, but a link that does not yet exist to give (parent DC-4 promises
WORKING links; a link that cannot work does not exist to give — the judge's
f3 pressure to synthesize matrix/verdict for a still-drafting feature is
rejected per ADJ-32, this reasoning upheld).

## DC-4

An empty bucket always renders — heading, zero count, explicit
empty-state text — the glance's three buckets are structurally fixed,
never conditionally omitted. This mirrors the existing Directory section's
own empty-group rendering (`directory.go`'s "None." shape) rather than
introducing a second convention for "nothing here."

## DC-5

Fixed placement and a binding selector contract, mirroring
`dirEntryTestId`/`dirGroupTestId`'s own precedent (`e2e/tests/fixtures.ts`):
the glance section (`data-testid="home-glance"`) renders immediately after
the store-root/disclosures lines and immediately BEFORE the existing
Directory section, which keeps its own markup, classes, and data-testids
completely unchanged. Its three bucket sub-sections carry
`data-testid="glance-group-<slug>"` for `<slug>` in the fixed order
`on-the-desk`, `in-flight`, `settling`; each shown entry carries
`data-testid="glance-entry-<name>"`. These are new, additional testids —
they never replace or repurpose `dir-group-*`/`dir-entry-*`, which AC-2
requires unchanged.

## CO-1

Every behavioral path is Playwright-proven under `e2e/` (parent CO-1): the
grouping/order/badges/links claim, the no-loss claim, and the empty-bucket
claim are each driven against a fixture store, never live data; no network
in any test.

## CO-2

Honest degradation, inherited from the parent (CO-2) and from
directory-home's own DC-5: an index-computation failure disclosed inline
by the existing `renderHome`/`writeDirectorySection` (the SAME `indexErr`
this story's glance section also receives from the one shared
`home.Index(ctx)` call) degrades both the glance and the exhaustive
section identically — the glance never renders a second, contradictory
notice, and never renders partial or fabricated groups when the index
itself failed.

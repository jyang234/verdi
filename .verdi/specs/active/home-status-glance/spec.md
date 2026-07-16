---
id: spec/home-status-glance
kind: spec
title: "Home Status Glance"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-27
problem: { text: "today's home directory (spec/directory-home) already groups every default-branch spec and design-branch draft into four status buckets — drafts in progress, accepted-pending-build, active components, terminal — but gives all four equal visual weight and equal position: an operator lands on GET / and must scan the whole exhaustive listing (plus the artifacts-by-kind, services, and boards sections beneath it) to find the handful of specs that actually need THEIR attention right now. Nothing on the page leads with what is actionable, and the one bucket that most needs a second look — terminal — even commingles a spec that is done and physically archived with one that is merely closed and still sitting in the active zone awaiting the archive move: two different next-actions, rendered identically (feature problem; feature dc-4's own 'closed awaiting archive' distinction)", anchor: "#problem" }
outcome: { text: "GET / leads with a new status-at-a-glance section, above the existing exhaustive Directory section (which keeps rendering exactly as it does today, in place, unchanged). Every active spec — default-branch and design-branch alike — regroups into three actionable-first buckets in fixed order: draft ('on the desk'), accepted-pending-build ('in flight'), then every other active status trailing as a settling group. Each entry carries only its status badge and its working links — a board link universally, matrix and verdict additionally for a feature — with no evidence-bearing state (feature dc-4). The glance is purely additive: every section and link the directory renders today is still there, byte-for-byte, further down the page", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "GET / renders a new leading section, above the existing Directory section, that re-groups every entry from the SAME computed directory index (internal/refindex, consumed once per render, never re-derived) into three buckets in fixed order — on-the-desk (status draft), in-flight (status accepted-pending-build), settling (every other status the index reports) — each shown entry carrying its title (linked exactly as its source already links it today), its raw status badge, and its working links: a board link whenever the routing can actually serve one, plus matrix and verdict additionally for a default-branch class:feature entry; proven over a fixture store spanning every status value this store's schema legalizes, across both a story/feature entry and a component entry, and across both a default-branch entry and a design-branch draft", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "every section and every link the directory renders today — the four existing status groups with their entries, chips, and links; the store-root notice; the disclosures pointer; the other-artifacts-by-kind listing; the services listing; the boards listing — is still present, in the same place, carrying the same content, after this story lands: the glance section is additive only, never a replacement or a removal, proven against the identical fixture store ac-1 uses", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "a glance bucket with zero matching entries still renders its heading, its zero count, and an explicit empty-state notice, mirroring the existing Directory section's own empty-group precedent — never a silently omitted bucket; all three buckets are structurally present on every render of GET /, regardless of which are populated", evidence: [behavioral, attestation], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/workbench-legibility#ac-3" }
decisions:
  - { id: dc-1, text: "the glance is a second, additive rendering pass over the exact same home.Index(ctx) call renderHome already makes once per render (directory.go) — no second index computation, no new persisted artifact, no new frontmatter field (parent dc-1 upheld). Its population is every non-disclosed entry the index already returns: every default-branch spec entry plus every ordinary (non-degraded) design-branch draft entry. A design branch with no draft spec yet (refindex's Disclosed case) carries no content to badge or link and is excluded from the glance — it remains fully rendered, unchanged, in the exhaustive Directory section below (co-2, ac-2's no-loss bar)", anchor: "#dc-1" }
  - { id: dc-2, text: "the three buckets are an exact re-grouping of refindex's existing four-value StatusGroup vocabulary (parent dc-1: the grouping vocabulary is consumed, never re-derived): on-the-desk = StatusGroupDraftsInProgress, in-flight = StatusGroupAcceptedPendingBuild, settling = StatusGroupActiveComponents union StatusGroupTerminal. DISCLOSED JUDGMENT CALL: refindex.Entry carries no field distinguishing an active-zone entry (a closed/superseded spec still sitting in .verdi/specs/active/, 'awaiting archive') from an archive-zone one already moved — both map to StatusGroupTerminal identically today, and this store's own e2e fixtures already render such a pair side by side in the SAME terminal group (e2e/tests/37-directory-home.spec.ts's TERMINAL_SPEC/ARCHIVED_SPEC). Rather than invent a new zone-tracking field on a shared computation seam to split them — a change this story's dc-3 (presentation only) and its own narrow scope do not clearly license — settling stays zone-agnostic and reuses the existing grouping exactly: an archived spec surfacing in settling alongside a genuinely-awaiting-archive one is the smallest reversible reading of parent dc-4's 'any remaining active statuses' language; it costs nothing against ac-2 (no-loss holds either way) and can be tightened later, under its own decision, if zone precision is wanted. Flagged for review", anchor: "#dc-2" }
  - { id: dc-3, text: "a glance entry is deliberately leaner than its exhaustive-section counterpart: title, status badge, and working links only — no source chip (local/remote/both/default), no in-review chip, no receipts or gate state (parent dc-4's evidence-bearing-state bar). Link derivation mirrors directory.go's existing per-source rules exactly, never a third grammar (parent dc-1/dc-3): a default-branch entry's title links to its corpus page and its board link is the unprefixed /board/spec/<name>, present only when the active-zone working tree actually carries the file (today's boardServable gate — an archive-zone entry surfacing in settling per dc-2 renders with no board link, honest degradation, exactly as it does in today's exhaustive terminal group); a design-branch entry's title IS its one link to the per-branch board address, exactly as writeDesignEntry renders it today. Matrix and verdict links appear only for a default-branch class:feature entry with a non-empty story field (today's exact condition) — a still-drafting feature on a design branch carries no built evidence for matrix/verdict to show, so it gets a board link only, mirroring writeDesignEntry's current behavior: not a link withheld, but a link that does not yet exist to give", anchor: "#dc-3" }
  - { id: dc-4, text: "an empty bucket always renders — heading, zero count, explicit empty-state text — the glance's three buckets are structurally fixed, never conditionally omitted. This mirrors the existing Directory section's own empty-group rendering (directory.go's None. shape) rather than introducing a second convention for 'nothing here'", anchor: "#dc-4" }
  - { id: dc-5, text: "fixed placement and a binding selector contract, mirroring dirEntryTestId/dirGroupTestId's own precedent (e2e/tests/fixtures.ts): the glance section (data-testid home-glance) renders immediately after the store-root/disclosures lines and immediately BEFORE the existing Directory section, which keeps its own markup, classes, and data-testids completely unchanged. Its three bucket sub-sections carry data-testid glance-group-<slug> for slug in the fixed order on-the-desk, in-flight, settling; each shown entry carries data-testid glance-entry-<name>. These are new, additional testids — they never replace or repurpose dir-group-*/dir-entry-*, which ac-2 requires unchanged", anchor: "#dc-5" }
constraints:
  - { id: co-1, text: "every behavioral path is Playwright-proven under e2e/ (parent co-1): the grouping/order/badges/links claim, the no-loss claim, and the empty-bucket claim are each driven against a fixture store, never live data; no network in any test", anchor: "#co-1" }
  - { id: co-2, text: "honest degradation, inherited from the parent (co-2) and from directory-home's own dc-5: an index-computation failure disclosed inline by the existing renderHome/writeDirectorySection (the SAME indexErr this story's glance section also receives from the one shared home.Index(ctx) call) degrades both the glance and the exhaustive section identically — the glance never renders a second, contradictory notice, and never renders partial or fabricated groups when the index itself failed", anchor: "#co-2" }
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
(status accepted-pending-build), `settling` (every other status the index
reports: active, closed, superseded). A disclosed no-draft-spec branch
entry (refindex's degraded case) carries no spec content and is left to
the exhaustive section below — the glance shows specs, not empty
branches (dc-1). Every entry the glance does show carries: its title,
linked exactly as its source already links it today (the unprefixed
corpus/board pair for a default-branch entry, the per-branch board grammar
for a design-branch entry); its raw status badge; and its working links —
a board link whenever the routing can actually serve one (mirroring
today's `boardServable`/design-branch rules exactly, including the
honest-degradation case of an archive-zone entry with no live board),
plus matrix and verdict additionally for a default-branch `class: feature`
entry (dc-3). Proven over a fixture store spanning every status value
this store's schema legalizes (draft, accepted-pending-build, active,
closed, superseded) across both a `class: story`/`feature` entry and a
`class: component` entry, and across both a default-branch entry and a
design-branch draft. Evidence: behavioral (Playwright drives `GET /` over
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
non-disclosed entry the index already returns: every default-branch spec
entry plus every ordinary (non-degraded) design-branch draft entry. A
design branch with no draft spec yet (refindex's `Disclosed` case) carries
no content to badge or link and is excluded from the glance — it remains
fully rendered, unchanged, in the exhaustive Directory section below
(co-2, AC-2's no-loss bar).

## DC-2

The three buckets are an exact re-grouping of refindex's existing
four-value `StatusGroup` vocabulary (parent dc-1: the grouping vocabulary
is consumed, never re-derived): `on-the-desk` = `StatusGroupDraftsInProgress`,
`in-flight` = `StatusGroupAcceptedPendingBuild`, `settling` =
`StatusGroupActiveComponents` union `StatusGroupTerminal`.

**Disclosed judgment call.** `refindex.Entry` carries no field
distinguishing an active-zone entry (a closed/superseded spec still
sitting in `.verdi/specs/active/`, "awaiting archive") from an
archive-zone one already moved — both map to `StatusGroupTerminal`
identically today, and this store's own e2e fixtures already render such a
pair side by side in the SAME terminal group
(`e2e/tests/37-directory-home.spec.ts`'s TERMINAL_SPEC/ARCHIVED_SPEC).
Rather than invent a new zone-tracking field on a shared computation seam
to split them — a change this story's DC-3 (presentation only) and its own
narrow scope do not clearly license — `settling` stays zone-agnostic and
reuses the existing grouping exactly: an archived spec surfacing in
`settling` alongside a genuinely-awaiting-archive one is the smallest
reversible reading of parent DC-4's "any remaining active statuses"
language. It costs nothing against AC-2 (no-loss holds either way) and can
be tightened later, under its own decision, if zone precision is wanted.
Flagged for review.

## DC-3

A glance entry is deliberately leaner than its exhaustive-section
counterpart: title, status badge, and working links only — no source chip
(local/remote/both/default), no in-review chip, no receipts or gate state
(parent DC-4's evidence-bearing-state bar). Link derivation mirrors
`directory.go`'s existing per-source rules exactly, never a third grammar
(parent DC-1/DC-3): a default-branch entry's title links to its corpus
page and its board link is the unprefixed `/board/spec/<name>`, present
only when the active-zone working tree actually carries the file (today's
`boardServable` gate — an archive-zone entry surfacing in `settling` per
DC-2 renders with no board link, honest degradation, exactly as it does in
today's exhaustive terminal group); a design-branch entry's title IS its
one link to the per-branch board address, exactly as `writeDesignEntry`
renders it today. Matrix and verdict links appear only for a
default-branch `class: feature` entry with a non-empty `story` field
(today's exact condition) — a still-drafting feature on a design branch
carries no built evidence for matrix/verdict to show, so it gets a board
link only, mirroring `writeDesignEntry`'s current behavior: not a link
withheld, but a link that does not yet exist to give.

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

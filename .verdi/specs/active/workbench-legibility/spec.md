---
id: spec/workbench-legibility
kind: spec
title: "Workbench Legibility"
owners: [platform-team]
class: feature
status: draft
problem: { text: "real usage surfaced three navigation-and-state legibility gaps in the workbench. First, a board tool view (the diagram designer) has no exit affordance — the only escape anywhere in the workbench is the top-left wordmark link to the index, and nothing inside a tool view marks how to leave it. Second, the family structure the store already knows is invisible at the board surface: an instantiated story carries `implements` edges to its feature's AC fragments, yet a story board renders no way to jump to its parent feature board, and a feature board's stub cards do not link to the story boards they gave rise to — the operator navigates by URL surgery or via the index. Third, the home directory (workbench-directory's real, functional index) renders every section at equal weight, so nothing communicates state — 'the Home Screen feels a little plain with just a listing of files' — though status, gate-bearing links, and family structure are all already in the store.", anchor: "#problem" }
outcome: { text: "the workbench is navigable and state-legible from the surface the operator is already on. Every board tool view has an explicit exit affordance returning to the board. Family navigation exists in both directions, derived entirely from existing typed edges — a story board links to its parent feature board, a feature board's stub cards link to their instantiated story boards — with unresolvable links disclosed, never broken. And the home page leads with status-at-a-glance: active specs grouped by lifecycle state with their working links foregrounded, exhaustive listings intact below. No new persisted artifact, field, or model change — this feature is pure projection over what the store already knows.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "every board tool view (the diagram designer included) has an explicit, visible exit affordance that returns to the board it was entered from, and the Escape key does the same; entering and leaving a tool view is proven end-to-end in the browser", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "family navigation is rendered in both directions from existing `implements` edges alone: a story board renders a parent-feature affordance resolving to the feature's board, and a feature board's stub card links to the instantiated story's board when a matching active story spec exists — with the in-between state (instantiated on a design branch, not yet in this checkout's active store) disclosed on the stub card rather than rendered as a dead or missing link", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "the home page leads with status-at-a-glance: active specs grouped by lifecycle status, each entry carrying its status badge and working links (board; matrix/verdict for features), so the state of the store is readable from the home page without opening any spec — the existing exhaustive sections (archived specs, artifacts by kind, services, boards) remain available below and no information the directory renders today is lost", evidence: [behavioral, attestation], anchor: "#ac-3" }
decisions:
  - { id: dc-1, text: "navigation and grouping derive ENTIRELY from existing store state and typed edges — no new persisted artifact, no new frontmatter field, no authored navigation config. The `implements` edge is the family join in both directions (a feature's stub↔story match is the computed inverse, exactly as the feature fold already computes it); lifecycle status comes from the spec's own `status` field. This feature is pure projection: if a link cannot be derived, it is disclosed, never authored around", anchor: "#dc-1" }
  - { id: dc-2, text: "the legibility standard is 03 §rung-3's property carried to the workbench: legible WITHOUT consulting backlinks — here, without URL surgery, without the index detour, and without reading the store on disk. Each surface must answer the operator's local question in place: a tool view answers 'how do I get out', a board answers 'where is my family', the home page answers 'what state is everything in'. This is the acceptance bar the e2e proofs exercise", anchor: "#dc-2" }
  - { id: dc-3, text: "presentation only: no gate, fold, lint, or CLI behavior changes under this feature. The workbench remains a projection of the store (05 §Workbench posture); a legibility feature that needed the model to change would be mis-scoped and must be split out", anchor: "#dc-3" }
  - { id: dc-4, text: "the home page's leading taxonomy is actionable-first and status-only: active specs group as `draft` ('on the desk' — awaiting design or acceptance action) first, then `accepted-pending-build` ('in flight' — the build queue), then any remaining active statuses (closed awaiting archive, superseded) as a trailing 'settling' group. Each entry carries its status badge and working links — board for all specs; matrix and verdict additionally for features — and NO evidence-bearing state: wall-receipts chips and gate summaries remain matrix's job, keeping the home render pure projection with no authoritative folds. Settles oq-1 (ADJ-24)", anchor: "#dc-4" }
  - { id: dc-5, text: "the stub card's in-between disclosure is derived live from git refs and never persisted: stub-instantiate deterministically creates `design/<stub-slug>` and fails closed on collision, so the ref IS the record. A stub card with no matching active story checks `refs/heads/design/<stub-slug>` at render time — present renders 'instantiated on design/<slug>, not yet in this checkout's active store' with the branch name shown; absent renders the plain un-instantiated state. No new persisted artifact or frontmatter field (dc-1 upheld); the disclosure cannot go stale because the ref is read at render. Settles oq-2 (ADJ-25)", anchor: "#dc-5" }
constraints:
  - { id: co-1, text: "every behavioral path is Playwright-proven under e2e/ (browser-facing rule): tool-view enter/exit including Escape, both family-link directions including the disclosed in-between state, and the home grouping with its links. No network in any test; fixture stores only", anchor: "#co-1" }
  - { id: co-2, text: "honest degradation everywhere: an unresolvable edge target, an unreadable store section, or a stub with no matching story renders a disclosed inline notice — never a broken link, a 404 affordance, or a silent omission. This is the index page's existing defensive posture, made a requirement for every surface this feature touches", anchor: "#co-2" }
  - { id: co-3, text: "the operative property: from any workbench surface the operator can answer 'where am I, how do I leave, and what state is my work in' without URL surgery, without the index detour, and without reading the store on disk. The feature satisfies this on all three touched surfaces or it is not done", anchor: "#co-3" }
stubs:
  - { slug: tool-view-exit, acceptance_criteria: [ac-1] }
  - { slug: family-board-links, acceptance_criteria: [ac-2] }
  - { slug: home-status-glance, acceptance_criteria: [ac-3] }
---
# Workbench Legibility

## Problem

Real usage surfaced three navigation-and-state legibility gaps in the
workbench.

**No exit from tool views.** A board tool view — the diagram designer — has no
exit affordance. The only escape anywhere in the workbench is the top-left
wordmark link back to the index; nothing inside a tool view marks how to leave
it, and the operator who entered one is stuck or forced through the index.

**Invisible family structure.** The store already knows the family: an
instantiated story carries `implements` edges to its feature's AC fragments,
and the feature fold computes the inverse. Yet a story board renders no way to
jump to its parent feature board, and a feature board's stub cards do not link
to the story boards they gave rise to. The operator navigates between related
boards by URL surgery or via the index — the one join the model guarantees is
the one the surface doesn't show.

**A flat home page.** The home directory (workbench-directory's index) is real
and functional — specs with badges, board/matrix/verdict links, artifacts,
services, boards — but renders every section at equal weight. Nothing
communicates *state*: "the Home Screen feels a little plain with just a
listing of files," though status and family structure are already in the
store, one render away.

## Outcome

The workbench is navigable and state-legible from the surface the operator is
already on. Every board tool view has an explicit exit returning to its board.
Family navigation exists in both directions, derived entirely from existing
typed edges, with unresolvable links disclosed rather than broken. The home
page leads with status-at-a-glance — active specs grouped by lifecycle state,
working links foregrounded — with today's exhaustive listings intact below.

No new persisted artifact, field, or model change: this feature is pure
projection over what the store already knows.

## AC-1

Every board tool view — the diagram designer included — has an explicit,
visible exit affordance that returns to the board it was entered from, and the
Escape key does the same. Entering and leaving a tool view is proven
end-to-end in the browser. Evidence: behavioral (Playwright drives
enter → exit via both the affordance and Escape, asserting the board is
restored) + attestation.

## AC-2

Family navigation renders in both directions from existing `implements` edges
alone. A story board renders a parent-feature affordance resolving to the
feature's **board** (not only the corpus page). A feature board's stub card
links to the instantiated story's board when a matching active story spec
exists — matched by the same computed inverse the feature fold already uses.
The in-between state (instantiated on a design branch, not yet in this
checkout's active store) is **disclosed** on the stub card (dc-5 settles the
disclosure's shape), never rendered as a dead or missing link. Evidence:
behavioral (Playwright proves both directions and the disclosed in-between
state on fixture stores) + attestation.

## AC-3

The home page leads with status-at-a-glance: active specs grouped by lifecycle
status, each entry carrying its status badge and working links (board for all;
matrix and verdict for features), so the state of the store is readable from
the home page without opening any spec. The existing exhaustive sections —
archived specs, artifacts by kind, services, boards — remain available below,
and no information the directory renders today is lost (this feature
re-weights the page; it never removes). The grouping taxonomy is settled by
dc-4. Evidence: behavioral (Playwright asserts grouping, badges, and links
against a fixture store spanning the statuses) + attestation.

## DC-1

Navigation and grouping derive **entirely** from existing store state and
typed edges — no new persisted artifact, no new frontmatter field, no authored
navigation config. The `implements` edge is the family join in both
directions; the stub↔story match is the computed inverse, exactly as the
feature fold already computes it; lifecycle status comes from the spec's own
`status` field. This feature is pure projection: if a link cannot be derived,
it is disclosed, never authored around.

## DC-2

The legibility standard is 03 §rung-3's property — legible **without
consulting backlinks** — carried to the workbench: without URL surgery,
without the index detour, without reading the store on disk. Each surface must
answer the operator's local question in place: a tool view answers "how do I
get out," a board answers "where is my family," the home page answers "what
state is everything in." This is the acceptance bar the e2e proofs exercise.

## DC-3

Presentation only: no gate, fold, lint, or CLI behavior changes under this
feature. The workbench remains a projection of the store (05 §Workbench). A
legibility change that needed the model to move would be mis-scoped here and
must be split into its own spec.

## DC-4

The home page's leading taxonomy is actionable-first and status-only: active
specs group as `draft` ("on the desk" — awaiting design or acceptance action)
first, then `accepted-pending-build` ("in flight" — the build queue), then any
remaining active statuses (closed awaiting archive, superseded) as a trailing
"settling" group. Each entry carries its status badge and working links —
board for all specs; matrix and verdict additionally for features — and
**no evidence-bearing state**: wall-receipts chips and gate summaries remain
matrix's job, keeping the home render pure projection with no authoritative
folds. Settles oq-1 (ADJ-24).

## DC-5

The stub card's in-between disclosure is derived live from git refs and never
persisted: stub-instantiate deterministically creates `design/<stub-slug>` and
fails closed on collision, so **the ref is the record**. A stub card with no
matching active story checks `refs/heads/design/<stub-slug>` at render time —
present renders "instantiated on design/<slug>, not yet in this checkout's
active store" with the branch name shown; absent renders the plain
un-instantiated state. No new persisted artifact or frontmatter field (dc-1
upheld); the disclosure cannot go stale because the ref is read at render.
Settles oq-2 (ADJ-25).

## CO-1

Every behavioral path is Playwright-proven under `e2e/` (the browser-facing
rule): tool-view enter/exit including Escape, both family-link directions
including the disclosed in-between state, and the home grouping with its
links. No network in any test; fixture stores only.

## CO-2

Honest degradation everywhere: an unresolvable edge target, an unreadable
store section, or a stub with no matching story renders a disclosed inline
notice — never a broken link, a 404 affordance, or a silent omission. This is
the index page's existing defensive posture, made a requirement for every
surface this feature touches.

## CO-3

The operative property: from any workbench surface the operator can answer
"where am I, how do I leave, and what state is my work in" without URL
surgery, without the index detour, and without reading the store on disk. The
feature satisfies this on all three touched surfaces or it is not done.

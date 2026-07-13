---
id: spec/feature-supersession-state
kind: spec
title: "Feature Supersession State"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-4
problem: { text: "a superseded spec's terminal state is not legible where an operator looks, and at the feature rung it does not exist at all. Round 5's D-12 gave the STORY rung a real terminal state — `verdi accept` flips a superseded predecessor story's `status` to `superseded` — but that flip has only ever been proven to change the frontmatter field; it has never been shown legible on the surfaces that render specs (`verdi matrix` prints no status line at all; the board renders no superseded badge; dex has latent badge CSS + a `superseded-by` backlink but no test that either renders for a spec). Worse, at the FEATURE rung there is no terminal-state mechanism whatsoever: accepting a feature v2 that carries a whole-spec `supersedes` edge to v1 prints a blast-radius label but never flips v1's status, so a superseded feature is discoverable only by consulting backlinks — the exact thing the rung-3 design set out to avoid (03 §rung 3: `superseded` is legible \"everywhere without consulting backlinks\"). 02 §Kind registry names this gap and carried the feature-predecessor terminal-state question to round 6.", anchor: "#problem" }
outcome: { text: "a superseded spec's terminal state is legible at both rungs on every surface that renders specs. The story-rung flip is proven visible on `verdi matrix`, the board, and dex; and an equivalent feature-rung mechanism — `verdi accept` flips a superseded feature predecessor's `status` to `superseded` in the same ritual, a sanctioned status-only edit — makes a superseded feature's terminal state readable the same way, from the spec's own rendered status, without consulting backlinks.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "`verdi accept` of a feature spec carrying a whole-spec `supersedes` edge to a feature predecessor flips that predecessor's `status` from `accepted-pending-build` to `superseded` in the same acceptance commit — a sanctioned status-only edit (VL-010's existing frozen-file exception, no new rule) that mirrors the rung-3 story flip and leaves the rung-4 cascade/blast-radius machinery for downstream stories untouched", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "a superseded spec's terminal state renders — not deleted, not silent — on every surface that renders specs (`verdi matrix`, the board, dex), at both the story and feature rungs, so finding it never requires reading raw frontmatter or chasing a `superseded-by` backlink", evidence: [behavioral], anchor: "#ac-2" }
links:
  - { type: implements, ref: "spec/true-closure#ac-4" }
decisions:
  - { id: dc-1, text: "feature-predecessor terminal state = a status flip to `superseded`, owner-resolved (oq-1, 2026-07-13), mirroring the rung-3 story flip D-12 shipped: `verdi accept` of a feature v2 carrying a whole-spec `supersedes` edge to a feature predecessor flips the predecessor's `status` to `superseded` in the same ritual. Chosen over a rendered-relation-only reading because only the status flip makes the terminal state legible \"without consulting backlinks\" (03 §rung 3) — the property that makes the two rungs legible by the same means", anchor: "#dc-1" }
  - { id: dc-2, text: "the flip fires only from `accepted-pending-build` (mirroring the story guard exactly, accept.go's existing `supersedePredecessors`). A `closed` feature being superseded (closed -> superseded) is the rarer case and is deferred as out of scope here — a closed feature is already terminally legible (`closed`), and its supersession is surfaced by the computed `superseded-by` backlink; the smallest reversible scope handles the common in-progress-predecessor case and reopens the closed case cleanly later (invention ledger)", anchor: "#dc-2" }
  - { id: dc-3, text: "`verdi matrix` is the weakest surface and this story fixes it at both rungs: today it prints no `status` line at all (story rung) and the feature fold silently DELETES superseded implementing stories (feature rung, featurematrix.go). After this story the story-level matrix prints the spec's `status`, and the feature-level matrix shows a superseded implementing story with a terminal marker instead of dropping it. dex already carries the badge CSS + `superseded-by` backlink (latent, unproven) and the board gains a superseded badge; both are then proven by an exerciser", anchor: "#dc-3" }
  - { id: dc-4, text: "legibility is proven on verdi's real case where one exists and honestly fixtured where it does not. The story rung uses the corpus's real superseded story (`disclosure-seam`, flipped by `disclosure-seam-v2`); verdi's own corpus has NO superseded feature, so the feature-rung accept-flip and rendering are proven hermetically over a fixture feature-v2-supersedes-v1 pair — the same honest scope as runtime-evidence dc-3: the mechanism is proven and a real feature supersession plugs into it unchanged", anchor: "#dc-4" }
constraints:
  - { id: co-1, text: "no network in any test: the accept flip, the surface rendering (matrix, board, dex), and both rungs are exercised hermetically — over the real `disclosure-seam` case for the story rung and a fixturegit feature-v2/v1 pair for the feature rung, the board/dex surfaces under Playwright e2e, matrix as a CLI end-to-end", anchor: "#co-1" }
  - { id: co-2, text: "the feature-predecessor flip is a status-ONLY edit to a frozen spec — VL-010's sanctioned exception (the same one that admits the story flip); the frozen content stays byte-identical except the single `status:` line, and the flip is committed in the same acceptance commit as the successor's own flip (the round-5 D-12 discipline carried verbatim to the feature rung)", anchor: "#co-2" }
  - { id: co-3, text: "\"legible without consulting backlinks\" (03 §rung 3) is the operative property: the terminal state must be readable from the spec's own rendered `status` on each surface, not merely inferable from a `superseded-by` relation. The story satisfies this at both rungs on all three surfaces or it is not done", anchor: "#co-3" }
open_questions:
  - { id: oq-1, text: "RESOLVED (owner, 2026-07-13): should a superseded feature predecessor's terminal state be a status flip (mirroring stories) or a rendered relation only? Owner chose the status flip; resolved into dc-1. The residual closed->superseded sub-case is deferred by dc-2", anchor: "#oq-1" }
frozen: { at: 2026-07-13, commit: dab21cfcca85a497b80a1bc8be9ba7cdde856476, stub_matched: true }
---
# Feature Supersession State

## Problem

Round 5's D-12 made `superseded` a terminal **status** at the story rung:
accepting a spec that carries a `supersedes` edge to a predecessor *story*
flips that predecessor's `status` to `superseded` in the same `verdi accept`
ritual (02 §Kind registry; accept.go's `supersedePredecessors`). The design
intent (03 §rung 3) is explicit about *why* it is a status flip and not a
backlink lookup: it makes the predecessor's state legible "everywhere without
consulting backlinks."

Two gaps remain, and this story closes both.

**The story-rung flip has never been proven legible.** It is proven only that
the frontmatter field changes — no surface test shows a human ever *sees* it.
`verdi matrix spec/disclosure-seam` (a real superseded story in this corpus)
prints the AC table and `story.eligible`, but **no status line at all**: an
operator reading matrix cannot tell the story is superseded. The board renders
no superseded badge. dex has `.badge-superseded` CSS and computes a
`superseded-by` backlink, but no test asserts either renders for a spec.

**The feature rung has no terminal state at all.** Accepting a feature v2 that
carries a whole-spec `supersedes` edge to v1 computes and prints a blast-radius
quorum label (blastradius.go) but never touches v1's `status`. `superseded` is
a legal value for a feature (the status enum is shared with stories), yet no
ritual ever writes it, so a superseded feature is discoverable only by chasing
the `superseded-by` backlink — the precise thing the rung-3 design avoids. 02
§Kind registry names this by name and defers it: "A superseded **feature**
predecessor's status remains governed by the rung-4 cascade machinery for now —
its terminal-state question is carried to round 6." This is round 6.

## Outcome

A superseded spec's terminal state is legible at both rungs on every surface
that renders specs. The existing story-rung flip is proven visible on `verdi
matrix`, the board, and dex; and an equivalent feature-rung mechanism — the
same-ritual `verdi accept` status flip, extended to a superseding *feature* —
makes a superseded feature's terminal state readable the same way, from the
spec's own rendered `status`, without consulting backlinks. The two rungs
become legible by the same means.

## AC-1

`verdi accept` of a feature spec carrying a whole-spec `supersedes` edge to a
feature predecessor flips that predecessor's `status` from
`accepted-pending-build` to `superseded` in the **same acceptance commit** — a
sanctioned status-only edit under VL-010's existing frozen-file exception (no
new lint rule is required; the exception already admits exactly this diff
shape). It mirrors the rung-3 story flip that `supersedePredecessors` performs
today, extended to a feature-class target via the whole-spec supersedes edge
`blastradius.go` already identifies. The rung-4 cascade/blast-radius machinery
that governs the predecessor's downstream *stories* is untouched: the flip is a
statement about the predecessor feature's own terminal lifecycle, orthogonal to
its stories' verdicts. Evidence: static (the flip logic and its VL-010
compatibility are declared and compile/lint clean) and behavioral (an exerciser
accepts a fixture feature v2 that supersedes v1 and observes v1's `status` flip
to `superseded`, hermetically).

## AC-2

A superseded spec's terminal state renders — not deleted, not silent — on every
surface that renders specs, at both rungs:

- **`verdi matrix`**: the story-level matrix prints the spec's `status` (so a
  superseded story is announced, closing the blindness above); the
  feature-level matrix shows a superseded implementing story with a terminal
  marker instead of silently dropping it from the fold.
- **the board**: a superseded spec renders a `superseded` badge/affordance.
- **dex**: a superseded spec's page shows its `superseded` status badge (the
  latent `.badge-superseded` CSS, now proven) at both rungs.

so that finding a superseded predecessor never requires reading raw frontmatter
or chasing a `superseded-by` backlink. Evidence: behavioral (an exerciser
confirms the terminal state renders on each surface at each rung — matrix as a
CLI end-to-end over the real `disclosure-seam` case and a fixture superseded
feature; the board and dex under Playwright e2e).

## DC-1

Feature-predecessor terminal state is a **status flip** to `superseded`,
owner-resolved (oq-1, 2026-07-13). It mirrors the rung-3 flip D-12 shipped:
`verdi accept` of a feature v2 carrying a whole-spec `supersedes` edge to a
feature predecessor flips the predecessor's `status` to `superseded` in the
same ritual. Chosen over a rendered-relation-only reading because only the
status flip delivers legibility "without consulting backlinks" (03 §rung 3) —
the property that makes the two rungs legible by the same means, from the
spec's own `status` field, on every surface.

## DC-2

The flip fires only from `accepted-pending-build`, mirroring the story guard in
`supersedePredecessors` exactly. A `closed` feature being superseded (a
`closed -> superseded` transition) is the rarer case and is **deferred** here as
the smallest reversible scope: a closed feature is already terminally legible
(`closed`), and its supersession is still surfaced by the computed
`superseded-by` backlink. Handling the common in-progress-predecessor case now
and reopening the closed case cleanly later is recorded in the invention
ledger, not resolved silently.

## DC-3

`verdi matrix` is the weakest surface and this story fixes it at both rungs.
Today `printMatrix` prints no `status` line (story rung) and
`discoverImplementingStories` `continue`s past — silently deletes — any
superseded story (feature rung). After this story the story-level matrix prints
the spec's `status`, and the feature-level matrix renders a superseded
implementing story with a terminal marker rather than dropping it. dex already
carries the `.badge-superseded` CSS and the `superseded-by` backlink computation
(latent, unproven); the board gains a superseded badge. All three surfaces are
then proven by an exerciser — the "legibility" the AC demands is rendering that
a test observes, not code that merely exists.

## DC-4

Legibility is proven on verdi's real case where one exists and honestly
fixtured where it does not. The **story rung** uses the corpus's real
superseded story — `disclosure-seam`, flipped to `superseded` when
`disclosure-seam-v2` was accepted. verdi's own corpus has **no** superseded
feature, so the **feature rung**'s accept-flip and its rendering are proven
hermetically over a fixturegit feature-v2-supersedes-v1 pair. This is the same
honest scope runtime-evidence dc-3 took: the mechanism is proven and a real
feature supersession plugs into it unchanged; the story does not invent a
superseded verdi feature that does not exist.

## CO-1

No network in any test. The accept flip, the surface rendering (matrix, board,
dex), and both rungs are exercised hermetically: over the real `disclosure-seam`
case for the story rung and a fixturegit feature-v2/v1 pair for the feature
rung; the board and dex surfaces under Playwright e2e; matrix as a CLI
end-to-end driving the built binary.

## CO-2

The feature-predecessor flip is a status-ONLY edit to a frozen spec — VL-010's
sanctioned exception, the same one that admits the story flip. The frozen
content stays byte-identical except the single `status:` line, and the flip is
committed in the same acceptance commit as the successor's own acceptance flip.
This is the round-5 D-12 discipline carried verbatim to the feature rung: the
frozen v1 is preserved, never content-edited; only its status field moves.

## CO-3

"Legible without consulting backlinks" (03 §rung 3) is the operative property.
The terminal state must be readable from the spec's own rendered `status` on
each surface, not merely inferable from a `superseded-by` relation a reader has
to go find. The story satisfies this at both rungs on all three surfaces — the
status is shown where the spec is shown — or it is not done.

## OQ-1

RESOLVED (owner, 2026-07-13). The question: should a superseded feature
predecessor's terminal state be a status flip (mirroring stories) or a rendered
relation only? The owner chose the status flip; it is resolved into dc-1. The
residual `closed -> superseded` sub-case is deferred by dc-2, disclosed in the
invention ledger rather than decided silently.

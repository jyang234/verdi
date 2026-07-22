---
id: spec/obligation-seam
kind: spec
title: "Obligation Seam"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-P2-9
problem: { text: "spec/creation-surfaces#ac-4 (X-9) names a gap the design wave's re-attack pinned exactly (design doc §12 + addendum, ledger L-N8): internal/workbench/obligationauthor.go's renderObligation/writeObligationFile already exist but are unexported and board-only, so nothing at accept time enforces that a story's declared (ac, kind) evidence pairs actually have obligations behind them. Worse, accept's own quartet lint gate (cmd/verdi/accept.go's lintQuartetOrRefuse, called before the status flip) runs while the spec's on-disk status is still draft, so VL-020's own co-2 draft-tolerance means the very check that exists never actually fires during accept — a story can freeze declaring evidence kinds with zero obligations stating what that evidence must specifically show, and nothing catches it. Design-branch authors have no surface to fix this ahead of time either: the renderer's only caller is the board's sticky-graduate action. This story is itself accepted under that exact gap, by hand, per the ritual-integrity precedent (obligations authored WITH the spec, DC-3/X-9) — the dogfood irony is deliberate: it is the last story this build will ever have to hand-author obligations for, since this story's own build is what makes that hand-work unnecessary for every story that accepts after it.", anchor: problem }
outcome: { text: "accept's freeze-moment backstop computes preFlipHead first, scaffolds exactly the missing (ac, kind) pairs to disk before the in-ritual lint gate ever runs, stamps every scaffolded obligation preFlipHead — identical to the spec's own flip stamp — and stages the newly-created paths into the accept commit itself, so a story is born with its declared evidence kinds' obligations already in hand and the pairing can never be replayed away. The backstop skips, never overwrites, any pair an already-decodable obligation already covers, keyed on the same coverage predicate VL-020 itself uses; on any refusal or error after scaffolding it unlinks exactly the obligations it newly created this invocation, leaving pre-existing files and the rest of the tree untouched. verdi obligation author gives the design branch a separate, pre-freeze surface for authoring or regenerating an obligation ahead of accept, through the identical shared renderer seam (never a second re-render in cmd/verdi) — and refuses outright on any obligation a merge to main has already frozen, since a frozen obligation is superseded through the normal ladder, never refined in place. None of this reverses VL-020's own draft-tolerance (evidence-obligations co-2 stands, vl020.go unchanged): the gap this story closes was always the freeze moment itself, by construction, never authoring-time tolerance.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "accept computes preFlipHead before running its in-ritual lint gate, then scaffolds a stub obligation for every declared (ac, kind) pair the story's own quartet is missing, before lintQuartetOrRefuse (accept.go:142) ever runs; every scaffolded stub is stamped frozen: { at, commit: preFlipHead } — the exact same stamp the spec's own flip writes moments later, never the not-yet-created accept commit — and the newly-scaffolded paths join accept's own scoped addPaths set so they land inside the same accept commit as the status flip, proven by asserting the accept commit's own diff contains them (the X-9 'replay impossible' pin: a bare git log cannot show the pairing ever having been separable)", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "the backstop never overwrites an existing obligation: for a story with some (ac, kind) pairs already covered by a hand- or board-authored obligation and others missing, only the missing pairs are scaffolded and every pre-existing obligation file is byte-untouched; coverage is decided by the same decode-based predicate VL-020 itself applies (a DECODABLE obligation.md at the .verdi/obligations/<spec>/<ac>--<kind>.md convention path), never a bare os.Stat, so a present-but-malformed file is never mistaken for coverage", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "on any refusal or operational error accept hits after scaffolding has begun — an unrelated quartet lint violation, a downstream write failure, anything — accept unlinks exactly the obligation paths it newly created this invocation and leaves everything else exactly as it found it: pre-existing obligations untouched, no partial commit, a pristine working tree. Induced by forcing an unrelated quartet refusal after scaffolding has run, asserting the tree matches its pre-scaffold state byte for byte and that a subsequent obligation author or accept retry is never blocked by an orphaned stub the backstop itself left behind", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "internal/workbench/obligationauthor.go's renderObligation, writeObligationFile, and pre-write self-validate are exported/extracted into exactly one shared seam that accept's backstop, the existing board sticky-graduate action, and verdi obligation author all three call — proven both behaviorally (the board's existing obligation-graduate tests pass completely unmodified, byte-identical rendered output) and statically (a source-text witness proves cmd/verdi carries no second render/self-validate implementation, mirroring the existing TestObligationAuthor_AtomicWrite_NoDirectCreateTemp convention)", evidence: [behavioral, static], anchor: ac-4 }
  - { id: ac-5, text: "verdi obligation author <story-ref> <ac-id> <kind> is the design-branch, pre-freeze authoring/regeneration surface: given a declared (story, ac) pair and a known evidence kind, it creates the obligation when none exists yet at the convention path and regenerates (overwrites) it when one exists but is not yet frozen, through the identical shared renderer seam ac-4 establishes — and it refuses outright, exit 2, naming the path, when the target obligation is already frozen by a merge to main (frozen decided the same way VL-010 scopes immutability: reachable from merge-base(HEAD, default branch), never merely 'exists on disk')", evidence: [behavioral], anchor: ac-5 }
links:
  - { type: implements, ref: "spec/creation-surfaces#ac-4" }
frozen: { at: 2026-07-21, commit: 67a8643f30fc797295cd1c66245c9ec523d54ec9, stub_matched: true }
---
# Obligation Seam

## Problem

`spec/creation-surfaces#ac-4` (X-9) names a gap the design wave's re-attack
pinned exactly (design doc §12 + addendum, ledger L-N8 as adjudicated at
Task 8): `internal/workbench/obligationauthor.go`'s `renderObligation`/
`writeObligationFile` already exist but are unexported and reachable only
from the board's sticky-graduate action. Nothing at accept time enforces
that a story's declared `(ac, kind)` evidence pairs actually have
obligations behind them.

Worse, the check that already exists (VL-020) never actually fires during
accept: `cmd/verdi/accept.go`'s quartet lint gate (`lintQuartetOrRefuse`,
called before the status flip) runs while the spec's on-disk status is
still `draft`, and VL-020's own co-2 draft-tolerance tolerates every draft
unconditionally — so a story can freeze declaring evidence kinds with zero
obligations stating what that evidence must specifically show, and nothing
catches it. Design-branch authors have no way to get ahead of this either:
the renderer's only caller is the board.

This story is itself accepted under that exact gap, by hand, per the
ritual-integrity precedent (obligations authored WITH the spec, DC-3/X-9)
— deliberately: it is the last story this build will ever have to
hand-author obligations for, since this story's own build is what makes
that hand-work unnecessary for every story that accepts after it.

## Outcome

Accept's freeze-moment backstop computes `preFlipHead` first, scaffolds
exactly the missing `(ac, kind)` pairs to disk before the in-ritual lint
gate ever runs, stamps every scaffolded obligation `preFlipHead` —
identical to the spec's own flip stamp — and stages the newly-created
paths into the accept commit itself, so a story is born with its declared
evidence kinds' obligations already in hand and the pairing can never be
replayed away. The backstop skips, never overwrites, any pair an
already-decodable obligation already covers, keyed on the same coverage
predicate VL-020 itself uses; on any refusal or error after scaffolding it
unlinks exactly the obligations it newly created this invocation, leaving
pre-existing files and the rest of the tree untouched.

`verdi obligation author` gives the design branch a separate, pre-freeze
surface for authoring or regenerating an obligation ahead of accept,
through the identical shared renderer seam — never a second re-render in
`cmd/verdi` — and refuses outright on any obligation a merge to main has
already frozen, since a frozen obligation is superseded through the normal
ladder, never refined in place.

None of this reverses VL-020's own draft-tolerance (evidence-obligations
co-2 stands, `vl020.go` unchanged): the gap this story closes was always
the freeze moment itself, by construction, never authoring-time tolerance.

## Ac 1

Accept computes `preFlipHead` before running its in-ritual lint gate, then
scaffolds a stub obligation for every declared `(ac, kind)` pair the
story's own quartet is missing, before `lintQuartetOrRefuse` (accept.go:142)
ever runs. Every scaffolded stub is stamped `frozen: { at, commit:
preFlipHead }` — the exact same stamp the spec's own flip writes moments
later, never the not-yet-created accept commit. The newly-scaffolded paths
join accept's own scoped `addPaths` set so they land inside the same
accept commit as the status flip — proven by asserting the accept commit's
own diff contains them (the X-9 "replay impossible" pin: a bare git log
cannot show the pairing ever having been separable).

## Ac 2

The backstop never overwrites an existing obligation. For a story with some
`(ac, kind)` pairs already covered by a hand- or board-authored obligation
and others missing, only the missing pairs are scaffolded and every
pre-existing obligation file is byte-untouched. Coverage is decided by the
same decode-based predicate VL-020 itself applies — a DECODABLE
obligation.md at the `.verdi/obligations/<spec>/<ac>--<kind>.md` convention
path — never a bare `os.Stat`, so a present-but-malformed file is never
mistaken for coverage.

## Ac 3

On any refusal or operational error accept hits after scaffolding has
begun — an unrelated quartet lint violation, a downstream write failure,
anything — accept unlinks exactly the obligation paths it newly created
this invocation and leaves everything else exactly as it found it:
pre-existing obligations untouched, no partial commit, a pristine working
tree. Induced by forcing an unrelated quartet refusal after scaffolding has
run, asserting the tree matches its pre-scaffold state byte for byte and
that a subsequent `obligation author` or accept retry is never blocked by
an orphaned stub the backstop itself left behind.

## Ac 4

`internal/workbench/obligationauthor.go`'s `renderObligation`,
`writeObligationFile`, and pre-write self-validate are exported/extracted
into exactly one shared seam that accept's backstop, the existing board
sticky-graduate action, and `verdi obligation author` all three call.
Proven both behaviorally (the board's existing obligation-graduate tests
pass completely unmodified, byte-identical rendered output) and statically
(a source-text witness proves `cmd/verdi` carries no second render/
self-validate implementation, mirroring the existing
`TestObligationAuthor_AtomicWrite_NoDirectCreateTemp` convention).

## Ac 5

`verdi obligation author <story-ref> <ac-id> <kind>` is the design-branch,
pre-freeze authoring/regeneration surface: given a declared `(story, ac)`
pair and a known evidence kind, it creates the obligation when none exists
yet at the convention path and regenerates (overwrites) it when one exists
but is not yet frozen, through the identical shared renderer seam ac-4
establishes. It refuses outright, exit 2, naming the path, when the target
obligation is already frozen by a merge to main — frozen decided the same
way VL-010 scopes immutability: reachable from `merge-base(HEAD, default
branch)`, never merely "exists on disk". Disclosed reading, controller-
reviewed: when the default branch (and so the merge-base) cannot be
resolved at all, this verb proceeds as create/regenerate rather than
refusing — the same "can't prove it, don't guess" posture VL-010 itself
wears when its own `DiffBase` is unknown, deliberately chosen over a
fail-closed refusal because the alternative would make the verb unusable
in exactly the hermetic, no-configured-remote layouts every other verb on
this seam already tolerates.

## Process note

Disclosed ritual-order deviation: `verdi build start` was run AFTER this
story's design/accept/build work was already complete on this same
branch (build-start-after-build), not before as the canonical order
names — a controller-directed correction mid-build, the branch content
is identical to what running it first would have produced; only the
order of ritual steps differs.

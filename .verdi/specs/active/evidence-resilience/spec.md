---
id: spec/evidence-resilience
kind: spec
title: "Evidence Resilience"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-P2-3
problem: { text: "the closure gate's ancestry check hard-fails with an operational exit 2 (\"Not a valid commit name\") whenever a synced CI evidence bundle carries a record referencing a commit that no longer exists anywhere — which happens routinely once the branch that produced the evidence is deleted after its PR merges; this bit model-schema's closure twice in the same round, from an unrelated story's branch deletion (X-15), and re-syncing did not help because the same poisoned bundle stayed the latest successful CI run; VL-009 has the identical hole from the opposite side — its is-a-real-commit check is satisfiable by a locally-dangling object that no branch or ref reaches, a false green (X-11b) that would let a closure-time check believe evidence is sound when its upstream source has already been deleted", anchor: problem }
outcome: { text: "sync quarantines, rather than silently drops or hard-fails on, any evidence record whose referenced commit is not reachable from HEAD at sync time, annotating the record with the quarantine reason and keeping it rather than discarding it; the closure gate's ancestry check reads a quarantined record as a per-record disclosed-unproven against the acceptance criterion it would otherwise have evidenced, never as an operational failure, so a branch deletion — however unrelated — can never again brick a story's closure; VL-009 tightens from \"is a real commit\" to \"is reachable from HEAD\", closing the false green from the other direction without changing behavior for any commit that legitimately is reachable", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "sync quarantines rather than drops or hard-fails on a record referencing a commit unreachable from HEAD at sync time: the record is kept with the quarantine reason annotated on it, and sync itself exits 0 for this shape — never an operational failure just because one record's source commit is gone", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "the closure gate's ancestry check reads a quarantined record as a per-record disclosed-unproven against the acceptance criterion it would have evidenced, never as an operational exit 2 (\"Not a valid commit name\") — so deleting a branch, however unrelated to the story being closed, can never again brick that story's closure (X-15's exact regression pinned negative)", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "VL-009's commit check tightens from \"is a real commit\" — satisfiable by a locally-dangling object no ref or branch reaches, X-11b's exact false green — to \"is reachable from HEAD\": a fixture with a dangling-but-locally-present frozen.commit reds, while a legitimately reachable commit is unaffected", evidence: [behavioral], anchor: ac-3 }
links:
  - { type: implements, ref: "spec/ritual-integrity#ac-3" }
---
# Evidence Resilience

## Problem

The closure gate's ancestry check hard-fails today with an *operational*
exit 2 ("Not a valid commit name") the moment a synced CI evidence
bundle carries even one record referencing a commit that no longer
exists anywhere in the repository's reachable history. This is not a
rare edge case: it happens routinely, as soon as the branch that
produced the evidence is deleted after its own pull request merges — the
ordinary lifecycle of a feature branch. It bit `model-schema`'s closure
twice in the same round, both times from an *unrelated* story's branch
deletion (X-15), and simply clearing and re-syncing did not help, because
the same poisoned bundle remained the latest successful CI run available
to sync. `VL-009` carries the identical hole from the opposite direction:
its own commit check today proves only "is a real commit" — a predicate
a locally-dangling object satisfies even when no branch or ref anywhere
reaches it — which is exactly X-11b's false green, a `frozen.commit` that
*looks* pinned to real history but that history has already stopped
retaining as reachable.

## Outcome

`sync` quarantines, rather than silently drops or hard-fails on, any
evidence record whose referenced commit is not reachable from `HEAD` at
sync time. The record is kept, annotated with the quarantine reason,
never discarded outright — the exact shape a deleted branch produces once
its PR has merged and CI evidence for it has already been captured. The
closure gate's ancestry check — today's exact X-15 failure point — reads
a quarantined record as a **per-record disclosed-unproven** against the
acceptance criterion it would otherwise have evidenced, rather than as an
operational failure: the gate's own verdict stays honest (an AC that only
that record would have evidenced is not silently marked proven), while
the exit-code discipline is preserved (a quarantined record is never, by
itself, an operational failure). A branch deletion, however unrelated to
the story actually being closed, can never again brick that closure.
`VL-009`'s own commit check tightens the same direction from the
opposite side: from "is a real commit" (X-11b's exact false green) to
"is reachable from `HEAD`" — closing that hole without changing behavior
for any commit that legitimately remains reachable.

## Ac 1

`sync` quarantines, rather than drops or hard-fails on, a record whose
referenced commit is not reachable from `HEAD` at sync time — driven
against `fixturegit` cases mirroring `cmd/verdi/sync_ancestor_test.go`'s
existing home: a bundle referencing a commit that only ever lived on a
now-deleted branch. The record must be kept, never silently removed from
the synced set, and annotated with the quarantine reason (the specific,
smallest-reversible shape — an annotation on the record — this story
adds). `sync` itself must exit 0 for this shape: one unreachable-commit
record is never, by itself, grounds for `sync` to report an operational
failure.

## Ac 2

The closure gate's ancestry check — the exact check that today hard-fails
with operational exit 2 and the literal text "Not a valid commit name" on
this shape, model-schema's own X-15 witness twice in one round — instead
reads a quarantined record as a **per-record disclosed-unproven** against
the acceptance criterion it would have evidenced. The gate's own overall
verdict must stay honest: an AC whose *only* evidence was the quarantined
record is not silently marked proven, but the closure run itself does not
exit operationally just because that one record degraded. This is proven
as a negative regression pin: a fixture reproducing X-15's exact shape
(an unrelated story's branch, deleted, whose evidence the story being
closed also references) must no longer brick that closure.

## Ac 3

`VL-009`'s commit check (`internal/lint/vl009.go`) tightens from "is a
real commit" — a predicate a locally-dangling object satisfies even
though no branch or ref anywhere reaches it, X-11b's exact false green —
to "is reachable from `HEAD`". A fixture pinning a `frozen.commit` that
exists as a locally-dangling object (created, then stripped of every ref
that would keep it reachable) must red under the tightened check. A
fixture whose `frozen.commit` legitimately is reachable through ordinary
history must be entirely unaffected — this tightening closes the false
green without narrowing what a legitimately pinned commit is allowed to
be.

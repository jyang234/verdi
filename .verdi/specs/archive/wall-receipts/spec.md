---
id: spec/wall-receipts
kind: spec
title: "Wall Receipts"
owners: [platform-team]
class: feature
status: closed
problem: { text: "the wall computes nothing it knows: lint findings, ladder flags, and evidence folds all exist, but no card carries them, so readiness is discovered at MR time and an unexplained verdict trains authors to game the badge rather than fix the cause", anchor: problem }
outcome: { text: "every computed claim on the wall is a badge that opens its derivation — the rule, the pinned inputs, the firing records — and readiness is ambient during authoring, never a surprise at review", anchor: outcome }
acceptance_criteria:
  - { id: ac-2, text: "every wall badge opens a derivation drawer naming the rule, the pinned inputs with their revisions, and the records that fired it", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "an acceptance criterion card renders its declared evidence kinds, and an empty evidence slot badges — disclosed, never blocking", evidence: [behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "spec-stale and pending-supersession render on the case file, computed identically to the dex story-lens flags", evidence: [behavioral], anchor: "#ac-4" }
  - { id: ac-5, text: "an acceptance-criteria column exceeding the viewport raises a size-smell badge on the case file — an observation, never a rule", evidence: [behavioral], anchor: "#ac-5" }
  - { id: ac-6, text: "judged findings on the wall display their sweep provenance, so a stale or partial sweep looks stale", evidence: [behavioral], anchor: "#ac-6" }
stubs:
  - { slug: badge-computes, acceptance_criteria: [ac-2, ac-4] }
  - { slug: derivation-drawer, acceptance_criteria: [ac-2, ac-6] }
  - { slug: evidence-slot, acceptance_criteria: [ac-3] }
  - { slug: case-file-flags, acceptance_criteria: [ac-4, ac-5] }
constraints:
  - { id: co-1, text: "badges compute with no LLM and read only pinned inputs; the drawer cites input revisions, never wall-clock time", anchor: "#co-1" }
  - { id: co-2, text: "badges never block authoring: disclosure, not refusal — readiness is ambient chrome, enforced only at MR time", anchor: "#co-2" }
decisions:
  - { id: dc-1, text: "the v1 badge set is existing computes only — VL lint findings scoped to this spec, spec-stale, pending-supersession, empty evidence slots, size-smell; no computation is invented for a badge", anchor: "#dc-1" }
  - { id: dc-2, text: "a derivation drawer names the rule id, the pinned inputs with revisions, and the firing records — receipts, not verdicts; an unexplained badge trains authors to game it", anchor: "#dc-2" }
  - { id: dc-3, text: "a VL finding is a wall badge only if it anchors to a rendered element (ac-2/dc-2: a receipt must name a firing record) — object-anchored findings badge their card, spec-level findings badge the case file, and store-structural or plumbing findings (gitattributes, data-tracking, status-path, dangling layout keys, decode failures) stay in verdi lint/CI, off the wall; a new rule classifies itself by whether it anchors", anchor: "#dc-3" }
  - { id: dc-4, text: "the ADR exemption count is out of v1 scope — it attaches to an external reference card, not this spec's own objects, so it rides the later context-rail pass (with parent-feature fold-status cards), reusing this spec's derivation-drawer machinery (ac-2) rather than being owned here", anchor: "#dc-4" }
frozen: { at: 2026-07-13, commit: d3b4a012200f29116ea8e5842922a2f1dbafd429 }
---
# Wall Receipts

## Problem

The store already computes the truths an author needs while authoring —
VL lint findings, spec-stale and pending-supersession ladder flags,
evidence-fold status — and the wall renders none of them. Readiness is
discovered at MR time, as a surprise. And a verdict without a receipt is
worse than none: an unexplained red dot trains authors to game the badge
rather than fix the cause, which is how disclosure regimes rot.

## Outcome

Every computed claim on the wall is a badge that opens its derivation —
the rule that fired, the pinned inputs with their revisions, the specific
records behind it. Readiness becomes ambient chrome during authoring and
is enforced only at MR time. Receipts, not verdicts.

## ac-2

every wall badge opens a derivation drawer naming the rule, the pinned inputs with their revisions, and the records that fired it

## ac-3

an acceptance criterion card renders its declared evidence kinds, and an empty evidence slot badges — disclosed, never blocking

## ac-4

spec-stale and pending-supersession render on the case file, computed identically to the dex story-lens flags

## ac-5

an acceptance-criteria column exceeding the viewport raises a size-smell badge on the case file — an observation, never a rule

## ac-6

judged findings on the wall display their sweep provenance, so a stale or partial sweep looks stale

## co-1

badges compute with no LLM and read only pinned inputs; the drawer cites input revisions, never wall-clock time

## co-2

badges never block authoring: disclosure, not refusal — readiness is ambient chrome, enforced only at MR time

## dc-1

the v1 badge set is existing computes only — VL lint findings scoped to this spec, spec-stale, pending-supersession, empty evidence slots, size-smell; no computation is invented for a badge

## dc-2

a derivation drawer names the rule id, the pinned inputs with revisions, and the firing records — receipts, not verdicts; an unexplained badge trains authors to game it

## dc-3

The discriminator is not a hand-picked allowlist; it falls out of ac-2
and dc-2. A wall badge opens a drawer naming the records that fired it, so
a finding can only be a badge if it has a rendered record to point at.
Three buckets: object-anchored findings (a dangling link ref, a missing
evidence kind, a dangling stub ref, an unresolved open question) badge the
card of the object they name; spec-level findings with no single object
anchor (a missing required attribute, size-smell, spec-stale,
pending-supersession) badge the case file; store-structural and
cross-file plumbing findings (gitattributes, the data-tracking rule,
status-in-path, a dangling layout.json key, and decode failures — which
are unparsed-island territory, not a badge at all) have no record the
author can act on from the wall and stay in verdi lint / CI. The partition
is self-maintaining: a new VL rule becomes a wall badge iff it anchors to
a rendered element, satisfying ac-2 by construction.

## dc-4

The "ADR-N: M active exemptions" count at the point of temptation is
on-thesis — a computed claim with a real derivation — but it attaches to
an external reference card (an ADR this spec exempts), not to any of this
spec's own objects, which is what every wall-receipts AC is about.
Reference cards with live counts are the context rail's domain (with
parent-feature fold-status cards), a distinct later pass. So the count is
out of v1 scope here — but wall-receipts builds the machinery it will
reuse: the derivation drawer (ac-2) is exactly the receipt surface the
count needs when the context-rail pass lands. This spec enables it without
owning it.

---
id: spec/obligation-gate
kind: spec
title: "Obligation Gate"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-6
problem: { text: "the obligation artifact now exists and can be authored on the wall, but nothing REQUIRES it: a story AC can still declare `evidence: [behavioral]` with no obligation stating what that behavioral evidence must show, and the spec activates anyway. The whole point of evidence-obligations (feature ac-2) is that a declared kind without an obligation cannot activate — otherwise obligations are optional decoration, not a gate.", anchor: "#problem" }
outcome: { text: "an activation lint — VL-006's obligation-shaped sibling — refuses a STORY spec whose acceptance criterion declares an evidence kind with no matching obligation object (the file `.verdi/obligations/<spec-name>/<ac-id>--<kind>.md`, the spec-name keying obligation-artifact settled). A spec may not be accepted saying what KIND of evidence it wants without an obligation saying what that evidence must specifically show; the refusal names each missing (ac, kind). Feature ACs are exempt (obligations are story-only), authoring a draft is never blocked (only activation is), and the fold is unchanged — the gate is at activation, not on the evidence record (feature oq-1's resolution).", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "an activation lint refuses a STORY spec whose AC declares an evidence kind with no matching obligation object at `.verdi/obligations/<spec-name>/<ac-id>--<kind>.md` — VL-006's obligation-shaped sibling; a story AC declaring `[behavioral]` with no behavioral obligation cannot activate, the refusal naming the missing (ac, kind), and with the obligation present it passes", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "the gate is correctly scoped: a FEATURE AC declaring kinds requires NO obligation (obligations are story-only, feature dc-3), and the timing mirrors VL-006 exactly (activation, not authoring — a draft is never refused for a missing obligation; only the accepted-pending-build / accept ritual is gated, feature co-2). The `verdi.evidence/v1` record and the fold are UNCHANGED — no obligation_id, no fold-match change (feature oq-1's resolution: the gate is at activation, not on the record)", evidence: [static, behavioral], anchor: "#ac-2" }
links:
  - { type: implements, ref: "spec/evidence-obligations#ac-2" }
decisions:
  - { id: dc-1, text: "the gate is a new VL lint rule (next free number after VL-019), modeled on VL-006 (every AC declares >=1 kind) — its obligation-shaped sibling: for each STORY AC and each evidence kind it declares, an obligation file must exist at `.verdi/obligations/<spec-name>/<ac-id>--<kind>.md`. Spec-name keying (not story-ref-slug) is carried from obligation-artifact's own implementation — the board knows its spec name unambiguously, avoiding the D6-18 story-ref ambiguity", anchor: "#dc-1" }
  - { id: dc-2, text: "the record and fold are UNCHANGED (feature oq-1's owner-resolved answer): verdi does not own its producers, so no obligation_id is added to `verdi.evidence/v1` and the fold's (AC, kind) match is untouched. This story adds ONLY the activation lint — the obligation is 1:1 with a (story-AC, kind), so requiring it to exist at activation is the whole gate. The optional record-proven self-attest tier (feature dc-2c) is explicitly out of scope here", anchor: "#dc-2" }
  - { id: dc-3, text: "feature ACs are exempt: the lint resolves the spec's class and only requires obligations for STORY specs' ACs, mirroring how obligations themselves only `verifies` story ACs (VL-019). A feature AC's declared kinds (its coarse floor + attestation) never require an obligation", anchor: "#dc-3" }
constraints:
  - { id: co-1, text: "no network in any test: the lint is exercised table-driven over hermetic fixtures — a story AC with a declared kind and no obligation (refused, naming the missing (ac,kind)); the same with the obligation present (clean); a feature AC with kinds and no obligation (clean — exempt); the VL-006 timing mirrored", anchor: "#co-1" }
  - { id: co-2, text: "authoring is never blocked (feature co-2): the gate fires only at activation, exactly as VL-006 does — a draft story with an un-obligated kind is not refused for that reason; the refusal is reserved for the accept/activation path. Drafting an obligation on the wall to satisfy the gate is the intended authoring loop", anchor: "#co-2" }
frozen: { at: 2026-07-13, commit: f877ff019cda7d7271aeea9f4fb1d36a3449c4dd, stub_matched: true }
---
# Obligation Gate

## Problem

The obligation artifact now exists and can be authored on the wall, but nothing
REQUIRES it: a story AC can still declare `evidence: [behavioral]` with no
obligation stating what that behavioral evidence must show, and the spec
activates anyway. The whole point of the feature (evidence-obligations ac-2) is
that a declared kind without an obligation cannot activate — otherwise
obligations are optional decoration, not a gate.

## Outcome

An activation lint — VL-006's obligation-shaped sibling — refuses a STORY spec
whose acceptance criterion declares an evidence kind with no matching obligation
object (the file `.verdi/obligations/<spec-name>/<ac-id>--<kind>.md`, the
spec-name keying obligation-artifact settled). A spec may not be accepted saying
what KIND of evidence it wants without an obligation saying what that evidence
must specifically show; the refusal names each missing (ac, kind). Feature ACs
are exempt (obligations are story-only), authoring a draft is never blocked
(only activation is), and the fold is unchanged — the gate is at activation, not
on the evidence record.

## AC-1

An activation lint refuses a STORY spec whose AC declares an evidence kind with
no matching obligation object at `.verdi/obligations/<spec-name>/<ac-id>--<kind>.md`
— VL-006's obligation-shaped sibling. A story AC declaring `[behavioral]` with
no behavioral obligation cannot activate, the refusal naming the missing (ac,
kind); with the obligation present it passes. Evidence: static (the rule is
declared, wired into the lint walk) + behavioral (table-driven refuse/pass over
fixtures).

## AC-2

The gate is correctly scoped. A FEATURE AC declaring kinds requires NO obligation
(obligations are story-only, feature dc-3). The timing mirrors VL-006 exactly —
activation, not authoring: a draft is never refused for a missing obligation;
only the accepted-pending-build / accept ritual is gated (feature co-2). And the
`verdi.evidence/v1` record and the fold are UNCHANGED — no `obligation_id`, no
fold-match change: feature oq-1's resolution is that the gate is at activation,
not on the record verdi does not produce. Evidence: static + behavioral.

## DC-1

The gate is a new VL lint rule (the next free number after VL-019), modeled on
VL-006 (every AC declares ≥1 kind) — its obligation-shaped sibling: for each
STORY AC and each evidence kind it declares, an obligation file must exist at
`.verdi/obligations/<spec-name>/<ac-id>--<kind>.md`. Spec-name keying (not
story-ref-slug) is carried from obligation-artifact's own implementation: the
board knows its spec name unambiguously, avoiding the D6-18 story-ref ambiguity.

## DC-2

The record and fold are UNCHANGED — feature oq-1's owner-resolved answer. verdi
does not own its producers, so no `obligation_id` is added to
`verdi.evidence/v1` and the fold's (AC, kind) match is untouched. This story
adds ONLY the activation lint: an obligation is 1:1 with a (story-AC, kind), so
requiring it to exist at activation is the whole gate. The optional
record-proven self-attest tier (feature dc-2c) is explicitly out of scope here.

## DC-3

Feature ACs are exempt: the lint resolves the spec's class and only requires
obligations for STORY specs' ACs, mirroring how obligations themselves only
`verifies` story ACs (VL-019). A feature AC's declared kinds — its coarse floor
plus the attestation outcome floor — never require an obligation.

## CO-1

No network in any test. The lint is exercised table-driven over hermetic
fixtures: a story AC with a declared kind and no obligation (refused, naming the
missing (ac, kind)); the same with the obligation present (clean); a feature AC
with kinds and no obligation (clean — exempt); and the VL-006 activation timing
mirrored.

## CO-2

Authoring is never blocked (feature co-2): the gate fires only at activation,
exactly as VL-006 does — a draft story with an un-obligated kind is not refused
for that reason; the refusal is reserved for the accept / activation path.
Drafting an obligation on the wall to satisfy the gate is the intended authoring
loop.

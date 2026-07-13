---
id: spec/evidence-obligations
kind: spec
title: "Evidence Obligations"
owners: [platform-team]
class: feature
status: accepted-pending-build
problem: { text: "a spec's acceptance criterion declares only coarse evidence KINDS — `evidence: [behavioral]` — which says a behavioral proof is expected but never what it must specifically show. The one place a specific artifact is named — the `producer` in `verdi.bindings.yaml` — lives in a sidecar keyed by producer id, off the AC and invisible to anyone reading the spec (D6-15/D6-17: what a gate checks is per-AC, invisible, and mis-slug-prone). So two ACs both declaring `[behavioral]` are indistinguishable on the page though they demand entirely different proofs, and a gate can pass on ANY behavioral record — one proving something unrelated — because the fold matches on kind alone. The wall renders no evidence on an AC card at all. What evidence an AC actually demands is neither legible nor enforced.", anchor: "#problem" }
outcome: { text: "a story AC's every declared evidence kind is backed by a first-class evidence OBLIGATION — a named artifact stating what that evidence must specifically show, graduated on the wall and frozen like any accepted artifact. Obligations gate at ACTIVATION: a declared story-AC kind with no obligation cannot activate — a spec may not say what KIND of evidence it wants without stating what that evidence must specifically show. Because an obligation is 1:1 with a (story-AC, kind) and the record→AC link is already verdi's own binding-owned join, the obligation needs no new field on the evidence record (verdi does not own its producers, so a record cannot be made to carry a parsable obligation id); a verdi-owned producer may optionally self-attest for a record-proven match. Feature ACs stay implementation-blind — obligations are a story-level concern. And every obligation is legible on the wall, so what an AC demands is read where the operator looks, never dug out of the bindings sidecar.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a story AC's declared evidence kind is backed by a first-class evidence-obligation artifact — a named object (`obligation/<story>--<ac>--<kind>`) stating what that evidence must specifically show, graduated from a board sticky and frozen at accept, carrying a `verifies` edge to a STORY AC fragment", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "obligations gate at ACTIVATION: a story AC cannot activate while it declares an evidence kind with no matching obligation — a lint refusal (VL-006's sibling), so a spec may not be accepted saying what KIND of evidence it wants without an obligation stating what that evidence must specifically show. Because an obligation is 1:1 with a (story-AC, kind), the fold's existing binding-owned (AC, kind) match is the obligation link and NO field is added to the evidence record (verdi does not own its producers); a verdi-owned producer may optionally self-attest to its obligation for a record-proven match — the stronger tier", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "obligations are a story-level concern only: a feature AC, being outcome-level and implementation-blind (03 §The feature fold), neither carries nor requires an obligation, and an obligation `verifies`-ing a feature AC is refused by lint — the feature/story split the model already enforces is carried to obligations unchanged", evidence: [behavioral, attestation], anchor: "#ac-3" }
  - { id: ac-4, text: "a story AC's obligations are legible on the wall: the board AC card and `verdi matrix` render each declared kind's obligation, so what evidence an AC demands is read from the AC's own rendered obligations, never recovered by reading `verdi.bindings.yaml`", evidence: [behavioral, attestation], anchor: "#ac-4" }
decisions:
  - { id: dc-1, text: "an evidence obligation is a FIRST-CLASS artifact (owner-resolved, 2026-07-13), not an inline `expects:` field: a new kind with its own id `obligation/<story-slug>--<ac-id>--<kind>`, graduated from a board sticky like an attestation, frozen at accept, carrying a `verifies` edge to its AC fragment. Chosen over the lighter inline field so an obligation is a wall object with its own identity and lifecycle the author graduates and the reader cites", anchor: "#dc-1" }
  - { id: dc-2, text: "obligations GATE, but at ACTIVATION, not on the record (owner-resolved 2026-07-13, resolving oq-1): (a) an activation lint refuses a story AC that declares an evidence kind with no matching obligation — VL-006's obligation-shaped sibling. (b) The `verdi.evidence/v1` record is UNCHANGED: verdi does not own its producers (upstream CLIs it only execs + strict-decodes, plus its own coarse make-verify producer), so a record cannot be made to carry a parsable obligation id. Instead an obligation is 1:1 with a (story-AC, kind), and the record→AC link is already verdi's OWN (the verdi.bindings.yaml join), so the fold's existing (AC, kind) match IS the obligation link, transitively — no fold change, no record field. The gate's teeth are legibility + completeness at activation ('you must state what each declared kind proves'), not a record-cryptographic match verdi cannot perform over evidence it did not produce. (c) OPTIONAL stronger tier: a producer verdi DOES control (a bespoke test, like runtime-evidence's probe) may self-attest to its obligation for a record-proven match — a graceful layer that never touches the upstream boundary", anchor: "#dc-2" }
  - { id: dc-3, text: "obligations attach to STORY ACs only. A feature AC is outcome-level and implementation-blind (03 §The feature fold; 02 §Kind registry) — it declares only its coarse kinds plus the attestation outcome floor and neither carries nor requires an obligation; an obligation whose `verifies` target resolves to a feature AC fails lint. The feature/story split the model already enforces (feature = downward-blind, story = implementation-scoped) is carried to obligations unchanged", anchor: "#dc-3" }
  - { id: dc-4, text: "the wall is the obligation's home surface — this is the first round-6 feature whose primary surface is the authoring WALL, not a CLI verb (it dogfoods the design front-end). An obligation is authored by graduating a board sticky and rendered on the AC card and `verdi matrix`; the sibling wall-receipts feature (its ac-3 renders declared evidence kinds) is the display surface an obligation lights up", anchor: "#dc-4" }
open_questions:
  - { id: oq-1, text: "RESOLVED (owner, 2026-07-13; folded into dc-2): how does a producing record attribute to an obligation, given verdi does not own its producers? Resolution — it does NOT: the record is unchanged. verdi execs upstream CLIs and strict-decodes their JSON (never teaching them its obligation ids), and its own make-verify producer is coarse (one suite-passed record per bound AC), so no record can carry a parsable obligation id. The obligation reference lives in verdi's OWN binding domain instead; and since an obligation is 1:1 with a (story-AC, kind), the binding-owned (AC, kind) link already IS the obligation link. The gate is thus activation-level (every declared kind needs an obligation) over the UNCHANGED fold, with an optional record-proven tier only for producers verdi controls", anchor: "#oq-1" }
constraints:
  - { id: co-1, text: "no network in any test: the obligation artifact (decode, validate, graduate, freeze), the activation lint, the fold's (kind, obligation) match, and the wall render are all exercised hermetically", anchor: "#co-1" }
  - { id: co-2, text: "obligations never block AUTHORING: a missing or incomplete obligation is a disclosed badge on the wall (the wall-receipts posture, \"disclosure, not refusal\"), and the gate fires only at ACTIVATION (accept) and at the fold — exactly as the kind-declared lint (VL-006) already does. Drafting a spec with an un-obligated kind on the wall is allowed; activating it is not", anchor: "#co-2" }
  - { id: co-3, text: "legible-without-the-sidecar is the operative property: what evidence an AC demands must be readable from the AC's own rendered obligations on the wall and matrix, not only recoverable by reading verdi.bindings.yaml. The feature satisfies this or it is not done", anchor: "#co-3" }
stubs:
  - { slug: obligation-artifact, acceptance_criteria: [ac-1, ac-3] }
  - { slug: obligation-gate, acceptance_criteria: [ac-2] }
  - { slug: obligation-wall, acceptance_criteria: [ac-4] }
frozen: { at: 2026-07-13, commit: 6b0f9a0924ae8360f0c5ff77f91a2b8535926565 }
---
# Evidence Obligations

## Problem

An acceptance criterion declares only coarse evidence **kinds**. `evidence:
[behavioral]` says a behavioral proof is expected; it never says *what* that
proof must demonstrate. The single place a specific producing artifact is named
— the `producer` string in `verdi.bindings.yaml` — lives in a sidecar keyed by
producer id, off the AC, invisible to anyone reading the spec. This is the
"coarse evidence" weakness recorded across round 6 (D6-15, D6-17): what a gate
checks is per-AC, invisible, and mis-slug-prone.

Two consequences. First, **illegibility**: two story ACs both declaring
`[behavioral]` are indistinguishable on the page though one demands "a Playwright
test that drives the edit form and asserts persistence across reload" and the
other "a unit test of the retry backoff." The wall renders no evidence on an AC
card at all (the projection carries no evidence field). Second, **weak
enforcement**: the fold matches on kind alone, so a gate passes on ANY behavioral
record — including one that proves something entirely unrelated to the AC's
intent. What an AC actually demands is neither read nor enforced.

## Outcome

A story AC's every declared evidence kind is backed by a first-class evidence
**obligation** — a named artifact stating what that evidence must specifically
show, graduated on the wall and frozen like any accepted artifact. Obligations
**gate**: a declared story-AC kind with no obligation cannot activate, and
evidence satisfies an obligation only when the producing record names it, so a
gate proves what was intended. Feature ACs stay **implementation-blind** —
obligations are a story-level concern. And every obligation is **legible on the
wall**, read where the operator looks rather than dug out of the sidecar.

## AC-1

A story AC's declared evidence kind is backed by a first-class
evidence-obligation artifact: a named object, id
`obligation/<story-slug>--<ac-id>--<kind>`, stating in prose what that kind of
evidence must specifically show, graduated from a board sticky and frozen at
accept, carrying a `verifies` edge to its AC fragment. It is an artifact with
its own identity and lifecycle — authored on the wall, cited by the reader —
not a free-text field buried in frontmatter. Evidence: behavioral (an obligation
is graduated, frozen, and round-trips through decode/validate) + attestation
(an operator affirms the obligation object carries the intended claim).

## AC-2

Obligations gate at **activation** — not on the record. A lint refuses a story
AC that declares an evidence kind with no matching obligation, the
obligation-shaped sibling of VL-006 (which already refuses an AC declaring no
kind): a spec may not be accepted saying what *kind* of evidence it wants
without an obligation stating what that evidence must specifically show.

The evidence record is deliberately **unchanged**. verdi does not own its
producers — it execs upstream CLIs and strict-decodes their JSON, and its own
make-verify producer is coarse — so a record cannot be made to carry a parsable
obligation id (oq-1). Instead, because an obligation is 1:1 with a (story-AC,
kind), the fold's existing binding-owned (AC, kind) match *is* the obligation
link; the gate's teeth are legibility and completeness at activation, not a
record-cryptographic match. A producer verdi *does* control may optionally
self-attest to its obligation for a record-proven match — the stronger tier.
Evidence: behavioral (the activation lint refuses, proven hermetically) +
attestation.

## AC-3

Obligations are a story-level concern only. A feature AC is outcome-level and
implementation-blind (03 §The feature fold; 02 §Kind registry: the feature is
downward-blind, its AC→story mapping only ever the computed inverse of stories'
`implements` edges) — it declares only its coarse kinds plus the mandatory
`attestation` outcome floor, and neither carries nor requires an obligation. An
obligation whose `verifies` target resolves to a feature AC fails lint. The
feature/story split the model already enforces is carried to obligations
unchanged. Evidence: behavioral + attestation.

## AC-4

A story AC's obligations are legible on the wall. The board AC card and `verdi
matrix` render each declared kind's obligation, so what evidence an AC demands is
read from the AC's own rendered obligations — never recovered by reading
`verdi.bindings.yaml`. This is the operative property (co-3): legible without the
sidecar. Evidence: behavioral (an exerciser confirms the obligation renders on
the board and matrix) + attestation.

## DC-1

An evidence obligation is a **first-class artifact**, owner-resolved
(2026-07-13), not an inline `expects:` field. It is a new kind with its own id
`obligation/<story-slug>--<ac-id>--<kind>`, graduated from a board sticky like an
attestation, frozen at accept, carrying a `verifies` edge to its AC fragment.
Chosen over the lighter inline field so an obligation is a wall object with its
own identity and lifecycle — the author graduates it, the reader cites it —
rather than prose buried in an AC.

## DC-2

Obligations **gate at activation, not on the record**, owner-resolved
(2026-07-13), resolving oq-1. (a) An activation lint refuses a story AC that
declares an evidence kind with no matching obligation — VL-006's
obligation-shaped sibling. (b) The `verdi.evidence/v1` record is **unchanged**:
verdi does not own its producers (the upstream CLIs it only execs and
strict-decodes, plus its own coarse make-verify producer), so a record cannot
carry a parsable obligation id. An obligation is 1:1 with a (story-AC, kind),
and the record→AC link is already verdi's own — the `verdi.bindings.yaml` join —
so the fold's existing (AC, kind) match *is* the obligation link, transitively.
No fold change, no record field. (c) A producer verdi controls (a bespoke test,
like runtime-evidence's probe) may optionally self-attest for a record-proven
match — a graceful layer that never touches the upstream boundary.

## DC-3

Obligations attach to **story ACs only**. A feature AC is outcome-level and
implementation-blind — it declares only its coarse kinds and the attestation
floor, never an obligation; an obligation `verifies`-ing a feature AC fails
lint. The feature-blind / story-scoped split (02 §Kind registry, 03 §The feature
fold) is carried to obligations unchanged. Note that this feature's OWN ACs,
being feature ACs, therefore carry no obligations — it does not obligate itself,
by its own rule.

## DC-4

The **wall** is the obligation's home surface. This is the first round-6 feature
whose primary surface is the authoring wall, not a CLI verb — it dogfoods the
design front-end. An obligation is authored by graduating a board sticky and
rendered on the AC card and `verdi matrix`. The sibling **wall-receipts** feature
(whose ac-3 renders an AC's declared evidence kinds) is the display surface an
obligation lights up; the two are complementary — this feature enriches the
declaration, wall-receipts displays it.

## OQ-1

**Resolved** (owner, 2026-07-13; folded into dc-2). The question: how does a
producing record attribute itself to an obligation, given verdi's own producer
is coarse and its upstream producers are foreign? The answer: **it does not —
the record is unchanged.** verdi execs upstream CLIs and strict-decodes their
JSON (it never teaches them its obligation ids), and its make-verify producer
proves the suite passed, not which test satisfied which obligation — so no
record can carry a parsable obligation id. The obligation reference lives in
verdi's own binding domain instead, and since an obligation is 1:1 with a
(story-AC, kind), the binding-owned (AC, kind) link already *is* the obligation
link. The gate is therefore activation-level over the unchanged fold, with an
optional record-proven tier only for producers verdi controls — a bespoke test
naming its own obligation, exactly like the runtime probe.

## CO-1

No network in any test. The obligation artifact (decode, validate, graduate,
freeze), the activation lint, the fold's (kind, obligation) match, and the wall
render are all exercised hermetically.

## CO-2

Obligations never block **authoring**. A missing or incomplete obligation is a
disclosed badge on the wall (the wall-receipts posture: disclosure, not
refusal), and the gate fires only at **activation** (accept) and at the fold —
exactly as the kind-declared lint VL-006 already does. Drafting a spec with an
un-obligated kind on the wall is allowed; activating it is not.

## CO-3

Legible-without-the-sidecar is the operative property. What evidence an AC
demands must be readable from the AC's own rendered obligations on the wall and
matrix, not only recoverable by reading `verdi.bindings.yaml`. The feature
satisfies this at both surfaces or it is not done.

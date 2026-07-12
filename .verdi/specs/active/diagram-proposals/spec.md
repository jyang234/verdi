---
id: spec/diagram-proposals
kind: spec
title: "Diagram Proposals"
owners: [platform-team]
class: feature
status: draft
problem: { text: "generated diagrams describe only what exists; design intent about future state has no honest home — a mutated copy of a generated diagram masquerades as truth, an outside drawing drifts silently, and nothing reconciles a shipped build against the diagram that motivated it", anchor: problem }
outcome: { text: "a designer proposes future-state diagrams — from scratch or derived from a generated base — that always disclose, per element, what exists versus what is proposed; that are verified deterministically wherever a truth generator exists; and that are reconciled against built reality in the pre-review alignment verdict", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "a flowchart proposal discloses per element, with no LLM anywhere in the computation: exists-in-truth, proposed-new, kept-but-gone (contradicted, with the removing commit as witness), and stale-base where a derived base has moved", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "proposal source text is byte-preserved by every write path; verification extraction is one-way and never rewrites the source", evidence: [static], anchor: ac-2 }
  - { id: ac-3, text: "a derived proposal offers a mechanical before-peek and reset, each reproducing the pinned base from its digest-verified inputs", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "a diagram without a truth generator carries the illustrative class: rendered, spec-tied, and disclosed as deterministically unverifiable — never silently blended with verified diagrams", evidence: [behavioral], anchor: ac-4 }
  - { id: ac-5, text: "the post-build/pre-review alignment verdict carries a diagram-alignment section: every accepted future-state flowchart is proven realized or its divergences are named with witnesses, and illustrative diagrams are listed as unverifiable rather than omitted", evidence: [behavioral, attestation], anchor: ac-5 }
  - { id: ac-6, text: "a proposal accepts at merge like any spec content, and an accepted diagram that truth later diverges from raises a diagram-stale flag computed the way spec-stale is", evidence: [behavioral], anchor: ac-6 }
  - { id: ac-7, text: "the built-in editor authors proposals only as deterministic source-text edits — code pane with live preview plus structural operations — and no author-positioned layout exists anywhere in the artifact", evidence: [behavioral], anchor: ac-7 }
  - { id: ac-8, text: "an on-demand judged sweep over a proposal yields provenance-stamped, dispositionable findings that never enter any gate's deterministic path", evidence: [behavioral], anchor: ac-8 }
constraints:
  - { id: co-1, text: "no LLM runs in diagram generation, verification, or any gate-consumed computation; judged findings are advisory and human-dispositioned only", anchor: co-1 }
  - { id: co-2, text: "truth comes from the pinned upstream flowmap CLI, strict-decoded from its graph JSON; verification never reimplements its graph semantics", anchor: co-2 }
  - { id: co-3, text: "layout is renderer-owned under a pinned mermaid version; the artifact stores no positions", anchor: co-3 }
decisions:
  - { id: dc-1, text: "mermaid text is the authored artifact; the verification graph is extracted from it one-way and read-only, failing closed to a lower verification tier rather than blocking authoring", anchor: dc-1 }
  - { id: dc-2, text: "interactive editing is structural-operations-only — add node, connect, rename, delete — and spatial positioning is refused by design", anchor: dc-2 }
  - { id: dc-3, text: "verification coverage is itself three-valued and disclosed per artifact: full (flowchart within the generator's vocabulary), partial (flowchart beyond it), illustrative (no generator)", anchor: dc-3 }
  - { id: dc-4, text: "realization is detected by regeneration diff against current truth, and divergence feeds the alignment verdict rather than any authoring-time block", anchor: dc-4 }
open_questions:
  - { id: oq-1, text: "rename semantics in the structural diff: an explicit rename operation, or declared remove+add — which yields honest witnesses?", anchor: oq-1 }
  - { id: oq-2, text: "large graphs: is the proposal unit a --root-scoped subgraph, and how is the scope declared and pinned?", anchor: oq-2 }
  - { id: oq-3, text: "which 02 amendments ratify the surface — diagram status vocabulary, derived_from + base-digest fields, the illustrative marker — and do they land as one batch?", anchor: oq-3 }
  - { id: oq-4, text: "where do illustrative diagrams attach: a spec link, or a body-figure convention?", anchor: oq-4 }
stubs:
  - { slug: proposal-artifact, acceptance_criteria: [ac-2, ac-6] }
  - { slug: verification-extractor, acceptance_criteria: [ac-1] }
  - { slug: board-editor, acceptance_criteria: [ac-3, ac-7] }
  - { slug: illustrative-class, acceptance_criteria: [ac-4] }
  - { slug: alignment-section, acceptance_criteria: [ac-5] }
  - { slug: judged-sweep, acceptance_criteria: [ac-8] }
---
# Diagram Proposals

## Problem

verdi-go's flowmap generates deterministic diagrams of what exists: mermaid
rendered as a view over a canonical graph JSON, "a view, never gated," and
the board can now pin those diagrams as planning material. But design
intent about *future* state has no honest home. A designer sketching
tomorrow's topology today has three options, all bad: mutate a copy of a
generated diagram, which then masquerades as truth the corpus cannot
distinguish from a real generation; draw in an outside tool, where nothing
verifies anything and drift is silent; or not diagram at all, which pushes
architectural thinking off the board this system exists to host.

Three absences follow. Nothing distinguishes "this is what is" from "this
is what we propose" at the element level, so a reader of a proposal cannot
tell inherited fact from invented future. Nothing detects when reality
moves underneath a proposal — the base a derived proposal forked from can
change on main without the proposal ever hearing about it. And nothing
reconciles a shipped build against the diagram that motivated it, so every
accepted design diagram begins rotting the day it merges — the quiet
failure mode of every architecture diagram ever drawn.

The deepest risk is to the determinism mission itself: a hand-mutated
diagram that *looks* generated is a lie the corpus cannot catch, and the
system's disclosure ethos — proven, contradicted-with-witness, or
disclosed-as-unproven — currently has no grammar for pictures.

## Outcome

A designer proposes future-state diagrams from scratch or derived from a
generated base. Either way the proposal always discloses, per element,
what exists versus what is proposed; is verified deterministically
wherever a truth generator exists; and is reconciled against built
reality in the post-build/pre-review alignment verdict — proven realized,
named diverged with witnesses, or honestly disclosed unverifiable. Never
silently stale, and never blocking: verification informs scrutiny, it
does not gate authoring.

## AC-1

The core disclosure, computed without any LLM. For a derived proposal:
regenerate truth from the current branch via the pinned flowmap CLI and
run a three-way structural comparison — elements the proposal inherited
from its base that truth still has are *exists*; elements the delta added
are *proposed-new* (design intent, honestly unverifiable, never
"impossible"); elements the proposal kept that truth has since dropped
are *contradicted*, with the removing commit as witness (the
oversight-catcher); a base that moved since the fork is *stale-base*,
disclosed with a rebase affordance. A from-scratch flowchart gets the
same tinting minus the base-relative checks: every element either names
something flowmap knows (*exists*) or does not (*proposed-new*). The
wall never lies about what is real.

## AC-2

The splice ethos applied to diagrams. The mermaid source is the authored
artifact, byte-preserved by every write path; the verification graph is
extracted one-way and read-only. There is no board→graph→mermaid
round-trip anywhere, so nothing ever normalizes or rewrites the author's
text — pasted diagrams survive bit-for-bit.

## AC-3

Both affordances are mechanical consequences of provenance, not features
with their own state: *before-peek* renders the pinned base (recoverable
from digest-verified inputs at the pinned commit), *reset* discards the
proposal's delta and reproduces that base exactly.

## AC-4

Diagram types with no truth generator — sequence, state, ER, and any
flowchart vocabulary the extractor cannot claim — are the *illustrative*
class: rendered under the pinned mermaid version, tied to their spec, and
badged as deterministically unverifiable. Coverage disclosure keeps the
verified and the illustrative visually and semantically distinct; the
judged sweep (AC-8) remains available to both.

## AC-5

The ruling that makes this a loop rather than a drawing feature:
future-state flowcharts are alignment inputs. At post-build/pre-review
time the alignment verdict gains a diagram-alignment section — for each
accepted future-state flowchart, regenerate truth and diff: an empty
residual means *realized*, proven; residual deltas are *divergences*,
each named with its witness, surfaced for review exactly as deviations
are; illustrative diagrams are listed as unverifiable rather than
omitted, because silence is never a pass.

## AC-6

Acceptance is merge — the total law, applied to diagrams identically.
After acceptance the drift check keeps running: truth that diverges from
an accepted diagram raises *diagram-stale*, computed the way spec-stale
is, so an accepted design picture can never quietly become historical
fiction.

## AC-7

The editor is a drafting-focus surface on the board: a code pane
accepting any mermaid the pinned renderer accepts, a live preview that
fails visible on render errors, and a verification rail showing the
artifact's tier and findings. Interactive editing is
structural-operations-only — add node, connect (click-click or
drag-to-connect), rename inline, delete — each operation a deterministic
edit to the source text. Spatial positioning is refused by design:
layout is renderer-owned, same source, same picture, and the artifact
stores no positions (co-3), so the entire class of position-drift,
collision, and layout-sidecar machinery is structurally impossible here.

## AC-8

The scrutiny-predictor. On demand — never in generation, never in a
gate's deterministic path — a judged sweep reads the proposal against
the corpus (ADRs, constraints, decisions) and yields provenance-stamped
findings ("this new synchronous edge collides with the outbox mandate —
expect scrutiny, or draw the exempts edge now"), each dispositionable by
the human: fix, rebut, or carry. The AI never edits in response to its
own finding, and judged output is never phrased as a completeness
guarantee.

## CO-1

Constitution. The deterministic core of this feature — extraction,
three-way diff, realization detection, drift flags — must be a pure
function of pinned inputs. The judged sweep exists precisely so that the
advisory half has a home that is not the gate.

## CO-2

The upstream law, restated for this surface: flowmap is execed as the
pinned CLI and its graph JSON strict-decoded; hermetic tests consume
canned captures. Verification diffs graphs; it never re-derives what a
graph means from source code.

## CO-3

Renderer-owned layout under a pinned mermaid version is what makes "same
source, same picture" hold — and is the reason spatial editing is
refused rather than deferred (dc-2).

## DC-1

Mermaid text as the authored artifact, one-way extraction for
verification. The rejected alternative — a graph-delta as the canonical
form with mermaid as a pure render — verifies more strongly but forbids
paste-anything authoring and makes the artifact illegible outside the
tool. One-way extraction preserves byte-fidelity and paste freedom; the
extractor failing closed to a lower verification tier (never blocking a
save) keeps authoring unblocked while keeping claims honest.

## DC-2

Structural operations only. Author-positioned nodes would reimport the
entire position-drift and collision class this system just spent a
release solving for cards — and mermaid's renderer-owned layout already
enforces the refusal by construction. Drag connects; it never places.

## DC-3

Coverage is three-valued and disclosed per artifact: full, partial,
illustrative. Applying the constitution's honesty grammar to the checker
itself prevents the one lie this feature could otherwise tell — implying
that everything on the wall was verified because some of it was.

## DC-4

Verification never blocks. Contradictions and staleness surface as
badges and readiness items during authoring and as the diagram-alignment
section at pre-review; the only hard failures are strict-decode failures
of the artifact itself. Gates visible early, enforced late — and the
enforcement is disclosure, not refusal.

## OQ-1

A rename in a diagram diff is ambiguous: remove+add is honest but noisy,
an explicit rename operation yields better witnesses but needs grammar.
The answer shapes contradiction quality and must land before the
extractor's schema freezes.

## OQ-2

flowmap caps hairballs (--max-nodes) and scopes with --root. A proposal
over a large graph probably needs to be a declared, pinned subgraph
scope; whether scope is part of derived_from or its own field is open.

## OQ-3

The ratification surface: diagram status vocabulary
(proposed/accepted/realized/stale semantics), derived_from plus
base-digest fields, and the illustrative marker are 02 amendments in the
round-5.2 pattern. Whether they land as one batch before implementation
or ride the first implementing story is open.

## OQ-4

Illustrative diagrams are "tied to the spec" by ruling; the binding
mechanism — a link type on the spec, or a body-figure convention — is
open, and interacts with oq-3's amendment batch.

---
id: spec/diagram-proposals
kind: spec
title: "Diagram Proposals"
owners: [platform-team]
class: feature
status: closed
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
  - { id: dc-5, text: "the structural diff represents a rename as remove+add — no rename inference (that would reimplement flowmap's graph semantics, co-2, and invite the judgment co-1 forbids); honest witnesses (kept-but-gone plus proposed-new) over concise inference; an author-declared rename is a possible later refinement, out of v1", anchor: dc-5 }
  - { id: dc-6, text: "a proposal declares its scope as a pinned field — the flowmap --root selector it was generated under — and verification regenerates truth at the same scope; an unscoped proposal is verified against the whole graph with the hairball cap disclosed; scope is its own field, orthogonal to derived_from which names the base", anchor: dc-6 }
  - { id: dc-7, text: "the schema surface ratifies as one 02 amendment batch before implementation (the scoping-canvas round-5.4 pattern; co-2 needs a stable artifact contract before the extractor is built): diagram status vocabulary, derived_from plus base-digest, the scope field, and the illustrative marker", anchor: dc-7 }
  - { id: dc-8, text: "illustrative diagrams attach as a body-figure convention — a fenced mermaid block or diagram/ reference in the spec body, prose register, already rendered by the dex — never a new edge type; the closed edge vocabulary stays closed; illustrative is a body figure, a proposal is a first-class artifact", anchor: dc-8 }
stubs:
  - { slug: proposal-artifact, acceptance_criteria: [ac-2, ac-6] }
  - { slug: verification-extractor, acceptance_criteria: [ac-1] }
  - { slug: board-editor, acceptance_criteria: [ac-3, ac-7] }
  - { slug: illustrative-class, acceptance_criteria: [ac-4] }
  - { slug: alignment-section, acceptance_criteria: [ac-5] }
  - { slug: judged-sweep, acceptance_criteria: [ac-8] }
frozen: { at: 2026-07-13, commit: 245eae286b7484f65c633ee5962592bcc1d58d02 }
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

## DC-5

A rename in a diagram diff is ambiguous only if the tool tries to guess
it. flowmap's `--diff` is set-difference over node/edge identity; calling
two differently-named nodes "the same node renamed" is a semantic
judgment flowmap does not make, co-2 forbids verification from
reimplementing, and co-1 keeps the LLM out of. So a rename is two honest
facts: the old node is kept-but-gone (contradicted, witnessed by the
removing commit) and the new node is proposed-new. Noisier than a single
"renamed" chip, but honest noise beats concise inference, and the two
facts render adjacent. An author-DECLARED rename (the human writes the
intent — an authorship act, not an inference) would give better witnesses
without guessing, but is out of v1 scope.

## DC-6

flowmap already scopes with `--root` and caps hairballs with
`--max-nodes` (above the cap it renders an index of entry points, not a
tangle). A proposal therefore carries its scope explicitly — the `--root`
selector it was generated under — pinned, so verification regenerates
truth at the same scope and the diff is comparable. An unscoped proposal
is verified against the whole graph, the cap disclosed. Scope is its own
field, orthogonal to `derived_from`: `derived_from` names the base a
DERIVED proposal forked from, while scope applies to from-scratch
proposals too (they can also be about a subgraph). The selector grammar
joins DC-7's amendment batch.

## DC-7

The schema surface — diagram status vocabulary (proposed / accepted /
realized / stale), `derived_from` plus its base digest, the scope field
(DC-6), and the illustrative marker (DC-8) — ratifies as one 02
amendment batch BEFORE any implementation, the scoping-canvas round-5.4
pattern. co-2's determinism needs a stable artifact contract before the
extractor is built; freezing the schema first and then building against
it is the only honest order.

## DC-8

Illustrative diagrams — the coverage tier with no truth generator
(DC-3) — attach as a body-figure convention: a fenced mermaid block or a
`diagram/` reference in the spec's own body, prose register, which the
dex already renders today. This is deliberately NOT a new edge type: an
"illustrates" edge would extend the closed five-value edge vocabulary
(implements / resolves / exempts / supersedes / depends-on) that VL-003,
the board yarn, and the whole edge grammar depend on staying closed. The
clean line: an illustrative diagram is a body figure (prose), a verified
proposal is a first-class artifact (spec register with status and
`derived_from`) — dc-3's two tiers become two document locations, no new
vocabulary.

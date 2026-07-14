---
id: spec/illustrative-class
kind: spec
title: "Illustrative Class"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-11
problem: { text: "fenced mermaid blocks in spec bodies and generator-less diagram artifacts already render (internal/render's one mermaid seam, the vendored pinned asset, dex and workbench alike) but carry no tier at all: nothing badges them deterministically unverifiable, no coverage disclosure separates them from the verified proposals the feature introduces, so the moment a verified diagram exists the two tiers blend silently — the one lie feature ac-4 exists to kill", anchor: problem }
outcome: { text: "the illustrative tier made legible: body figures and generator-less diagram artifacts render under the same pinned mermaid version on the dex and the board's spec-body surfaces, each wearing a deterministic badge disclosing it as deterministically unverifiable and tied to its spec, with coverage disclosure keeping verified proposals and illustrative figures visually and semantically distinct — never silently blended; the judged sweep remains available to both tiers", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "fenced mermaid blocks in spec bodies render to an SVG under the one vendored pinned mermaid asset on both the dex spec page and the board's spec-body surfaces (corpus page, placard body dialog, reference peek) — the same renderer bytes everywhere, no CDN, no network", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "every illustrative render is wrapped server-side, at the one shared mermaid render seam, in a deterministic badged figure — a visible deterministically-unverifiable badge and a machine-readable tier marker — covering body figures unconditionally and diagram-kind artifact pages when the artifact is not a class: proposal; the proposal render path is never painted with the illustrative badge", evidence: [static, behavioral], anchor: ac-2 }
  - { id: ac-3, text: "coverage disclosure keeps the tiers distinct wherever diagrams render: an illustrative figure wears the illustrative badge, a verified proposal's surfaces carry the extractor-computed tier instead, the two are visually and semantically distinguishable in the DOM, and no surface renders a body-figure diagram unbadged — never silently blended", evidence: [behavioral], anchor: ac-3 }
links:
  - { type: implements, ref: "spec/diagram-proposals#ac-4" }
decisions:
  - { id: dc-1, text: "badge visual grammar: the shared mermaid render seam (internal/render's fenced-mermaid path and RenderMermaidBlock) emits a figure wrapper around the `<pre class=\"mermaid\">` — figure element carrying a data-diagram-tier=\"illustrative\" semantic marker and a figcaption badge chip reading \"illustrative · not deterministically verifiable\" — static deterministic markup emitted once at the seam both the dex and the workbench already consume, styled in the one shared stylesheet; no second markdown implementation, no client-side badge computation", anchor: dc-1 }
  - { id: dc-2, text: "tier is decidable without the extractor: a fenced body figure is illustrative BY LOCATION (feature dc-8's two-locations rule — prose register is the illustrative register) and a diagram-kind artifact without class: proposal is illustrative BY CLASS (no truth generator); no extractor call, no LLM, no clock — the badge is a pure function of the artifact bytes; the full/partial vocabulary applies only to proposals and stays with the verification-extractor story", anchor: dc-2 }
  - { id: dc-3, text: "the diagrams/ reference form stays a link: a body reference to a diagram artifact links to that artifact's own page (corpus /a/diagram/{name}, dex artifact page), which carries the badge itself; no inline transclusion in v1 — the smallest reversible option, keeping the render seam a pure function of the body it renders", anchor: dc-3 }
  - { id: dc-4, text: "spec-tie is containment: a fenced body figure is tied to its spec by living in that spec's body — it renders only inside the owning spec's own pages and sections, never as a free-floating artifact — and the reference form (dc-3) ties through the owning body's link; no new field, no new edge type (feature dc-8: the closed edge vocabulary stays closed)", anchor: dc-4 }
constraints:
  - { id: co-1, text: "no LLM and no clock or randomness anywhere in badge or coverage computation (feature co-1): the markup is a deterministic pure function of the artifact bytes", anchor: co-1 }
  - { id: co-2, text: "one pinned renderer: the vendored mermaid 10.9.1 asset serves every surface; no CDN, no second copy, no network in any test (feature co-3)", anchor: co-2 }
  - { id: co-3, text: "never silently blended (feature ac-4 verbatim): no surface renders a body-figure diagram without its badge; illustrative stays a body-figure convention in the prose register, never a first-class artifact or a new edge type (feature dc-8); the judged sweep (feature ac-8) remains available to both tiers — the judged-sweep story owns that surface", anchor: co-3 }
frozen: { at: 2026-07-14, commit: 941e68b442168a6c9c8e6832c7f3b6929b9cbe9b, stub_matched: true }
---
# Illustrative Class

## Problem

The rendering half of the illustrative tier already exists: a fenced
mermaid block in a spec body becomes `<pre class="mermaid">` through
`internal/render`'s one mermaid seam, diagram-kind artifacts render the
same way on their own pages, and both the dex and the workbench serve
the same vendored pinned mermaid asset. What does not exist is the
tier itself. Nothing badges these renders as deterministically
unverifiable; no coverage disclosure separates them from the verified
proposals the diagram-proposals feature introduces; nothing in the DOM
distinguishes "this picture was checked against truth" from "this
picture is an illustration someone drew." The moment the first verified
proposal renders, the two tiers blend silently — a reader has no way to
tell which diagrams earned their claims — which is exactly the one lie
feature ac-4 exists to kill: implying everything on the wall was
verified because some of it was.

## Outcome

The illustrative tier made legible. Fenced mermaid blocks in spec
bodies (and `diagrams/` references, through their targets' own pages)
render under the same pinned mermaid version on the dex spec page and
the board's spec-body surfaces, each wearing a deterministic badge
disclosing it as deterministically unverifiable, tied to its spec.
Coverage disclosure keeps verified proposals and illustrative figures
visually and semantically distinct — never silently blended. The
judged sweep (feature ac-8) remains available to both tiers.

## AC-1

Same pinned renderer everywhere. A fenced mermaid block in a spec body
renders to an SVG under the one vendored pinned mermaid asset — on the
dex spec page and on the board's spec-body surfaces alike: the corpus
artifact page, the placard body dialog (the attribute-body seam), and
the reference peek fragment. It is the same renderer bytes on every
surface (the dex-embedded asset the workbench re-serves), so "same
source, same picture" holds across the whole system, hermetically: no
CDN, no network, in production or in any test. Behavioral register:
Playwright e2e opens a fixture spec carrying a fenced mermaid block and
asserts the rendered SVG on the dex page and on the board surfaces,
with the suite running network-free.

## AC-2

The badge, emitted at the seam. Every illustrative render is wrapped
server-side — at the one shared mermaid render seam both surfaces
already consume — in a deterministic badged figure: a visible
deterministically-unverifiable badge chip and a machine-readable tier
marker (dc-1). It covers body figures unconditionally (illustrative by
location, dc-2) and diagram-kind artifact pages when the artifact is
not a `class: proposal` (illustrative by class). The proposal render
path is never painted with the illustrative badge — a proposal's tier
is the extractor's to compute (feature dc-3), and a false "illustrative"
on a verified proposal would be the same blending lie inverted. Static
register: table-driven unit tests on the render seam prove the badge
markup on the fenced path and the non-proposal diagram path, its
byte-determinism across runs, and the negative case — the proposal
path emits no illustrative badge. Behavioral register: e2e sees the
badge on the dex spec page, the dex diagram page, and the board's
spec-body surfaces.

## AC-3

Coverage disclosure — never silently blended. Wherever diagrams render,
the tiers stay distinct: an illustrative figure wears the illustrative
badge; a verified proposal's surfaces carry the extractor-computed
verification tier instead (rendered by the board-editor rail and the
proposal's own pages; computed by the verification-extractor story —
consumed here as vocabulary, never recomputed); and the two are
distinguishable both visually (the badge chip) and semantically (the
DOM tier marker, dc-1). No surface renders a body-figure diagram
unbadged. Behavioral register: e2e renders a fixture store containing
both an illustrative body figure and a proposal artifact and asserts
the two carry different, non-empty tier markers — and that no
`<pre class="mermaid">` from body prose appears outside a badged
figure.

## DC-1

Badge visual grammar. The shared mermaid render seam —
`internal/render`'s fenced-mermaid path and `RenderMermaidBlock` — emits
a figure wrapper around the `<pre class="mermaid">`: a figure element
carrying `data-diagram-tier="illustrative"` as the semantic marker and
a figcaption badge chip reading "illustrative · not deterministically
verifiable". Static, deterministic markup, emitted once at the seam the
dex, the corpus page, the placard body dialog, and the peek fragment
already all consume — so every surface inherits the badge without a
second markdown implementation (the CLAUDE.md shared-seam rule), and no
client-side JavaScript computes any part of the disclosure. Styling
lives in the one shared stylesheet both surfaces already serve.

## DC-2

Tier without the extractor. The illustrative badge is decidable from
the artifact bytes alone: a fenced body figure is illustrative BY
LOCATION — feature dc-8's two-locations rule makes the prose register
the illustrative register, by definition — and a diagram-kind artifact
without `class: proposal` is illustrative BY CLASS (no truth generator
exists for it, feature dc-3). No extractor call, no LLM, no clock: the
badge is a pure function of its input, so it can never be stale, flaky,
or blocked on the extractor story. The full/partial coverage vocabulary
applies only to proposals and stays with the verification-extractor
story; this story only guarantees the illustrative side of the
distinction and the absence of blending.

## DC-3

The `diagrams/` reference form stays a link. A body reference to a
diagram artifact links to that artifact's own page — the corpus page at
`/a/diagram/{name}` and the dex artifact page — which carries the badge
itself (ac-2). No inline transclusion in v1: transcluding would demand
resolution inside the markdown renderer (making the render seam impure
and store-dependent) for no disclosure gain, since the tier is disclosed
at the target. The smallest reversible option; inline transclusion can
be layered on later without changing the badge grammar.

## DC-4

Spec-tie is containment. A fenced body figure is tied to its spec by
living in that spec's body: it renders only inside the owning spec's
own pages and sections — the dex spec page, that spec's corpus page,
its placard dialogs and peeks — never as a free-floating artifact. The
reference form ties through the owning body's link (dc-3). No new
frontmatter field and no new edge type carries the tie: feature dc-8's
ruling that the closed five-value edge vocabulary stays closed is
inherited whole, and this story adds no vocabulary anywhere.

## CO-1

No LLM, no clock, no randomness anywhere in badge or coverage
computation (feature co-1). The badge markup is a deterministic pure
function of the artifact bytes — same input, same bytes out, every
run.

## CO-2

One pinned renderer. The vendored mermaid 10.9.1 asset (the dex's
embedded copy, re-served by the workbench) serves every surface this
story touches; no CDN, no second vendored copy, no version skew between
surfaces, and no network in any test (feature co-3) — the e2e suite
proves rendering with outbound network unavailable.

## CO-3

Never silently blended — feature ac-4's own words, carried as this
story's binding posture. No surface renders a body-figure diagram
without its badge. Illustrative stays a body-figure convention in the
prose register, never a first-class artifact and never a new edge type
(feature dc-8). And the tier is not a quality ghetto: the judged sweep
(feature ac-8) remains available to both tiers — an illustrative figure
can be swept for scrutiny exactly like a verified proposal — with that
surface owned by the judged-sweep story, stated here so the boundary is
explicit.

---
id: obligation/illustrative-class--ac-2--static
kind: obligation
title: "Unit tests pin the badge markup at the shared render seam, its determinism, and the proposal-path negative"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/illustrative-class" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Unit tests pin the badge markup at the shared render seam, its determinism, and the proposal-path negative

The static evidence must show the badge implemented at internal/render's one
mermaid seam (the fenced-mermaid path and RenderMermaidBlock's callers — not
duplicated per surface) with table-driven unit tests proving: (1) a fenced
mermaid block in markdown renders wrapped in the dc-1 figure — the
data-diagram-tier="illustrative" marker and the visible figcaption badge chip
— with the diagram source still HTML-escaped inside `<pre class="mermaid">`;
(2) a diagram-kind artifact body WITHOUT class: proposal renders through the
same badged wrapper; (3) the negative case: the class: proposal render path
emits NO illustrative badge and no illustrative tier marker; (4) determinism:
the same input yields byte-identical markup across repeated calls, with no
clock, randomness, or store lookup in the render function's inputs. Evidence
that badges in page templates per surface instead of at the shared seam, or
that omits the proposal-path negative, does not satisfy this obligation.

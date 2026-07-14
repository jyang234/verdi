---
id: obligation/illustrative-class--ac-1--behavioral
kind: obligation
title: "e2e sees a body-figure mermaid render to SVG under the vendored asset on the dex page and every board spec-body surface"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/illustrative-class" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# e2e sees a body-figure mermaid render to SVG under the vendored asset on the dex page and every board spec-body surface

The behavioral evidence must show Playwright e2e coverage over a fixture spec
whose body carries a fenced mermaid block, asserting a rendered `<svg>` (not
just the `<pre class="mermaid">` placeholder) on EACH surface the AC names:
the dex spec page, the workbench corpus artifact page (/a/spec/{name}), the
board placard body dialog (the attribute-body render), and the board
reference-peek fragment. The test must prove the renderer is the vendored
asset: the page loads /assets/mermaid.min.js (the dex-embedded copy) and the
suite passes with no external network available — a run that fetches any
diagram renderer from a remote origin fails. Evidence covering only the dex
page, or asserting the pre element without the rendered SVG, does not
satisfy this obligation.

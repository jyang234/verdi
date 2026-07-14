---
id: obligation/illustrative-class--ac-2--behavioral
kind: obligation
title: "e2e sees the deterministically-unverifiable badge on the dex spec page, the dex diagram page, and the board's spec-body surfaces"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/illustrative-class" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# e2e sees the deterministically-unverifiable badge on the dex spec page, the dex diagram page, and the board's spec-body surfaces

The behavioral evidence must show Playwright e2e coverage asserting the
visible illustrative badge (the dc-1 figcaption chip, with its
data-diagram-tier="illustrative" marker present in the DOM) on: (1) the dex
spec page rendering a fixture spec's fenced mermaid body figure; (2) the dex
artifact page of a fixture diagram-kind artifact that carries no
class: proposal; and (3) the board's spec-body surfaces rendering the same
fixture spec (corpus page, and at least one of placard body dialog / peek
fragment — whichever renders the body section carrying the figure). The
badge must sit on the SAME figure that contains the rendered diagram, not
elsewhere on the page. Evidence asserting the badge on one surface only, or
asserting the marker attribute without the visible chip, does not satisfy
this obligation.

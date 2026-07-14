---
id: obligation/illustrative-class--ac-3--behavioral
kind: obligation
title: "e2e proves the two tiers render distinct and no body-figure diagram appears unbadged"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/illustrative-class" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# e2e proves the two tiers render distinct and no body-figure diagram appears unbadged

The behavioral evidence must show Playwright e2e coverage over a fixture
store containing BOTH an illustrative body figure and a class: proposal
diagram artifact, proving the non-blending claim: (1) the illustrative
figure's DOM carries data-diagram-tier="illustrative" and its visible badge;
(2) the proposal's rendered surface carries a different, non-empty tier
marker (the extractor-computed vocabulary, from a canned report — this suite
never runs flowmap) and does NOT carry the illustrative badge; (3) a
distinctness assertion that the two markers differ, so a future regression
that collapses the tiers fails the test rather than passing silently; and
(4) a sweep assertion that every `<pre class="mermaid">` originating from
body prose on the exercised pages sits inside a badged figure — no unbadged
body-figure diagram anywhere. Evidence exercising only one tier, or
asserting distinctness by visual screenshot alone without the semantic DOM
markers, does not satisfy this obligation.

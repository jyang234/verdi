---
id: obligation/evidence-slot--ac-1--behavioral
kind: obligation
title: "Declared kinds render; a folded record fills its slot"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/evidence-slot" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Declared kinds render; a folded record fills its slot

The behavioral evidence must render fixture story walls proving (a) every
kind an AC declares appears as a slot entry, and no undeclared kind does;
(b) a fixture derived-tree record of a declared kind flips exactly that
kind's slot from empty to held while sibling kinds stay empty; (c) an
attestation file on disk fills the attestation kind's slot; and (d) a wall
with no derived tree renders every declared kind as an empty slot without
error — the ordinary authoring state, not a failure path.

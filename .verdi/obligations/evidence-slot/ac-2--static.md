---
id: obligation/evidence-slot--ac-2--static
kind: obligation
title: "The empty-slot badge rides the one compute layer with a full record"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/evidence-slot" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# The empty-slot badge rides the one compute layer with a full record

The static evidence must show the empty-slot badge constructed in the
badge compute layer's single attachment point (the loadBoard I/O
enrichment tier) using the canonical derivation record schema — source
fold:empty-slot, inputs naming the spec digest and the derived-tree path
probed with digests of files read, records disclosing per-kind findings —
and must show no write handler, gate, or lint rule reading slot or badge
state (disclosure, never a consumed input).

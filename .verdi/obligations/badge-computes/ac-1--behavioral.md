---
id: obligation/badge-computes--ac-1--behavioral
kind: obligation
title: "Page, fragment, and get_board carry the same badges"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/badge-computes" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Page, fragment, and get_board carry the same badges

The behavioral evidence must drive a fixture spec with at least one firing
compute and prove the SAME badge set appears on GET /board/spec/{name}, on
the post-mutation fragment (GET /board/spec/{name}/fragment), and in
LoadProjection's returned projection (the get_board path) — asserting on
badge source ids, not just badge counts, so a lookalike-but-different
enrichment on one surface fails the test.

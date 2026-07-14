---
id: obligation/obligation-wall--ac-1--static
kind: obligation
title: "One loader reads obligations by (spec-name, ac-id)"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/obligation-wall" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# One loader reads obligations by (spec-name, ac-id)

The static evidence must show internal/evidence.Obligations declared — the single reader both verdi matrix and the board card consume (dc-1, not two readers) — loading .verdi/obligations/<spec>/<ac>--*.md keyed by for_kind, treating a missing file as ordinary absence and surfacing a present-but-broken obligation as an operational error.

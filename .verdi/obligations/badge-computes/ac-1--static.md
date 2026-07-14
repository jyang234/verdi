---
id: obligation/badge-computes--ac-1--static
kind: obligation
title: "Badges attach in loadBoard's I/O tier; the projector stays pure"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/badge-computes" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Badges attach in loadBoard's I/O tier; the projector stays pure

The static evidence must show the badge compute layer called from loadBoard
(internal/workbench/boardspec.go) after buildProjection — the
attachObligations posture — with buildProjection itself untouched by any
badge input: no lint, decisionsweep, or evidence import enters the pure
projector. It must also show that LoadProjection (the get_board entrypoint)
reaches the same attachment through loadBoard, so no surface gets a
badge-free projection by construction.

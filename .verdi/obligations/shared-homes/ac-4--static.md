---
id: obligation/shared-homes--ac-4--static
kind: obligation
title: "One classification table, both walks, divergence healed"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/shared-homes" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# One classification table, both walks, divergence healed

The static evidence must show the path-classification table living in
internal/artifact, both lint's and index's walks consuming it (walks still
separate), index's decodeEntry carrying the reaffirmation arm, and lint's
stale mirrors-index comment corrected.

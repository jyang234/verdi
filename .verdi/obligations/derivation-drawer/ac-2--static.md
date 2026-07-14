---
id: obligation/derivation-drawer--ac-2--static
kind: obligation
title: "One server-side drawer renderer over the record schema, no recomputation"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/derivation-drawer" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# One server-side drawer renderer over the record schema, no recomputation

The static evidence must show exactly one drawer-body renderer in
internal/workbench, taking the canonical derivation record as its sole
data input — no call from the drawer render path back into lint,
decisionsweep, or evidence recomputation — and show that
assets/boardspec.js contains no derivation-data templating: the client
only toggles/positions the server-rendered hidden drawer element (the
writePlacardFull idiom). Both the full page and the fragment must reach
the same renderer.

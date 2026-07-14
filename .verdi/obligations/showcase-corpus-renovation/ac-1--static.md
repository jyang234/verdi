---
id: obligation/showcase-corpus-renovation--ac-1--static
kind: obligation
title: "The relocated, renamed tree exists and every artifact is recorded against the vetting bar"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/showcase-corpus-renovation" }
frozen: { at: 2026-07-14, commit: e367364ef3d001864a3f453d965a4109d25e26f6 }
---
# The relocated, renamed tree exists and every artifact is recorded against the vetting bar

The static evidence must show that `examples/showcase` exists as a single
committed store at the repo root — the merge of the former `testdata/corpus`
and `testdata/dexoverlay` trees, with `testdata/corpus` and
`testdata/dexoverlay` no longer present — and that its `layers.txt` reflects
the merged, one-construction-path layering (no separate overlay-copy step
left in the e2e harness). It must further show that
`verdi/docs/showcase-vetting.md` exists and, walked against a listing of
every file under `examples/showcase`, has a row for every one of them
recording all three vetting-bar columns (lint-clean, editorially exemplary,
narrative-coherent + depth-justified) or an explicit, reasoned cut — zero
files present in the tree without a corresponding row. Renamed artifacts
(`accepted-pending-build` → `escrow-autopay`, `new-feature-x` removed) must
show zero remaining references to the old names anywhere in the committed
tree outside of history.

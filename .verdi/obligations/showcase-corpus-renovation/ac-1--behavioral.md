---
id: obligation/showcase-corpus-renovation--ac-1--behavioral
kind: obligation
title: "A provisioned checkout of the renovated tree lints clean"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/showcase-corpus-renovation" }
frozen: { at: 2026-07-14, commit: e367364ef3d001864a3f453d965a4109d25e26f6 }
---
# A provisioned checkout of the renovated tree lints clean

The behavioral evidence must show `verdi lint` run against a scratch store
provisioned exactly as the e2e harness provisions one — a temp directory
with `examples/showcase/.verdi` copied in, `git init` and an initial commit,
and `mutable/`/`derived/` materialized into `.verdi/data/` — exits 0 over
the fully renovated tree. `SeverityDisclosure` (VL-017) lines are permitted
in the output; any other non-zero-exit finding is not. The run must be
reproduced against the actual relocated-and-renovated `examples/showcase`
tree, not against `testdata/corpus` or a partial migration, and must
demonstrate the merged construction (no dexoverlay-copy step, single
`layers.txt` history) actually resolves and lints clean end to end.

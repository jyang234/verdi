---
id: obligation/case-file-flags--ac-3--static
kind: obligation
title: "No client viewport measurement anywhere in the size-smell path"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/case-file-flags" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# No client viewport measurement anywhere in the size-smell path

The static evidence must show the size-smell compute and its drawer
content are produced entirely server-side from pinned inputs, and that no
client script measures or injects a viewport dimension into the badge or
its drawer — no window.innerHeight (or equivalent) feeding badge state in
assets/boardspec.js, and no drawer field whose value originates outside
the derivation record. The drawer cites the reference constant by name,
disclosed as a constant, never as a measurement.

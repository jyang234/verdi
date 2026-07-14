---
id: obligation/shared-homes--ac-2--behavioral
kind: obligation
title: "Digest strings byte-identical to the golden"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/shared-homes" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# Digest strings byte-identical to the golden

The behavioral evidence must show the pinned golden digest string for a
committed fixture value unchanged across the extraction, and every former
copy's caller suite green — the proof no digest anywhere in the store
changed value.

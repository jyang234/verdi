---
id: obligation/stale-decline--ac-1--static
kind: obligation
title: "VL-019 overlay: obligation verifies a FEATURE AC"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/stale-decline#ac-1" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-019 overlay: obligation verifies a FEATURE AC

`spec/stale-decline` is `class: feature` in the golden corpus. Obligations
attach to STORY acceptance criteria only (03 §The feature fold) — VL-019
must refuse this obligation's `verifies` edge, naming the FEATURE ac it
wrongly targets.

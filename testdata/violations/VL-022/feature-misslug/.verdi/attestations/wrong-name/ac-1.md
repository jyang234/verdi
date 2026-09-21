---
id: attestation/wrong-name--ac-1
kind: attestation
title: "VL-022 overlay: feature outcome attestation with a mis-slugged directory"
owners: [platform-team]
links:
  - { type: verifies, ref: "spec/vl-022-feature" }
frozen: { at: 2026-07-16, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-022 overlay: feature outcome attestation with a mis-slugged directory

This attestation's own directory/id segment is "wrong-name", but its
`verifies` edge resolves to spec/vl-022-feature, whose own name is
"vl-022-feature" — ac-1 declares the attestation evidence kind, so the
feature fold's own path (attestations/vl-022-feature/ac-1.md) is what
gets read; this file sits at the wrong path. VL-022 must refuse this,
naming the disagreeing values (R-RR2-2).

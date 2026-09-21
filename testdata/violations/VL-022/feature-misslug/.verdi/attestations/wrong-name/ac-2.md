---
id: attestation/wrong-name--ac-2
kind: attestation
title: "VL-022 overlay: feature outcome attestation whose AC declares no attestation kind"
owners: [platform-team]
links:
  - { type: verifies, ref: "spec/vl-022-feature" }
frozen: { at: 2026-07-16, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-022 overlay: feature outcome attestation whose AC declares no attestation kind

This attestation's own directory/id segment is also "wrong-name" (the
same mis-slug ac-1.md carries), but ac-2 declares only `static` among its
expected evidence kinds — it is outside every consumer the fold's
attestation path serves (R-RR2-2), so VL-022 must SKIP it regardless of
the slug: the kind-declaration boundary is checked before, and
independent of, slug agreement.

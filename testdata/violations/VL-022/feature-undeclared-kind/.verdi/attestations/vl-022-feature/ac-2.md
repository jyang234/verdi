---
id: attestation/vl-022-feature--ac-2
kind: attestation
title: "VL-022 overlay: correctly-slugged feature attestation whose AC declares no attestation kind"
owners: [platform-team]
links:
  - { type: verifies, ref: "spec/vl-022-feature" }
frozen: { at: 2026-07-16, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-022 overlay: correctly-slugged feature attestation whose AC declares no attestation kind

This attestation's own directory/id segment ("vl-022-feature") agrees
with its `verifies` target's own name — VL-011's id/path agreement and
R-RR2-2's slug check would both pass — but ac-2 declares only `static`
among its expected evidence kinds: attestation is outside every consumer
of this AC, so VL-022 must SKIP it. Isolated from the slug fixture in
feature-misslug/: here the slug is deliberately correct, proving the
undeclared-kind skip is about kind declaration, not slug correctness.

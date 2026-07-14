---
id: obligation/obligation-gate--ac-2--behavioral
kind: obligation
title: "A test proves feature-exempt scoping and draft-tolerant timing"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-gate" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# A test proves feature-exempt scoping and draft-tolerant timing

The behavioral evidence must show a Go test proving a FEATURE AC declaring kinds requires no obligation (the lint resolves class and only gates STORY specs) and that a DRAFT story is tolerated (the VL-006 activation timing) — so authoring on the wall is never blocked; only accept/activation is gated.

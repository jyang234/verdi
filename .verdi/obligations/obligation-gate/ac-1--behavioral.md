---
id: obligation/obligation-gate--ac-1--behavioral
kind: obligation
title: "A test proves the gate refuses a declared kind with no obligation and passes with one"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-gate" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# A test proves the gate refuses a declared kind with no obligation and passes with one

The behavioral evidence must show a Go test (TestVL020_*) proving VL-020 refuses a story AC that declares an evidence kind with no matching obligation on disk — naming the missing (ac, kind) — and passes once that obligation exists; including the two-declared-one-authored case, which must refuse only the missing kind.

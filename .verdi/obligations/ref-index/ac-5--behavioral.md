---
id: obligation/ref-index--ac-5--behavioral
kind: obligation
title: "The serving checkout's HEAD and working tree are byte-identical before and after a ComputeIndex run"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/ref-index" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# The serving checkout's HEAD and working tree are byte-identical before and after a ComputeIndex run

The behavioral evidence must show a Go test that, against a fixture repository carrying multiple `design/*` branches, records the checkout's `git symbolic-ref HEAD` (or its resolved commit, for a detached checkout) and a content hash of its working tree before calling `ComputeIndex`, runs `ComputeIndex`, then asserts both are byte-identical afterward — proving no checkout switch occurred as a side effect of enumerating other branches' content via ref-scoped reads.

---
id: obligation/shared-homes--ac-1--behavioral
kind: obligation
title: "Atomic writes proven happy and refused loud"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/shared-homes" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# Atomic writes proven happy and refused loud

The behavioral evidence must show atomicfile's table tests (happy round
trip, unwritable destination refused with a wrapped error) and the existing
boardio/boardlayout suites green over the converted call sites.

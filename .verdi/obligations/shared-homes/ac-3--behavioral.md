---
id: obligation/shared-homes--ac-3--behavioral
kind: obligation
title: "Quoting byte-identical across representative strings"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/shared-homes" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# Quoting byte-identical across representative strings

The behavioral evidence must show the quote table — plain, embedded
quotes, newlines, unicode — producing byte-identical output to the deleted
copies, and the three consumers' rendering suites green.

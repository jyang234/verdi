---
id: obligation/obligation-wall--ac-1--behavioral
kind: obligation
title: "A test proves matrix renders each kind's obligation or a disclosed marker"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-wall" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# A test proves matrix renders each kind's obligation or a disclosed marker

The behavioral evidence must show a Go/CLI test proving verdi matrix's story-AC output renders, for each declared evidence kind, that kind's obligation title, and a disclosed '(no obligation)' marker for a declared kind without one — so what an AC demands is read from matrix, never recovered from verdi.bindings.yaml.

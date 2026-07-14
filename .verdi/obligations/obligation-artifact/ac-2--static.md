---
id: obligation/obligation-artifact--ac-2--static
kind: obligation
title: "VL-019 is a registered, ratified rule"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/obligation-artifact" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# VL-019 is a registered, ratified rule

The static evidence must show VL-019 registered in the lint engine (internal/lint) and recorded in 02 §Lint rules, resolving its target's class through storyresolve.LoadSpec — the same class resolver accept.go's supersession uses — rather than a bespoke check.

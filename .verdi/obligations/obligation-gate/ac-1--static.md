---
id: obligation/obligation-gate--ac-1--static
kind: obligation
title: "VL-020 is a registered activation lint"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/obligation-gate" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# VL-020 is a registered activation lint

The static evidence must show VL-020 declared in internal/lint/vl020.go, registered in the lint engine's rule set, and counted in the rule inventory — the obligation-shaped sibling of VL-006, keyed on the spec-name obligation path .verdi/obligations/<spec>/<ac>--<kind>.md.

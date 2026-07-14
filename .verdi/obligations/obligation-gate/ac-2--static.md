---
id: obligation/obligation-gate--ac-2--static
kind: obligation
title: "The evidence record and fold are provably unchanged"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/obligation-gate" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# The evidence record and fold are provably unchanged

The static evidence must show that verdi.evidence/v1 (internal/artifact/evidence.go) gained NO obligation_id field and the story fold's (AC, kind) match logic is untouched — the whole gate lives at activation (VL-020), honoring the feature's oq-1 resolution that verdi cannot make a record it does not produce carry a parsable obligation reference.

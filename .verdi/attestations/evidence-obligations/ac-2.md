---
id: attestation/evidence-obligations--ac-2
kind: attestation
title: "AC-2 attested: obligations gate at activation, record and fold unchanged"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/evidence-obligations }
frozen: { at: 2026-07-14, commit: 658e6ce3bb078692da8c638e1e5ace8d2936e127 }
---
# AC-2 attested: obligations gate at activation, record and fold unchanged

Operator attests (2026-07-14): VL-020 refuses a story AC that declares an evidence kind with no matching obligation, naming the missing (ac, kind) — proven, and proven live: the feature's own three stories were removed from VL-020's baseline and pass only because their obligations now exist. The verdi.evidence/v1 record and the fold are unchanged (no obligation_id) — the gate is at activation, honoring oq-1's resolution. AC-2 satisfied.

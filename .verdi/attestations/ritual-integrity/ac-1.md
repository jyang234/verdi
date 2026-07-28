---
id: attestation/ritual-integrity--ac-1
kind: attestation
title: "AC-1 attested: judge-backed verbs are waitable and pollable — no caller parked on an open-ended gate through an entire closure phase"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/ritual-integrity }
frozen: { at: 2026-07-21, commit: bbb160f03c5344bbe921dd585b24b7851b6ce007 }
---
# AC-1 outcome attestation

Stand-in operator attests (Phase 2, 2026-07-21): the X-8 failure mode is
dead in practice. Every judge exchange of this feature's own closure walk
— five real aligns, 1m46s to 5m44s — ran under `--wait` with the report
path on stdout's first line; no agent or human parked, none needed a
rescue nudge (five occurrences required them in Phase 1). The extend-only
guard's first field use refused the operator's own too-short bound
(--wait=560 vs the 600s ceiling) — the contract catching its author
before anyone else. close's freeze-align inherits the identical engine
contract, proven at verb level and interlocked with closure condition 4.

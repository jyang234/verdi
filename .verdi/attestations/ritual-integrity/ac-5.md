---
id: attestation/ritual-integrity--ac-5
kind: attestation
title: "AC-5 attested: guide capability claims are machine-checked in every verify — hand-transcription drift is caught, not trusted"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/ritual-integrity }
frozen: { at: 2026-07-21, commit: bbb160f03c5344bbe921dd585b24b7851b6ce007 }
---
# AC-5 outcome attestation

Stand-in operator attests (2026-07-21): the claims gate has run inside
make verify since #184 and earned its thesis on its own first report —
the judge caught the manifest claiming MORE than the guide (four
mixed-status rows silently flattened) within one build of the gate's
birth, and the PASS-coupling check rejected an isolation-fragile witness
during its own fix wave. Rows are atomic with pinned (id, status) pairs
for the adjudicated sections; witnesses are anchored and pass-coupled;
downgrades demand fresh citations; the gate discloses its inventory-only
scope honestly until the in-repo guide enables completeness (Task 18).

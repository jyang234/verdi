---
schema: verdi.policy/v1
id: policy/unsealed-provenance
kind: policy
title: "Unsealed-provenance exemption authority"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
payloads:
  unsealed-provenance:
    permitted: true
    cap: 1
    inventory:
      - id: vatc-machine-projections
        story: spec/vatc-machine-projections
        admitted_by: "unsealed-provenance exemption ratification (SI-244)"
    cutoff:
      commit: 0123456789abcdef0123456789abcdef01234567
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Permits one pilot unsealed-provenance exemption for the inventoried
story and fixes the append-only cutoff commit.

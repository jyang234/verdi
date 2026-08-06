---
schema: verdi.policy/v1
id: policy/fixture-policy
kind: policy
title: "Fixture policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
  - id: verify-required
    family: action
    operator: required-values
    subject: make-verify
    values: [clean-exit]
    scope: {phases: [build], environments: [], paths: [], refs: []}
    overridable: false
instructions:
  - "Run make verify before claiming completion."
payloads: {}
---
Fixture policy proving policy artifacts under `.verdi/policy/` lint clean.

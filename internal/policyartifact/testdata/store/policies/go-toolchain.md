---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
  - id: go-version
    family: configuration
    operator: allowed-values
    subject: go-version
    values: ["1.25", "1.24"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: verify-required
    family: action
    operator: required-values
    subject: make-verify
    values: [clean-exit]
    scope: {phases: [build], environments: [], paths: [], refs: []}
    overridable: false
instructions:
  - "Run make verify before claiming completion."
  - "Never claim a gate passed without command output."
payloads:
  design_assistance: {mode: proposal-only, layout: false}
---
Pin the Go toolchain and the verification gate so builds are
reproducible and completion claims stay proven.

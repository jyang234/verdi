---
schema: verdi.policy-exemption/v1
id: policy-exemption/legacy-service-go
kind: policy-exemption
title: "Legacy service stays on Go 1.23"
owners: [service-team]
scope: {phases: [], environments: [], paths: ["services/legacy/"], refs: []}
witnesses:
  - policy: policy/go-toolchain
    claim: go-version
    claim_digest: "sha256:939dc350ca2599363d9b5b89ecf681061f35081ed39025e785696d8f92c23261"
compensating_controls:
  - "Weekly CVE review of the pinned toolchain."
approvals:
  - role: policy-owner
    principal: principal/github-org/YWxpY2U
expiry: "2026-12-31"
template: {identity: "embedded:policy-exemption.md", digest: "sha256:cf3977e08d4259c963e3b7ca9b974e2334d35548ac155b0e972bc7441733dad9"}
---
The legacy service cannot move until its cgo dependency updates; the
departure is bounded by an expiry and a weekly review control.

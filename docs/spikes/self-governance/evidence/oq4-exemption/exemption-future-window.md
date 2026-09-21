---
schema: verdi.policy-exemption/v1
id: policy-exemption/golangci-lint-standard-set
kind: policy-exemption
title: "golangci-lint runs the standard set only"
owners: [spike-operator]
scope: {phases: [], environments: [], paths: [], refs: []}
witnesses:
  - policy: policy/starter
    claim: golangci-lint-standard-set
    claim_digest: "sha256:9f1d0fb5b3aa53e6f1543730bd1f856491ccc4c602e84bee50adcc75583f1e44"
compensating_controls:
  - "gofmt -l . and go vet ./... still run in every CI job."
approvals:
  - role: policy-owner
    principal: principal/github-org/c3Bpa2Utb3BlcmF0b3I
expiry: "2027-06-30"
template: {identity: "embedded:policy-exemption.md", digest: "sha256:cf3977e08d4259c963e3b7ca9b974e2334d35548ac155b0e972bc7441733dad9"}
---
spec/self-governance oq-4 scratch fixture: the real golangci-lint parity
exception (process-audit PA-004/PA-016) modeled as a bounded departure
from the golangci-lint-standard-set claim, with a review window (expiry)
with a future window (2027-06-30).

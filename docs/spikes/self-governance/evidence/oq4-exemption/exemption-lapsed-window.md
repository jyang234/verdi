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
    principal: principal/local/c3Bpa2Utb3BlcmF0b3JAZXhhbXBsZS5pbnZhbGlk
expiry: "2026-01-01"
template: {identity: "embedded:policy-exemption.md", digest: "sha256:cf3977e08d4259c963e3b7ca9b974e2334d35548ac155b0e972bc7441733dad9"}
---
spec/self-governance oq-4 scratch fixture: the same claim, same
compensating control, same approval as exemption-future-window.md in
this directory — the ONLY field this variant changes is expiry, now
lapsed as of 2026-09-20 (today). Every other input held constant, so
fix round 1's paired conflict-path re-run (review finding F2) isolates
the review window as the one variable behind the two reports' different
`resolution.bound`/`removed_claims` outcome.

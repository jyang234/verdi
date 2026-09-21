---
schema: verdi.policy/v1
id: policy/starter
kind: policy
title: "Starter policy"
owners: [spike-operator]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
  - id: golangci-lint-standard-set
    family: configuration
    operator: equals
    subject: golangci-lint-linter-set
    values: ["standard"]
    scope: {phases: [build], environments: [], paths: [], refs: []}
    overridable: false
instructions: []
payloads:
  design_assistance: {mode: draft-write, layout: false}
template: {identity: "embedded:policy-starter.md", digest: "sha256:bcdc72560213490005b68534b2edc108e2e4717ef609a6045a3fca164152fad6"}
---
Starter policy: one real rule and nothing else. The design_assistance
payload sets mode draft-write — what delegated agents
may do on a design branch. No constraint claims, no instruction lines, and
no other payloads are declared; add them through the project's own review
process. Acceptance of this policy is the owner's merge to the default
branch.

---
id: spec/loan-refi-2023
kind: spec
class: feature
title: "Loan refinance rollout 2023 (fixture, closed)"
status: closed
owners: [platform-team]
story: jira:LOAN-2023
impacts: [loansvc]
acceptance_criteria:
  - { id: ac-1, text: "refinance rate applied correctly", evidence: [static, behavioral] }
frozen: { at: 2026-06-20, commit: 4e5ef0b6b00f23c9faf7a9e4857255b7be5bea03 }
---
# Loan refinance rollout 2023

LoanServ's first rollout of an automated refinance rate check: before
this feature, an underwriter re-keyed the published rate table by hand
against every refinance quote, a step that was both slow and the source
of a recurring class of pricing errors when the table changed mid-week.
`ac-1` closed that gap by verifying the applied rate against the
published table before rollout instead of after a borrower had already
signed.

Closed and archived (jira:LOAN-2023): the full quartet below — this spec,
`board.json`, `rollup.json`, `deviation-report.md` — is frozen at the same
commit, the closure snapshot 02 §Kind registry's `... → closed(archive)`
transition produces. `spec/refi-rate-check-2024`, the following year's
round-four successor to this same rate-check problem, carries an
`implements` edge into this spec's `ac-1`.

---
id: spec/stale-decline
kind: spec
class: feature
title: "Stale decline handling (fixture)"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1482
links:
  - { type: implements, ref: adr/0002-outbox-events }
  - { type: story, ref: jira:LOAN-1482 }
  - { type: impacts, ref: svc/loansvc/boundary-contract }
impacts: [loansvc, notification-svc]
context:
  - adr/0002-outbox-events@f80b677cac43645416a4a1441a258234e2ef763d
declares:
  boundaries:
    - { from: loansvc, to: notification-svc, via: events }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds for the retry path", evidence: [static] }
  - { id: ac-2, text: "static and behavioral: charge API retried on stale decline", evidence: [static, behavioral] }
  - { id: ac-3, text: "behavioral: golden flow for partial refunds", evidence: [behavioral] }
  - { id: ac-4, text: "runtime: post-deploy decline-rate check", evidence: [runtime] }
dispositions:
  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: incorporated, where: "#design-notes" }
  - { sticky: a-01J8Z0K4BBBBBBBBBBBBBBBBBB, disposition: contradicted, note: "partial refunds are out of scope for this story; tracked separately" }
  - { sticky: a-01J8Z0K5CCCCCCCCCCCCCCCCCC, disposition: open-question }
frozen: { at: 2026-05-14, commit: f80b677cac43645416a4a1441a258234e2ef763d }
---
# Stale decline handling

## Design notes

Charge API calls are retried through the outbox pattern (adr/0002)
when a decline is stale relative to the customer's current balance.

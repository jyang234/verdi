---
id: obligation/escrow-notify--ac-1--behavioral
kind: obligation
title: "A triggered escrow shortfall produces a borrower notification within 24 hours, observed end to end"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/escrow-notify" }
frozen: { at: 2026-07-12, commit: 16219044c9d6d41de9a0de9464ed24d49283b40c }
---
# A triggered escrow shortfall produces a borrower notification within 24 hours, observed end to end

The behavioral evidence must show a real escrow-shortfall event, injected
against a test account, resulting in an actual notification delivered to
the borrower's registered channel within 24 hours of the event — not a
wiring check that a notification handler exists, but the notification
itself arriving inside the window ac-1 promises.

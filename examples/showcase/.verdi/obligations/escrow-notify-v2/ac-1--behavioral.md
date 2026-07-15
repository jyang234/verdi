---
id: obligation/escrow-notify-v2--ac-1--behavioral
kind: obligation
title: "A triggered escrow shortfall produces a borrower notification within one hour, observed end to end"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/escrow-notify-v2" }
frozen: { at: 2026-07-12, commit: 16219044c9d6d41de9a0de9464ed24d49283b40c }
---
# A triggered escrow shortfall produces a borrower notification within one hour, observed end to end

The behavioral evidence must show the same injected escrow-shortfall
scenario `spec/escrow-notify`'s own obligation describes, but timed
against the tightened one-hour window: the notification must be observed
arriving within one hour of the event, not merely faster than 24 hours —
a run that happens to land at, say, ninety minutes is a failure of this
obligation even though it would have passed the predecessor's.

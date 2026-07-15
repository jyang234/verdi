---
id: obligation/escrow-notify-v2--ac-1--behavioral
kind: obligation
title: "A triggered escrow shortfall produces a borrower notification within one hour, observed end to end"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/escrow-notify-v2" }
frozen: { at: 2026-07-12, commit: 74c957aed504671bd4fc4ceb30907d2f4813e9b7 }
---
# A triggered escrow shortfall produces a borrower notification within one hour, observed end to end

The behavioral evidence must show the same injected escrow-shortfall
scenario `spec/escrow-notify`'s own obligation describes, but timed
against the tightened one-hour window: the notification must be observed
arriving within one hour of the event, not merely faster than 24 hours —
a run that happens to land at, say, ninety minutes is a failure of this
obligation even though it would have passed the predecessor's.

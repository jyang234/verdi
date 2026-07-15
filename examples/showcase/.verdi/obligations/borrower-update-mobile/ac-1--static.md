---
id: obligation/borrower-update-mobile--ac-1--static
kind: obligation
title: "The mobile PUT route writes directly, staying inside the declared offline-write exemption"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/borrower-update-mobile" }
frozen: { at: 2026-07-12, commit: 74c957aed504671bd4fc4ceb30907d2f4813e9b7 }
---
# The mobile PUT route writes directly, staying inside the declared offline-write exemption

The static evidence must show the mobile client's
`PUT /applications/:id/update` call resolves to the same handler contract
`spec/borrower-update-api` registers (no divergent mobile-only route
shape) while confirming the write itself is direct rather than
outbox-enqueued, matching the `exempts` edge this story carries against
`spec/escrow-autopay#dc-2`. It must also show the exemption is scoped
narrowly to this one mutation: every consequence downstream of the write
(staff notification, escrow recalculation) still enters loansvc's outbox
exactly as the desktop path does — a static check that finds the
exemption "leaking" into a downstream consequence is a defect the review
must catch, not evidence that ac-1 holds.

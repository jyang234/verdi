---
id: obligation/borrower-update-api--ac-1--static
kind: obligation
title: "The PUT route is registered on the application resource and returns the full updated state's shape"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/borrower-update-api" }
frozen: { at: 2026-07-12, commit: 791108c9fbc210e4ca2a23ba5625c9071883118b }
---
# The PUT route is registered on the application resource and returns the full updated state's shape

The static evidence must show `PUT /applications/:id/update` is registered
against the submitted-application resource (not draft-only), that its
handler writes through the same code path the desktop portal already uses
for every other application mutation, and that its response type is the
full application state — a partial-fields response would satisfy ac-1's
"200 with the new state" text technically but not honestly, since the
route is a full replace (`## AC-1`, `spec/borrower-update-api`), not a
patch. It must further show no outbox indirection on this route: the API
path is the feature's canonical, synchronous route (unlike the mobile
client's own offline direct-write exemption), so a static check that finds
an outbox enqueue here instead of a direct write is itself a defect, not a
pass.

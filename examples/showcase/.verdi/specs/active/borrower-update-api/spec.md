---
id: spec/borrower-update-api
kind: spec
class: story
title: "Borrower update API"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "the update API has no PUT route for a submitted application", anchor: "#problem" }
outcome: { text: "PUT /applications/:id/update returns 200 with the new state", anchor: "#outcome" }
story: jira:LOAN-1482
links:
  - { type: implements, ref: "spec/stale-decline#ac-2" }
acceptance_criteria:
  - { id: ac-1, text: "PUT /applications/:id/update returns 200 with the new state", evidence: [static, behavioral], anchor: "#ac-1" }
frozen: { at: 2026-07-12, commit: 74c957aed504671bd4fc4ceb30907d2f4813e9b7 }
---
# Borrower update API

## Problem

The update API has no PUT route for a submitted application: once a
borrower's application moves out of the draft state, servicing has no way
to correct it short of a manual database fix, which is how the mobile
story's own offline-write exemption (`spec/escrow-autopay#dc-2`) came to
matter — the desktop/API path is this feature's canonical, outbox-backed
route, and the mobile client exists to reach the same endpoint's outcome
under a harder connectivity constraint.

## Outcome

`PUT /applications/:id/update` returns 200 with the new state.

## AC-1

`PUT /applications/:id/update` returns 200 with the new state — a full
replace of the application resource, not a partial patch (unlike the
mobile route; see `spec/borrower-update-mobile-spike`'s findings for why
the two routes chose different verbs).

## Provenance

**Stub-matched fast path fixture** (R4-I-12, 03 §Lifecycle "stub-matched
fast path"): this story's implements-set ({ac-2}) equals the
`borrower-update-api` stub `spec/stale-decline` declares exactly, and
`RefSlug("Borrower update API")` equals the stub's slug
`borrower-update-api` exactly. No `supersedes`/`exempts` edges — eligible
for single-approver acceptance. The implements edge targets
`spec/stale-decline#ac-2` (the retried-charge AC) rather than a fragment
on this feature's own umbrella spec — the desktop/API route is the
canonical path the charge-retry obligation is proven against; the
mobile route below reaches the same feature under a harder connectivity
constraint via its own, separately-mapped edges.

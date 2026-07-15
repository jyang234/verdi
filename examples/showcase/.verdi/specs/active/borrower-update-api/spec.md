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
  - { type: implements, ref: "spec/escrow-autopay#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "PUT /applications/:id/update returns 200 with the new state", evidence: [static, behavioral], anchor: "#ac-1" }
frozen: { at: 2026-07-12, commit: 791108c9fbc210e4ca2a23ba5625c9071883118b }
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
fast path"): this story's implements-set ({ac-1}) equals the
`borrower-update-api` stub's declared AC set exactly, and
`RefSlug("Borrower update API")` equals the stub's slug
`borrower-update-api` exactly. No `supersedes`/`exempts` edges — eligible
for single-approver acceptance.

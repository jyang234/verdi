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
  - { type: implements, ref: "spec/accepted-pending-build#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "PUT /applications/:id/update returns 200 with the new state", evidence: [static, behavioral], anchor: "#ac-1" }
frozen: { at: 2026-07-12, commit: 93ddc5bbbb398cf747151e1c466afb83114398df }
---
# Borrower update API

**Stub-matched fast path fixture** (R4-I-12, 03 §Lifecycle "stub-matched
fast path"): this story's implements-set ({ac-1}) equals the
`borrower-update-api` stub's declared AC set exactly, and
`RefSlug("Borrower update API")` equals the stub's slug
`borrower-update-api` exactly. No `supersedes`/`exempts` edges — eligible
for single-approver acceptance.

## Problem

The update API has no PUT route for a submitted application.

## Outcome

`PUT /applications/:id/update` returns 200 with the new state.

## AC-1

`PUT /applications/:id/update` returns 200 with the new state.

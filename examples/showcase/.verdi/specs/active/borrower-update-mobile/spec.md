---
id: spec/borrower-update-mobile
kind: spec
class: story
title: "Borrower update, mobile app"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "the mobile app has no update flow for a submitted application", anchor: "#problem" }
outcome: { text: "a borrower can update their application from the mobile app and see it reflected", anchor: "#outcome" }
story: jira:LOAN-1483
links:
  - { type: implements, ref: "spec/accepted-pending-build#ac-1" }
  - { type: implements, ref: "spec/accepted-pending-build#ac-2" }
  - { type: exempts, ref: "spec/accepted-pending-build#dc-2", note: "mobile app uses direct writes for offline support, not the outbox pattern" }
  - { type: implements, ref: "spec/loan-workflow#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "mobile PUT /applications/:id/update returns 200 with the new state", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "mobile app reflects the change within the session", evidence: [behavioral], anchor: "#ac-2" }
frozen: { at: 2026-07-12, commit: 93ddc5bbbb398cf747151e1c466afb83114398df }
---
# Borrower update, mobile app

**Deviating fixture** (03 §Lifecycle "stories that deviate from the plan
... get full review"): implements the same AC set as the
`borrower-update-ui` stub ({ac-1, ac-2}), but `RefSlug` of this story's
title does not equal that stub's slug (`borrower-update-ui`) — the
title-match half of stub-match fails, so this story is not stub-matched
even though its AC set coincides. It also carries an `exempts` edge against
a feature decision fragment (rung 2, 03 §The amendment ladder) and, via its
fourth `implements` edge, touches `spec/loan-workflow#ac-1` — the object
`spec/loan-workflow-v2`'s supersession block marks `amended` (see the
rung-4 supersession-pair fixture and this story's re-affirmation record,
`reaffirmations/jira-loan-1483/ac-1.md`).

## Problem

The mobile app has no update flow for a submitted application.

## Outcome

A borrower can update their application from the mobile app and see it
reflected.

## AC-1

Mobile `PUT /applications/:id/update` returns 200 with the new state.

## AC-2

Mobile app reflects the change within the session.

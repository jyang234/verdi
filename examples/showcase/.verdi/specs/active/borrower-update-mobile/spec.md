---
id: spec/borrower-update-mobile
kind: spec
class: story
title: "Borrower update, mobile app"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "a borrower who starts an application on the mobile app has no way to correct it once submitted — they either call servicing or wait for the desktop portal, both of which lose the mobile session they were already in", anchor: "#problem" }
outcome: { text: "a borrower can update a submitted application from the mobile app, offline if needed, and sees the change reflected before they leave the session", anchor: "#outcome" }
story: jira:LOAN-1483
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
  - { type: implements, ref: "spec/stale-decline#ac-3" }
  - { type: implements, ref: "spec/escrow-autopay#ac-2" }
  - { type: exempts, ref: "spec/escrow-autopay#dc-2", note: "mobile app uses direct writes for offline support, not the outbox pattern" }
  - { type: implements, ref: "spec/loan-workflow#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "mobile PUT /applications/:id/update returns 200 with the new state", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "mobile app reflects the change within the session", evidence: [behavioral], anchor: "#ac-2" }
frozen: { at: 2026-07-12, commit: 74c957aed504671bd4fc4ceb30907d2f4813e9b7 }
---
# Borrower update, mobile app

**Deviating fixture** (03 §Lifecycle "stories that deviate from the plan
... get full review"): implements the same AC set as the
`borrower-update-mobile` stub `spec/stale-decline` declares ({ac-1, ac-3}),
but `RefSlug("Borrower update, mobile app")` (the comma in the title
produces a double-dashed slug) does not equal that stub's slug
(`borrower-update-mobile`) — the title-match half of stub-match fails, so
this story is not stub-matched even though its AC set coincides. It
carries three further edges beyond that pair: an `implements` edge into
`spec/escrow-autopay#ac-2` (the mobile client's own session-reflected-edit
guarantee applies equally to an autopay mandate edit as to a
submitted-application edit, so this one story legitimately serves both
features — and is the reason `spec/escrow-autopay-v2`'s open supersession
candidate, which amends exactly `ac-2`, flags this story
pending-supersession rather than `spec/borrower-update-api`, which does
not touch it); an `exempts` edge against a feature decision fragment (rung
2, 03 §The amendment ladder); and, via its final `implements` edge,
`spec/loan-workflow#ac-1` — the object `spec/loan-workflow-v2`'s
supersession block marks `amended` (see the rung-4 supersession-pair
fixture and this story's re-affirmation record,
`reaffirmations/jira-loan-1483/ac-1.md`).

## Problem

A borrower who starts a loan application on the mobile app has no way to
correct it once submitted. Today they either call servicing — which means
re-explaining the change to someone who has to look it up manually — or
wait until they're back at a desktop to use the portal, which loses the
mobile session they were already in and, for a borrower on a spotty
connection, is often the whole reason they were on mobile in the first
place.

## Outcome

A borrower can update a submitted application from the mobile app,
including while offline, and sees the change reflected in the app before
they leave the session — no separate confirmation step, no waiting for a
desktop.

## AC-1

Mobile `PUT /applications/:id/update` returns 200 with the new state. The
mobile client writes directly to the application record rather than going
through loansvc's outbox (see the `exempts` edge against
`spec/escrow-autopay#dc-2` above): a borrower on a train with one bar of
signal needs the update to land the moment connectivity returns, and the
outbox's eventual-consistency window — fine for a background retry — reads
as a broken save button on a phone. The direct write is scoped to this one
mutation; every downstream consequence (staff notification, escrow
recalculation) still enters through the outbox exactly as it would from
the desktop path.

## AC-2

The mobile app reflects the change within the session: the local view
updates optimistically on submit and reconciles against the server's
response, so the borrower never sees their own edit "disappear" only to
reappear after a refresh.

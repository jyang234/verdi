---
id: spec/borrower-update-mobile-spike
kind: spec
class: story
title: "Borrower update, mobile app: PUT vs PATCH spike"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "we don't know whether the mobile update route should be PUT or PATCH", anchor: "#problem" }
outcome: { text: "a recommendation with tradeoffs recorded", anchor: "#outcome" }
spike: true
story: jira:LOAN-1484
links:
  - { type: resolves, ref: "spec/escrow-autopay#oq-1" }
frozen: { at: 2026-07-12, commit: 30c5ff945413930879823be6db0ccc07d5abd6b9 }
---
# Borrower update, mobile app: PUT vs PATCH spike

**Spike variant fixture** (02 §Kind registry: "Spike variant"), sibling to
`spec/borrower-update-mobile` (the deviating story above): `spike: true`,
≥1 `resolves` edge to an open-question fragment, no `implements` edges — E3.
Exempt from the evidence model and path-fenced from product source
(03 §Ceremony pricing, VL-016) — see the path-fence violation twin under
`testdata/violations/VL-016/`.

## Problem

We don't know whether the mobile update route should be PUT (replace the
whole application resource) or PATCH (send only the changed fields). The
mobile client's offline-write model (`spec/borrower-update-mobile` ac-1)
means a stale local copy is a real risk on a flaky connection, and PUT and
PATCH fail differently when the client's copy is out of date — the choice
isn't cosmetic.

## Outcome

A recommendation with tradeoffs recorded.

## Method

Compared the two shapes against the mobile client's actual failure mode
(a queued offline write racing a server-side change made from the desktop
portal in the meantime), not against REST convention alone:

- Built the request/response contract both ways against the same
  application-update payload the desktop portal already sends.
- Traced what happens when the client's local copy is stale at submit
  time under each verb, since that's the scenario `borrower-update-mobile`
  actually has to survive offline.
- Checked how each verb interacts with the direct-write exemption
  (`spec/escrow-autopay#dc-2`) the mobile story already carries — PUT and
  PATCH both bypass the outbox equally, so the outbox question was not a
  deciding factor.

## Findings

PUT loses under this workload: it requires the client to send the entire
application state, so a stale local copy silently overwrites fields the
borrower never touched if a desktop edit landed first. PATCH sends only
the changed fields and lets the server merge them against its own current
state, which is exactly the failure mode that matters for an
intermittently-offline mobile client. Recommendation: PATCH for the mobile
route, keeping the desktop portal's existing PUT contract unchanged (it
does not have the same offline-staleness exposure) — resolved as
`spec/escrow-autopay#oq-1`.

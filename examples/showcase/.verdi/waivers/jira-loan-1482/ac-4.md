---
id: waiver/jira-loan-1482--ac-4
kind: waiver
title: "ac-4 runtime probe deferred (active)"
status: active
owners: [platform-team]
reason: "runtime probe mechanism not yet built (OQ-2)"
frozen: { at: 2026-05-01, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }
---
# Waiver: ac-4 runtime probe deferred (active)

`spec/stale-decline#ac-4` (the stale-decline rate for the affected
cohort is checked against the pre-change baseline seven days
post-deploy) can only be satisfied by a production signal — no pre-deploy
evidence can show the retry logic actually reduces stale-decline noise
in real traffic. The runtime evidence mechanism itself is still
undecided at the spec level (`03 §Open questions` OQ-2: scheduled probe
vs. OTel-derived check vs. flowmap behavior-ingest against production
traces), so there is nothing yet to run against ac-4's post-deploy
window.

No `expiry` is set: this waiver stays active until OQ-2 resolves and a
concrete runtime mechanism exists to satisfy ac-4, not until a calendar
date. `spec/stale-decline`'s own AC-rationale section names this waiver
by id alongside the story's other evidence gaps; product-lead signed off
the open-ended grant on the same basis — the gap is mechanism-blocked,
not effort-blocked, so a fixed expiry would only force a premature
re-grant.

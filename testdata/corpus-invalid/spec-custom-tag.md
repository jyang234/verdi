---
id: spec/stale-decline
kind: spec
class: feature
title: "Stale decline handling (fixture)"
status: accepted-pending-build
owners: [platform-team]
story: !urgent jira:LOAN-1482
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds for the retry path", evidence: [static] }
frozen: { at: 2026-05-14, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# Stale decline handling (custom-tag dialect twin)

Twin with a custom YAML tag (`!urgent`) on `story:`. Must fail the
restricted dialect check (PLAN.md I-1, VL-001).

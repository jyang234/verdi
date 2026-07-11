---
id: spec/stale-decline
kind: spec
class: feature
title: "Stale decline handling (fixture)"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1482
default_owner: &owner platform-team
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds for the retry path", evidence: [static] }
alias_owner: *owner
frozen: { at: 2026-05-14, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# Stale decline handling (alias dialect twin)

Twin with a YAML alias (`*owner`) referencing an anchor. Must fail the
restricted dialect check (PLAN.md I-1, VL-001) — and would also fail
KnownFields on `default_owner`/`alias_owner` even if the dialect were
somehow permitted, since neither is a real schema field.

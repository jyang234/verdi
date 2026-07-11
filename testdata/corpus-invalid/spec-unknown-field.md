---
id: spec/stale-decline
kind: spec
class: feature
title: "Stale decline handling (fixture)"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1482
bogus_extra_field: "this key does not exist in the schema"
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds for the retry path", evidence: [static] }
frozen: { at: 2026-05-14, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# Stale decline handling (unknown-field twin)

Twin of testdata/corpus/.verdi/specs/active/stale-decline/spec.md with an
injected unknown top-level field. Must fail KnownFields(true) (VL-001).

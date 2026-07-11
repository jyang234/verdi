---
id: spec/multi-story
kind: spec
class: feature
title: "VL-005 overlay: more than one story link"
status: draft
owners: [platform-team]
story: jira:LOAN-0003
links:
  - { type: story, ref: jira:LOAN-0003 }
  - { type: story, ref: confluence:LOAN-0003-notes }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-005 overlay: multiple story associations

Two `type: story` entries in `links:`, in addition to the dedicated
`story:` field. VL-005 requires "exactly one story: link with a configured
scheme" — this overlay names two schemes for the same feature spec.

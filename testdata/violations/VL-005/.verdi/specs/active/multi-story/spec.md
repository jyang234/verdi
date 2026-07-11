---
id: spec/multi-story
kind: spec
class: story
title: "VL-005 overlay: more than one story link"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0003
links:
  - { type: story, ref: jira:LOAN-0003 }
  - { type: story, ref: confluence:LOAN-0003-notes }
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-005 overlay: multiple story associations

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.

Two `type: story` entries in `links:`, in addition to the dedicated
`story:` field. VL-005 (rescoped, R4-I-2) requires the **story** class to
carry exactly one story: association with a configured scheme — this
overlay names two schemes for the same story spec. `implements` targets a
real corpus AC fragment (`spec/stale-decline#ac-1`) purely so this overlay
satisfies the story class's own decode-time requirements without tripping
an unrelated VL-003 finding.

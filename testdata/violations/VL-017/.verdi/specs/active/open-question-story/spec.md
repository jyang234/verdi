---
id: spec/open-question-story
kind: spec
class: story
title: "Open question story (VL-017 skeleton)"
status: draft
owners: [platform-team]
problem: { text: "retry behavior under tenant load is unclear", anchor: "#problem" }
outcome: { text: "retry behavior is documented and configurable if needed", anchor: "#outcome" }
story: jira:LOAN-1499
links:
  - { type: implements, ref: "spec/accepted-pending-build#ac-1" }
---
# Open question story (VL-017 skeleton)

## Problem

Retry behavior under tenant load is unclear.

## Outcome

Retry behavior is documented and configurable if needed.

## Open questions

Should the retry window be configurable per tenant? (see the sibling
mutable-zone annotation, `mutable/annotations/spec--open-question-story.jsonl`
— `status: open`, neither resolved nor carried as a declared object on
this spec.)

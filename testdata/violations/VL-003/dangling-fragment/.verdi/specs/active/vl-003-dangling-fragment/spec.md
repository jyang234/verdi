---
id: spec/vl-003-dangling-fragment
kind: spec
class: story
title: "VL-003 overlay: dangling object-id fragment"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0099
links:
  - { type: implements, ref: "spec/stale-decline#ac-99" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-003 overlay: dangling object-id fragment

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.

`links[0].ref` names a real spec (`spec/stale-decline`, in the golden
corpus) but an object id (`ac-99`) that spec does not declare — VL-003
requires an object-id fragment to resolve against the target's parsed
frontmatter objects (§Identity and references, §Object model).

---
id: spec/localop-story
kind: spec
class: story
title: "Local-operator story"
owners: [platform-team]
story: "jira:LOCALOP-1"
problem: { text: "no story implements the local-operator feature yet", anchor: "#problem" }
outcome: { text: "the local-operator feature is implemented and its acceptance criterion holds", anchor: "#outcome" }
acceptance_criteria:
  - {id: ac-1, text: "the story's local-operator claim resolves", evidence: [behavioral], anchor: ac-1}
links:
  - { type: implements, ref: "spec/localop-feature#ac-1" }
---
# Local-operator story

## Problem

No story implements the local-operator feature yet.

## Outcome

The local-operator feature is implemented and its acceptance criterion holds.

## AC-1

The story's local-operator claim resolves.

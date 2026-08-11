---
id: spec/story-multi-parent
kind: spec
title: "Multi-parent story"
owners: [story-team]
class: story
story: jira:CTX-1
problem: {text: "The two features are disconnected.", anchor: problem}
outcome: {text: "The story joins both features.", anchor: outcome}
acceptance_criteria:
  - {id: ac-1, text: "both features are joined", evidence: [behavioral], anchor: ac-1}
links:
  - {type: implements, ref: spec/feature-beta#ac-1}
  - {type: implements, ref: spec/feature-alpha#ac-2}
  - {type: implements, ref: spec/feature-alpha#ac-1}
---
# Multi-parent story

## Problem

Disconnected.

## Outcome

Joined.

## AC-1

Both features are joined.

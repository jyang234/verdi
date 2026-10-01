---
id: spec/tea-timer-alarm
kind: spec
title: "Tea timer alarm"
owners: [platform-team]
class: story
story: jira:TEA-2
problem: { text: "an over-steeped pot is noticed only when it is poured", anchor: "#problem" }
outcome: { text: "the kitchen hears an alarm when a pot passes its steep time", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the alarm sounds within a second of the steep time", evidence: [behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/tea-timer#ac-2" }
---
# Tea timer alarm

## Problem

An over-steeped pot is noticed only when it is poured.

## Outcome

The kitchen hears an alarm when a pot passes its steep time.

## ac-1

The alarm sounds within a second of the steep time.

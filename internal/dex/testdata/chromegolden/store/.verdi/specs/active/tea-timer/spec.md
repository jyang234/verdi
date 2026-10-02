---
id: spec/tea-timer
kind: spec
title: "Tea timer"
owners: [platform-team]
class: feature
story: jira:TEA-1
problem: { text: "a steeped pot goes bitter because nobody remembers when it started", anchor: "#problem" }
outcome: { text: "every pot is poured at its steep time", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a started pot reports its remaining steep time", evidence: [behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "a pot past its steep time raises an alarm", evidence: [behavioral], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "no network access", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "steep times come from the ADR's table", anchor: "#dc-1",
      links: [ { type: depends-on, ref: adr/0001-steep-time } ] }
---
# Tea timer

## Problem

A steeped pot goes bitter because nobody remembers when it started.

## Outcome

Every pot is poured at its steep time.

## ac-1

A started pot reports its remaining steep time.

## ac-2

A pot past its steep time raises an alarm.

## co-1

No network access.

## dc-1

Steep times come from the ADR's table.

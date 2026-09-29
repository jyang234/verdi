---
id: spec/closed-story
kind: spec
title: "Closed Story"
owners: [platform-team]
class: story
story: jira:CS-2
problem: { text: "the record list has no reader", anchor: problem }
outcome: { text: "the record list renders for an operator", anchor: outcome }
links:
  - { type: implements, ref: "spec/closed-feature#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the record list renders every governed record", evidence: [attestation], anchor: ac-1 }
decisions:
  - { id: dc-1, text: "the list is rendered on the server", anchor: dc-1 }
---
# Closed Story

## Problem

The record list has no reader.

## Outcome

The record list renders for an operator.

## AC-1

The record list renders every governed record.

## DC-1

The list is rendered on the server.

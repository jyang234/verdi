---
id: spec/story-successor-v2
kind: spec
title: "Story Successor v2"
owners: [platform-team]
class: story
story: jira:CS-4
problem: { text: "the record list order hides an owner's own records", anchor: problem }
outcome: { text: "the record list opens on the owner's own records", anchor: outcome }
links:
  - { type: implements, ref: "spec/other-feature#ac-1" }
  - { type: supersedes, ref: "spec/story-successor" }
acceptance_criteria:
  - { id: ac-1, text: "the record list opens on the owner's own records", evidence: [attestation], anchor: ac-1 }
decisions:
  - { id: dc-1, text: "the list opens on the owner's own records, replacing newest-first order", anchor: dc-1, links: [ { type: supersedes, ref: "spec/closed-feature#dc-1" } ] }
---
# Story Successor v2

## Problem

The record list order hides an owner's own records.

## Outcome

The record list opens on the owner's own records.

## AC-1

The record list opens on the owner's own records.

## DC-1

The list opens on the owner's own records, replacing spec/closed-feature's
newest-first order.

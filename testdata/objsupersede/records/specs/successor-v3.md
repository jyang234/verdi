---
id: spec/successor-v3
kind: spec
title: "Successor v3"
owners: [platform-team]
class: feature
problem: { text: "the governed-record list is hard to scan", anchor: problem }
outcome: { text: "operators scan governed records by owner", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "an operator can scan governed records grouped by owner", evidence: [attestation], anchor: ac-1 }
decisions:
  - { id: dc-1, text: "governed records are grouped by owner and sorted by owner name, replacing newest-first order", anchor: dc-1, links: [ { type: supersedes, ref: "spec/closed-feature#dc-1" } ] }
  - { id: dc-2, text: "the grouped list replaces the server-rendered record list and pages by owner", anchor: dc-2, links: [ { type: supersedes, ref: "spec/closed-story#ac-1" } ] }
links:
  - { type: supersedes, ref: "spec/successor-v2" }
supersession:
  carried: [ac-1]
  amended: [ { id: dc-1, note: "groups are sorted by owner name" }, { id: dc-2, note: "the grouped list pages by owner" } ]
stubs:
  - { slug: owner-groups, acceptance_criteria: [ac-1] }
---
# Successor v3

## Problem

The governed-record list is hard to scan.

## Outcome

Operators scan governed records by owner.

## AC-1

An operator can scan governed records grouped by owner.

## DC-1

Governed records are grouped by owner. This replaces spec/closed-feature's
newest-first order, because owners look for their own records first.

## DC-2

The grouped list replaces spec/closed-story's server-rendered record list
and pages by owner; ac-1 above restates what that criterion required and the
successor keeps.

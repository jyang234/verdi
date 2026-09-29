---
id: spec/successor
kind: spec
title: "Successor"
owners: [platform-team]
class: feature
problem: { text: "the governed-record list is hard to scan", anchor: problem }
outcome: { text: "operators scan governed records by owner", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "an operator can scan governed records grouped by owner", evidence: [attestation], anchor: ac-1 }
decisions:
  - { id: dc-1, text: "governed records are grouped by owner, replacing newest-first order", anchor: dc-1 }
  - { id: dc-2, text: "the grouped list replaces the server-rendered record list", anchor: dc-2 }
stubs:
  - { slug: owner-groups, acceptance_criteria: [ac-1] }
---
# Successor

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

The grouped list replaces spec/closed-story's server-rendered record list;
ac-1 above restates what that criterion required and the successor keeps.

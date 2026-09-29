---
id: spec/other-feature
kind: spec
title: "Other Feature"
owners: [platform-team]
class: feature
problem: { text: "operators cannot filter records", anchor: problem }
outcome: { text: "operators can filter records", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "an operator can filter records by owner", evidence: [attestation], anchor: ac-1 }
decisions:
  - { id: dc-1, text: "filters apply on the server", anchor: dc-1 }
stubs:
  - { slug: other-filter, acceptance_criteria: [ac-1] }
---
# Other Feature

## Problem

Operators cannot filter records.

## Outcome

Operators can filter records.

## AC-1

An operator can filter records by owner.

## DC-1

Filters apply on the server.

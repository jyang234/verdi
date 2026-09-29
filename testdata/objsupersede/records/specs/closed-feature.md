---
id: spec/closed-feature
kind: spec
title: "Closed Feature"
owners: [platform-team]
class: feature
problem: { text: "operators cannot see which records a closed feature governed", anchor: problem }
outcome: { text: "operators can read the records a closed feature governed", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "an operator can read the governed records", evidence: [attestation], anchor: ac-1 }
constraints:
  - { id: co-1, text: "records are read, never rewritten", anchor: co-1 }
decisions:
  - { id: dc-1, text: "the governed records are listed newest first", anchor: dc-1 }
stubs:
  - { slug: closed-story, acceptance_criteria: [ac-1] }
---
# Closed Feature

## Problem

Operators cannot see which records a closed feature governed.

## Outcome

Operators can read the records a closed feature governed.

## AC-1

An operator can read the governed records.

## CO-1

Records are read, never rewritten.

## DC-1

The governed records are listed newest first.

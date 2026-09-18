---
id: spec/lockbox
kind: spec
title: "Lockbox"
owners: [platform-team]
class: feature
links:
  - { type: supersedes, ref: "spec/lockbox-v0" }
  - { type: supersedes, ref: "spec/lockbox-v0#dc-1" }
problem: { text: "Keys are shared.", anchor: problem }
outcome: { text: "Each key has one holder.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A key opens one box.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "A lost key is revoked.", evidence: [static], anchor: ac-2 }
constraints:
  - { id: co-1, text: "No network.", anchor: co-1 }
decisions:
  - { id: dc-1, text: "One holder per key.", anchor: dc-1 }
open_questions:
  - { id: oq-1, text: "Who audits holders?", anchor: oq-1 }
  - { id: oq-2, text: "How long is a revocation valid?", anchor: oq-2 }
stubs:
  - { slug: key-holder, acceptance_criteria: [ac-1] }
  - { slug: audit-probe, spike: true, resolves: [oq-1] }
supersession:
  carried: [ac-1]
  amended: [ { id: ac-2, note: "tightened" } ]
  amended_advisory: []
  removed: []
  added: [co-1, dc-1, oq-1, oq-2]
---
# Lockbox

## Problem

Keys are shared today.

## Outcome

One holder.

## ac-1

Proven by opening.

## ac-2

## co-1

## dc-1

Because two holders means no holder.

## oq-1

## oq-2

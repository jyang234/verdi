---
id: spec/vatc-forge-countersign-v2
kind: spec
title: "jira:VERDI-ATC-2"
owners: [unassigned]
class: story
story: jira:VERDI-ATC-2
problem: { text: "a solo repository can never satisfy a close countersign: GitHub forbids an author's approval of their own change, while spec/vatc-forge-countersign accepts only change approvals, refuses self-approval unconditionally, and bars comments and claims as substitutes, so the solo governance profile GLG v3 defines is unreachable at close", anchor: problem }
outcome: { text: "under a solo governance profile, the owner's GitHub environment approval of the dispatch-only close run is a forge-authenticated approval fact bound to that run's exact head commit and satisfies the close countersign with the solo role collapse and every derived witness field disclosed, while team and high-assurance profiles keep independent change approvals and the self-approval refusal", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static], anchor: ac-1 }
links:
  - { type: implements, ref: "spec/todo-replace-feature-name#ac-1" }
---
# jira:VERDI-ATC-2

## Problem

TODO: design notes.

## Outcome

TODO: design notes.

## Ac 1

TODO: design notes.

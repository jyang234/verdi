---
id: spec/feature-alpha
kind: spec
title: "Feature Alpha"
owners: [alpha-team]
class: feature
problem: {text: "Alpha is unreliable.", anchor: problem}
outcome: {text: "Alpha is reliable.", anchor: outcome}
acceptance_criteria:
  - {id: ac-2, text: "alpha reports failures", evidence: [static], anchor: ac-2}
  - {id: ac-1, text: "alpha succeeds", evidence: [behavioral, static], anchor: ac-1}
  - {id: ac-untargeted, text: "alpha has an unrelated control", evidence: [attestation], anchor: ac-untargeted}
constraints:
  - {id: co-second, text: "preserve the second declared constraint", anchor: co-second}
  - {id: co-first, text: "preserve the first-named constraint second", anchor: co-first}
decisions:
  - id: dc-second
    text: "preserve the second-named decision first"
    anchor: dc-second
    links:
      - {type: depends-on, ref: adr/alpha-base, note: "complete authored link"}
      - {type: exempts, ref: adr/alpha-exception}
  - {id: dc-first, text: "preserve the first-named decision second", anchor: dc-first}
open_questions:
  - {id: oq-1, text: "which alpha strategy applies?", anchor: oq-1}
  - {id: oq-untargeted, text: "which unrelated alpha strategy applies?", anchor: oq-untargeted}
stubs:
  - {slug: alpha-story, acceptance_criteria: [ac-1]}
---
# Feature Alpha

Body prose must not enter the fragment.

## Problem

Alpha is unreliable.

## Outcome

Alpha is reliable.

## AC-2

Alpha reports failures.

## AC-1

Alpha succeeds.

## AC-Untargeted

Unrelated.

## CO-Second

Second declared.

## CO-First

First named.

## DC-Second

Second named.

## DC-First

First named.

## OQ-1

Question.

## OQ-Untargeted

Unrelated question.

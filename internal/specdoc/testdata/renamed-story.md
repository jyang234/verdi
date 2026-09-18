---
id: spec/renamed-vocabulary
kind: spec
title: "Renamed Vocabulary Walkthrough"
owners: [platform-team]
class: feature
acceptance_criteria:
  - { id: ac-1, text: "First criterion holds.", evidence: [static], anchor: ac-1 }
  - { id: ac-2, text: "Second criterion holds.", evidence: [static], anchor: ac-2 }
  - { id: ac-3, text: "Third criterion holds.", evidence: [static], anchor: ac-3 }
  - { id: ac-4, text: "Fourth criterion holds.", evidence: [static], anchor: ac-4 }
  - { id: ac-5, text: "Fifth criterion holds.", evidence: [static], anchor: ac-5 }
  - { id: ac-6, text: "Sixth criterion holds.", evidence: [static], anchor: ac-6 }
  - { id: ac-7, text: "Seventh criterion holds.", evidence: [static], anchor: ac-7 }
  - { id: ac-8, text: "Eighth criterion holds.", evidence: [static], anchor: ac-8 }
  - { id: ac-9, text: "Ninth criterion holds.", evidence: [static], anchor: ac-9 }
  - { id: ac-10, text: "Tenth criterion holds.", evidence: [static, behavioral], anchor: ac-10 }
  - { id: ac-11, text: "Eleventh criterion holds.", evidence: [static], anchor: ac-11 }
constraints:
  - { id: co-1, text: "Renamed words must still fail closed on an unknown id.", anchor: co-1 }
open_questions:
  - { id: oq-1, text: "Does a third rename layer ever apply?", anchor: oq-1 }
---
# Renamed Vocabulary Walkthrough

## ac-10

First paragraph of detail for the tenth criterion, deliberately placed on
the item whose number crosses into double digits so the continuation
indent must widen to stay aligned.

Second paragraph of detail for the same criterion, to prove the whole
multi-paragraph detail stays nested inside the list item once the
continuation indent is computed per item rather than hard-coded.

## co-1

Prose explaining why a renamed word must still fail closed on an id the
vocabulary block never declared.

## oq-1

Prose describing what a third rename layer would even mean, and why none
exists today.

---
id: spec/vl-003-dangling-pin
kind: spec
class: feature
title: "VL-003 overlay: dangling pin"
status: draft
owners: [platform-team]
story: jira:LOAN-0001
context:
  - adr/0002-outbox-events@0000000000000000000000000000000000000000
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-003 overlay: dangling pin

`context[0]` is a well-formed pinned ref (valid kind/name@commit shape)
but `0000...0000` is not a real commit anywhere in the corpus's git
history. VL-003 requires "pins name real commits".

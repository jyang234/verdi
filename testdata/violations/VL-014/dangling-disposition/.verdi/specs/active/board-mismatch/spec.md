---
id: spec/board-mismatch
kind: spec
class: feature
title: "VL-014 overlay: dangling disposition"
status: draft
owners: [platform-team]
story: jira:LOAN-0006
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
dispositions:
  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: open-question }
  - { sticky: a-01J8Z0K9ZZZZZZZZZZZZZZZZZZ, disposition: open-question }
---
# VL-014 overlay: dangling disposition

Paired with `board.json` in this directory, which has only one sticky
(`a-01J8Z0K3AAAAAAAAAAAAAAAAAA`). The second `dispositions:` entry names
`a-01J8Z0K9ZZZZZZZZZZZZZZZZZZ`, which is not a real board sticky. VL-014
requires every disposition entry to name a real board sticky.

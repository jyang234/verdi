---
id: spec/board-mismatch
kind: spec
class: feature
title: "VL-014 overlay: missing sticky"
status: draft
owners: [platform-team]
story: jira:LOAN-0005
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
dispositions:
  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: open-question }
---
# VL-014 overlay: missing sticky

Body has no `#design-notes` heading (irrelevant here). Paired with
`board.json` in this directory, which has two stickies
(`a-01J8Z0K3AAAAAAAAAAAAAAAAAA` and `a-01J8Z0K4BBBBBBBBBBBBBBBBBB`) but
this `dispositions:` block covers only the first. VL-014 requires every
board sticky to be dispositioned.

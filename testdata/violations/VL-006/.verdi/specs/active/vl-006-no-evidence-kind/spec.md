---
id: spec/vl-006-no-evidence-kind
kind: spec
class: feature
title: "VL-006 overlay: AC declares no evidence kind"
status: draft
owners: [platform-team]
story: jira:LOAN-0004
acceptance_criteria:
  - { id: ac-1, text: "no evidence kinds declared", evidence: [] }
---
# VL-006 overlay: AC with an empty evidence list

`acceptance_criteria[0].evidence` is empty. VL-006 ("activation lint")
requires every AC to declare at least one expected evidence kind. Note:
internal/artifact already enforces this at decode time (defense in depth)
— this overlay documents the same violation shape VL-006 checks.

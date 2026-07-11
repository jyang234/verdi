---
id: spec/vl-014-contradicted-without-note
kind: spec
class: feature
title: "VL-014 overlay: contradicted without note"
status: draft
owners: [platform-team]
story: jira:LOAN-0008
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
dispositions:
  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: contradicted }
---
# VL-014 overlay: contradicted without a note

`disposition: contradicted` requires a `note` (I-5); this entry has none.
Note: internal/artifact already rejects this at decode time (defense in
depth) — this overlay documents the same violation shape VL-014 checks.

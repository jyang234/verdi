---
id: spec/vl-014-incorporated-without-where
kind: spec
class: feature
title: "VL-014 overlay: incorporated without where"
status: draft
owners: [platform-team]
story: jira:LOAN-0007
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
dispositions:
  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: incorporated }
---
# VL-014 overlay: incorporated without a where anchor

`disposition: incorporated` requires a resolving `where` anchor (I-5);
this entry has none. Note: internal/artifact already rejects this at
decode time (defense in depth) — this overlay documents the same
violation shape VL-014 checks.

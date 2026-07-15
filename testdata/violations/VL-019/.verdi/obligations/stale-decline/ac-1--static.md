---
id: obligation/stale-decline--ac-1--static
kind: obligation
title: "VL-019 overlay: obligation verifies a whole FEATURE spec"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/stale-decline" }
frozen: { at: 2026-07-13, commit: 9f5621543d6e5158ad3230a7febc83754f2be3dd }
---
# VL-019 overlay: obligation verifies a whole FEATURE spec

An obligation verifies the WHOLE story spec — a bare `spec/<story>` ref with
no fragment, exactly as an attestation does; the acceptance criterion is
named by the obligation's own id (`ac-1`), not the edge. `spec/stale-decline`
is `class: feature` in the golden corpus, so it is not a STORY — VL-019 must
refuse this obligation's `verifies` edge, naming the whole feature spec it
wrongly targets. (Obligations attach to STORY acceptance criteria only, 03
§The feature fold.)

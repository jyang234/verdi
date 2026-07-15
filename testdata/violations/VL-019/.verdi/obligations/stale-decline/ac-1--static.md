---
id: obligation/stale-decline--ac-1--static
kind: obligation
title: "VL-019 overlay: obligation verifies a whole FEATURE spec"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/stale-decline" }
frozen: { at: 2026-07-13, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-019 overlay: obligation verifies a whole FEATURE spec

An obligation verifies the WHOLE story spec — a bare `spec/<story>` ref with
no fragment, exactly as an attestation does; the acceptance criterion is
named by the obligation's own id (`ac-1`), not the edge. `spec/stale-decline`
is `class: feature` in the golden corpus, so it is not a STORY — VL-019 must
refuse this obligation's `verifies` edge, naming the whole feature spec it
wrongly targets. (Obligations attach to STORY acceptance criteria only, 03
§The feature fold.)

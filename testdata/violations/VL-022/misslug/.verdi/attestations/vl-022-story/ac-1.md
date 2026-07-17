---
id: attestation/vl-022-story--ac-1
kind: attestation
title: "VL-022 overlay: attestation slug disagrees with its verifies target's story-ref slug"
owners: [platform-team]
links:
  - { type: verifies, ref: "spec/vl-022-story" }
frozen: { at: 2026-07-16, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-022 overlay: attestation slug disagrees with its verifies target's story-ref slug

This attestation's id/path both name "vl-022-story" (VL-011's own id/path
agreement is satisfied — the spec's OWN directory name), but its `verifies`
edge resolves to `spec/vl-022-story`, whose own `story:` field is
`jira:VL022-1` — `store.RefSlug("jira:VL022-1")` is `jira-vl022-1`, not
`vl-022-story`. This is the exact D6-18 class of bug: a spec-name slug
substituted for the story-ref slug the fold actually reads. VL-022 must
refuse this, naming the disagreeing values.

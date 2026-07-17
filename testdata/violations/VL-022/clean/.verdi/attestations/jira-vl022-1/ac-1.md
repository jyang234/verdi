---
id: attestation/jira-vl022-1--ac-1
kind: attestation
title: "VL-022 overlay: a correctly-slugged, well-formed attestation"
owners: [platform-team]
links:
  - { type: verifies, ref: "spec/vl-022-story" }
frozen: { at: 2026-07-16, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-022 overlay: a correctly-slugged, well-formed attestation

This attestation's directory is `jira-vl022-1` — exactly
`store.RefSlug("jira:VL022-1")`, the `verifies` target's own story-ref slug
(`spec/vl-022-story`'s `story:` field). Every VL-022 check passes: the
target resolves, is `class: story`, declares `ac-1`, and the slug agrees.
VL-022 must produce no finding here.

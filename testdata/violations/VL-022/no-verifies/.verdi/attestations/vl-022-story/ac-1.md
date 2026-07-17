---
id: attestation/vl-022-story--ac-1
kind: attestation
title: "VL-022 overlay: no verifies edge at all — out of scope by construction"
owners: [platform-team]
frozen: { at: 2026-07-16, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-022 overlay: no verifies edge at all — out of scope by construction

This attestation carries NO `verifies` link at all — mirroring every
pre-existing, hand-authored attestation in the store as of this rule's
authoring (dc-4). Its own directory ("vl-022-story") does not even match
its nominal target's story-ref slug ("jira-vl022-1"), but VL-022 must stay
silent regardless: the rule fires ONLY on attestations that carry a
`verifies` edge at all (dc-4's disclosed scope limit) — this is the
residual gap disclosed, not silently accepted.

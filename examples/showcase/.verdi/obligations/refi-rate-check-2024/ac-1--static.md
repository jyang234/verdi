---
id: obligation/refi-rate-check-2024--ac-1--static
kind: obligation
title: "The rate-check path reads every field it prices off the published table, never a cached or hardcoded value"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/refi-rate-check-2024" }
frozen: { at: 2026-07-01, commit: 30c5ff945413930879823be6db0ccc07d5abd6b9 }
---
# The rate-check path reads every field it prices off the published table, never a cached or hardcoded value

The static evidence must show the rollout rate-check path resolves every
priced field — base rate, promotional-rate expiration column included —
by column name against the current published-table schema, not by a
fixed column position or a value cached from a prior table version. This
is the exact defect class that let two stale promotional rates through
before the 2024 rebuild: a hardcoded or position-based read is a static
defect even on a build that happens to price correctly against today's
table layout.

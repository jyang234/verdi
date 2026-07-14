---
id: obligation/forge-transport--ac-3--static
kind: obligation
title: "Deadlines and 429 classification in source"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/forge-transport" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# Deadlines and 429 classification in source

The static evidence must show the seam's default client carrying the 30s
deadline with injected clients used as-is, jira's classifier mapping 429 to
provider.ErrUnavailable, and the forge classifiers producing a rate-limited
refusal naming status 429.

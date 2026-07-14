---
id: obligation/forge-transport--ac-3--behavioral
kind: obligation
title: "Stalls time out and rate limits degrade"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/forge-transport" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# Stalls time out and rate limits degrade

The behavioral evidence must show a stalling handler timed out by a short
injected client (never a 30s test sleep), a canned 429 matching
provider.ErrUnavailable via errors.Is on the tracker side, and the forge
refusal naming 429 — routing rate limits to the degrade/retry path instead
of a hard failure.

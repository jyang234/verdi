---
id: obligation/shared-homes--ac-2--static
kind: obligation
title: "One digest tail in canonjson"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/shared-homes" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# One digest tail in canonjson

The static evidence must show canonjson.Digest as the only
marshal→sha256→sha256:-hex implementation, the ten former copies collapsed
onto it, and artifact.ObjectContentHash surviving as a documented one-line
wrapper.

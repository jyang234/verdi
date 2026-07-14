---
id: obligation/forge-transport--ac-4--behavioral
kind: obligation
title: "Pin mismatch refused, absence disclosed"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/forge-transport" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# Pin mismatch refused, absence disclosed

The behavioral evidence must show a matching pseudo-version accepted
silently, a mismatched one refused as an operational error naming both the
recorded and pinned commits, a carrier-less bundle accepted WITH the
disclosed-unproven stdout notice, and a malformed carrier refused by the
strict decode.

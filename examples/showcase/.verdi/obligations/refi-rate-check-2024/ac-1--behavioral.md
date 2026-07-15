---
id: obligation/refi-rate-check-2024--ac-1--behavioral
kind: obligation
title: "A refinance quote priced against a changed published table matches the table, not a stale cached rate"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/refi-rate-check-2024" }
frozen: { at: 2026-07-01, commit: 30c5ff945413930879823be6db0ccc07d5abd6b9 }
---
# A refinance quote priced against a changed published table matches the table, not a stale cached rate

The behavioral evidence must show a real rollout run: the published
table is changed between two quotes for the same loan program, and the
second quote's applied rate matches the new table, not the rate the
first quote used — reproducing, and closing, the exact promotional-rate
staleness gap the 2024 rebuild exists to fix.

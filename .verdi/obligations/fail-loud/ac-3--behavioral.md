---
id: obligation/fail-loud--ac-3--behavioral
kind: obligation
title: "Strict refusal and connection traces witnessed"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/fail-loud" }
frozen: { at: 2026-07-13, commit: 7f3c08d367dd4f76b45e982dc03813875e0e7a7c }
---
# Strict refusal and connection traces witnessed

The behavioral evidence must show: a typo'd tool-argument field
(target_reff) refused with an error NAMING the unknown field, never
silently dropped; trailing data refused; a lock file with an unknown field
refused by name; a dropped socket connection leaving exactly one stderr
line while a clean close leaves none and a nil ErrLog stays silent — all
hermetic, existing well-formed callers regression-free.

---
id: obligation/fail-loud--ac-4--static
kind: obligation
title: "Contracts and counts stated where a reader looks"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/fail-loud" }
frozen: { at: 2026-07-13, commit: 7f3c08d367dd4f76b45e982dc03813875e0e7a7c }
---
# Contracts and counts stated where a reader looks

The static evidence must show the record matching reality: boardio's
package doc states the caller-holds-the-write-lock contract its
read-modify-write helpers assume (naming workbench's writeMu as the
production instance); the three fourteen-rules comments and
testdata/violations/README.md match the registered rule set; VL-019
carries its ratified row in 02 §Lint rules with the 08-revision-notes
recording entry.

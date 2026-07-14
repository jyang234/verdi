---
id: obligation/file-topics--ac-4--behavioral
kind: obligation
title: "Harness hygiene proven by the gate it serves"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/file-topics" }
frozen: { at: 2026-07-14, commit: 15d60efbe02636c1112907ded017f80eb4c46e94 }
---
# Harness hygiene proven by the gate it serves

The behavioral evidence must show the full e2e suite passing unchanged
through the reworked harness, plus targeted unit tests for the
extractable pieces (copyTree's error split, the run-git helper's env).

---
id: obligation/fail-loud--ac-1--behavioral
kind: obligation
title: "The binary-refusal check witnessed red then green"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/fail-loud" }
frozen: { at: 2026-07-13, commit: 7f3c08d367dd4f76b45e982dc03813875e0e7a7c }
---
# The binary-refusal check witnessed red then green

The behavioral evidence must show the check actually FIRING: red against
the tracked 21.8 MB e2eharness Mach-O (the witness run naming that path and
its matched magic), green after `git rm` + ignore — proving the instance
was removed AND the class is henceforth refused by the same test.

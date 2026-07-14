---
id: obligation/fail-loud--ac-1--static
kind: obligation
title: "The gate itself refuses tracked compiled binaries"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/fail-loud" }
frozen: { at: 2026-07-13, commit: 7f3c08d367dd4f76b45e982dc03813875e0e7a7c }
---
# The gate itself refuses tracked compiled binaries

The static evidence must show the class-level refusal EXISTS in the gate:
internal/specalign carries a repo-hygiene check that walks `git ls-files`
and refuses any tracked file opening with a compiled-binary magic (the
Mach-O family including swapped/fat forms, ELF, PE), naming the offending
path as witness — and .gitignore carries /e2eharness so the deleted build
output cannot silently return.

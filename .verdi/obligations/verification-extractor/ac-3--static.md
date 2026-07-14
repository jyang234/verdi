---
id: obligation/verification-extractor--ac-3--static
kind: obligation
title: "The three-way comparison's output type has exactly exists/proposed-new/kept-but-gone, no fourth rename value"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/verification-extractor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# The three-way comparison's output type has exactly exists/proposed-new/kept-but-gone, no fourth rename value

The static evidence must show the comparison function's result type (an
enum or equivalent closed set) contains exactly three classification
values — `exists`, `proposed-new`, `kept-but-gone` — and no `renamed` or
similar fourth value anywhere in the type or its consumers, and must show
the `kept-but-gone` case's shape carries an optional witness-commit field
(present when resolved, absent when `git log -S` found no hit — never a
fabricated placeholder commit). The evidence must also point to the
specific function implementing DC-4's `git log -S<identity> -1
--format=%H -- <dir>` witness lookup and confirm it is the only commit-
discovery mechanism in this story (no second, competing implementation).

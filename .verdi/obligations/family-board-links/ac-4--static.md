---
id: obligation/family-board-links--ac-4--static
kind: obligation
title: "A Go unit test proves an unresolvable implements target renders the disclosed notice and no href"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/family-board-links" }
frozen: { at: 2026-07-16, commit: 71bf8422596fadd4c3060aa719785b91e114924f }
---
# A Go unit test proves an unresolvable implements target renders the disclosed notice and no href

The static evidence must show the negative path of AC-1's enrichment function,
table-driven, over a fixture `index.Index` missing the edge target: the function
yields a disclosed inline notice naming the unresolved ref and NO href — never a
silently inert ref card, never a dead `<a href>` that would 404 when followed
(co-3). Paired with AC-1's present-target row, this is the happy and negative
coverage of one resolution seam. Build and vet clean over the package.

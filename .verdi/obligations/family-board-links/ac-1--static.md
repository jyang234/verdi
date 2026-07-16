---
id: obligation/family-board-links--ac-1--static
kind: obligation
title: "A Go unit test proves the story-to-feature-board affordance resolves its declared implements target, present and absent"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/family-board-links" }
frozen: { at: 2026-07-16, commit: 71bf8422596fadd4c3060aa719785b91e114924f }
---
# A Go unit test proves the story-to-feature-board affordance resolves its declared implements target, present and absent

The static evidence must show a table-driven Go unit test over AC-1's
parent-feature enrichment — the store-derived, per-request field dc-2 attaches
after the pure projector runs — driven against a fixture `index.Index` with the
target feature ref both present and absent. Present yields the
`/board/spec/<feature-name>` affordance (the feature's own board, not only the
corpus page); absent yields no href and defers to AC-4's disclosed notice. The
resolution is a plain index lookup — the same existence check
`internal/dex/permalink.go`'s `resolvableLinkURL` already performs (dc-1), never
a second backlink walk. Build and vet clean over the package; happy and
negative rows both present.

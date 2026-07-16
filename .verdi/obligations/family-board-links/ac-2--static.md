---
id: obligation/family-board-links--ac-2--static
kind: obligation
title: "A Go unit test proves the stub-card match reuses the backlink inversion and renders active vs archived distinctly"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/family-board-links" }
frozen: { at: 2026-07-16, commit: 71bf8422596fadd4c3060aa719785b91e114924f }
---
# A Go unit test proves the stub-card match reuses the backlink inversion and renders active vs archived distinctly

The static evidence must show a table-driven Go unit test over AC-2's stub-card
match, computed by filtering `ix.Backlinks(featureRef+"#"+acID)` to
`Type == "implemented-by"` — the SAME primitive the feature fold already uses
(dc-1), never a second graph walk or heuristic title/slug matching. The table
must cover the ADJ-28 completion reading exhaustively: one matching ACTIVE story
(renders the plain `/board/spec/<story>` link, parent ac-2 verbatim); one
matching ARCHIVED story (renders the same board link WITH its archived state
disclosed on the card, and NEVER the "not yet in this checkout's active store"
text); zero matches (defers to AC-3); and the multi-story fan-out (dc-4: every
distinct match linked, unranked). Build and vet clean over the package.

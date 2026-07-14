---
id: obligation/draft-boards--ac-3--static
kind: obligation
title: "No new mode: the branch-state computation is consumed, not reimplemented"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/draft-boards" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# No new mode: the branch-state computation is consumed, not reimplemented

The static evidence must show that the board's mode vocabulary is
unchanged — the modeAuthoring/modeReadOnly/modeReview set carries no new
value and no /b/-specific mode — and that the per-branch instances reach
their mode through the SAME existing branch-state computation (loadBoard's
mode switch: status draft on a non-default branch means authoring),
evaluated against each instance's own tree rather than through any
duplicated or special-cased mode logic in the routing layer. The
unprefixed /board/spec/{name} route must be present and unchanged in the
route table (dc-3: the serving checkout's view retires no address). Build
and vet clean.

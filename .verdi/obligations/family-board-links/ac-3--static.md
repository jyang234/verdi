---
id: obligation/family-board-links--ac-3--static
kind: obligation
title: "A Go unit test proves the branch-presence check gates the verbatim in-between disclosure only in the no-match case"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/family-board-links" }
frozen: { at: 2026-07-16, commit: 71bf8422596fadd4c3060aa719785b91e114924f }
---
# A Go unit test proves the branch-presence check gates the verbatim in-between disclosure only in the no-match case

The static evidence must show a table-driven Go unit test over AC-3's
branch-presence enrichment, which reuses `internal/gitx.HasLocalBranch(ctx,
root, "design/"+slug)` against the serving checkout's own root (dc-3). It must
assert, in the no-match-anywhere case: ref present renders the disclosure
"instantiated on design/<slug>, not yet in this checkout's active store" with
the literal branch name substituted, VERBATIM per parent dc-5; ref absent
renders the plain un-instantiated state, unchanged. Per ADJ-28 it must also
assert that an ARCHIVED match never reaches this ref-check path at all (it takes
AC-2's archived-disclosure card instead). Build and vet clean; happy and
negative rows both present.

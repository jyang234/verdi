---
id: obligation/directory-home--ac-1--behavioral
kind: obligation
title: "An e2e sees the four status groups, each entry once, chipped and linked"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/directory-home" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# An e2e sees the four status groups, each entry once, chipped and linked

The behavioral evidence must show a Playwright e2e (under e2e/tests/)
driving `GET /` on a served fixture store whose refs span all four feature
dc-2 groups — at least one draft on a design branch, one
accepted-pending-build spec, one active component, and one terminal
(closed/archived or superseded) spec — and asserting: (a) the page renders
the four status groups as its organizing structure; (b) every fixture spec
appears exactly once (the design-branch draft included — the entry the old
home page could not show); (c) every entry carries a visible status chip;
and (d) every entry's board link href is the correct address per dc-3 — an
unprefixed /board/spec/<name> for a default-branch entry, and a
/b/<branch-escaped>/board/spec/<name> address for the design-branch draft.

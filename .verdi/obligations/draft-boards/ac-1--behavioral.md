---
id: obligation/draft-boards--ac-1--behavioral
kind: obligation
title: "An e2e opens a draft under /b/ and works the authoring wall and its sub-routes"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/draft-boards" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# An e2e opens a draft under /b/ and works the authoring wall and its sub-routes

The behavioral evidence must show a Playwright e2e (under e2e/tests/)
against a served fixture store with a draft spec on a local design branch:
(a) GET /b/<branch-escaped>/board/spec/<name> renders that spec's board in
AUTHORING mode (the authoring affordances present — not the read-only
record), with the spec content coming from the design branch's tree, not
the serving checkout's; and (b) the board's sub-routes function beneath
the prefix — at minimum a board mutation through the prefixed api route
succeeds and the prefixed fragment route returns the re-rendered region
reflecting it. The first open (the lazy worktree cut, dc-2) must complete
within the suite's ordinary timeouts, and a branch that cannot be resolved
must render a disclosed error/notice page rather than a bare failure.

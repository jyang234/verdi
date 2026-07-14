---
id: obligation/draft-boards--ac-2--behavioral
kind: obligation
title: "Two tabs, two branches, zero interference — and a clean serving checkout"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/draft-boards" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Two tabs, two branches, zero interference — and a clean serving checkout

The behavioral evidence must show a Playwright e2e (under e2e/tests/)
driving TWO pages/tabs against one served fixture store carrying two
different design branches, each with its own draft spec: (a) both boards
render and stay usable simultaneously — not alternately; (b) an authoring
edit performed through tab A lands in branch A's managed worktree only —
tab B's board, re-fetched, is byte-for-byte unaffected by it, and branch
B's tree carries no trace of the edit; and (c) the serving checkout's own
working tree is clean after the whole exchange (asserted via the harness,
e.g. git status --porcelain empty over the serve root), proving that
opening and editing drafts never disturbs the serving checkout (feature
dc-1's no-surprise-mutation law, witnessed end to end).

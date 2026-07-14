---
id: obligation/draft-boards--ac-3--behavioral
kind: obligation
title: "The same spec: sealed at its default-branch address, authoring under /b/"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/draft-boards" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# The same spec: sealed at its default-branch address, authoring under /b/

The behavioral evidence must show a Playwright e2e (under e2e/tests/) over
a fixture store where the SAME spec name exists both landed on the default
branch and as a draft on its own design branch: in one session, (a) the
unprefixed /board/spec/<name> renders the sealed, read-only record — no
authoring affordances — from the serving checkout; (b)
/b/<branch-escaped>/board/spec/<name> renders the authoring wall from the
design branch's managed worktree, its content the draft's, not the landed
record's; and (c) both renders are reachable in the same session without
either changing the other — the mode law (feature ac-6) witnessed as two
simultaneous truths of one spec, not a toggle.

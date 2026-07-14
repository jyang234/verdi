---
id: obligation/directory-home--ac-3--behavioral
kind: obligation
title: "Empty and deleted branches degrade to disclosed notices, never dead links"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/directory-home" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Empty and deleted branches degrade to disclosed notices, never dead links

The behavioral evidence must show a Playwright e2e (under e2e/tests/)
driving both feature-ac-5 shapes against a served fixture store: (a) a
design branch that carries NO draft spec renders in the directory as a
disclosed notice entry that names the branch and states the absence — it
is not linked as if a board existed and it is not silently omitted; and
(b) after the directory page is rendered, the fixture deletes a listed
design branch, and clicking that entry's link resolves to a rendered
disclosed notice page — HTTP 404, a human-readable body naming what
vanished, and a working link back to the directory — never a bare
NotFound, never a blank or broken response. In both shapes the directory
page itself still renders fully: one unresolvable entry never fails the
page.

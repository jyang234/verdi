---
id: obligation/home-status-glance--ac-2--behavioral
kind: obligation
title: "Every pre-existing directory section and link survives unchanged"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/home-status-glance" }
frozen: { at: 2026-07-16, commit: d11cd50bf4840109ef8834b16e97a1920805c178 }
---
# Every pre-existing directory section and link survives unchanged

The behavioral evidence must show `e2e/tests/43-home-status-glance.spec.ts`
(a dedicated test within it, run against the SAME fixture store the ac-1
obligation's test uses) asserting that every pre-existing surface
`37-directory-home.spec.ts` already proves is still present and
unchanged: all four `dir-group-*` sections and every fixture's
`dir-entry-*`, still status-chipped, source-chipped, and linked exactly as
before; the `.home-kinds`/`.home-services`/`.home-boards` sections with
their existing headings; the store-root notice; and the disclosures
pointer. The store must carry both the glance's own population and at
least one case the existing directory suite already covers end to end
(e.g. the in-review chip, or the disclosed-branch-with-no-spec notice), so
the test can show the new leading section changed nothing about how the
old section renders that same data.

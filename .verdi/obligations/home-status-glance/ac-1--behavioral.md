---
id: obligation/home-status-glance--ac-1--behavioral
kind: obligation
title: "The glance section groups every entry into three fixed buckets, badged and linked correctly per source and class"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/home-status-glance" }
frozen: { at: 2026-07-16, commit: d11cd50bf4840109ef8834b16e97a1920805c178 }
---
# The glance section groups every entry into three fixed buckets, badged and linked correctly per source and class

The behavioral evidence must show a new Playwright spec,
`e2e/tests/43-home-status-glance.spec.ts`, driving `GET /` over a fixture
store spanning every status value this store's schema legalizes: a
design-branch draft (reusing the existing directory-home fixtures
`SHOWCASE.DESIGN_SPEC`/`SHOWCASE.DIR_LOCAL_DRAFT`/`SHOWCASE.DIR_REMOTE_DRAFT`),
a default-branch `accepted-pending-build` feature (`stale-decline`), a
default-branch `active` component (`store-layout-notes`), a default-branch
`superseded` component still in the active zone (`legacy-cache-policy`), an
archive-zone `closed` feature (`loan-refi-2023`), and — because no
committed showcase fixture today carries this exact shape — one new,
minimal e2e-harness-provisioned (scratch store, never the committed
examples/showcase corpus) spec whose status is `closed` while it is still
physically in `.verdi/specs/active/` (parent dc-4's own "closed awaiting
archive" example).

The test must assert: the three `glance-group-*` sections
(`on-the-desk`, `in-flight`, `settling`) render in that fixed order; each
fixture's entry appears exactly once, under the correct bucket; each
entry's status badge reads its real raw status; each entry's board link
resolves to the unprefixed `/board/spec/<name>` for a default-branch entry
(present only when boardServable — absent for the archive-zone entry) or
the per-branch `/b/<branch>/board/spec/<name>` for the design-branch
draft; and matrix+verdict links appear only on the default-branch feature
entry, never on the component entries and never on the design-branch
draft.

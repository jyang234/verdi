---
id: obligation/directory-home--ac-2--behavioral
kind: obligation
title: "Source disclosure and the in-review chip, with its disclosed degradation, witnessed"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/directory-home" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Source disclosure and the in-review chip, with its disclosed degradation, witnessed

The behavioral evidence must show, over hermetic fixtures (no network):
(a) a directory render where a local design branch and a remote-tracking
design ref (a fixture refs/remotes/origin/design/* entry) are visibly
distinguished by source on the page; (b) with a hermetic forge double
(httptest or canned feed) reporting an open MR for one branch, that
branch's entry — and only that entry — renders an "in review" chip; and
(c) the SAME surface rendered with the forge double unreachable (or
erroring) shows a disclosed "MR status unavailable" notice in place of the
chip while every entry of the refs-computed directory still renders — the
page is complete, not blocked, not partial, and the absence is stated, not
silent. At least the on-page assertions run as Playwright e2e under
e2e/tests/.

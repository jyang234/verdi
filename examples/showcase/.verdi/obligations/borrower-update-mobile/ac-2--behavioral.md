---
id: obligation/borrower-update-mobile--ac-2--behavioral
kind: obligation
title: "The mobile app's own view reflects the update before the borrower leaves the session"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/borrower-update-mobile" }
frozen: { at: 2026-07-12, commit: 16219044c9d6d41de9a0de9464ed24d49283b40c }
---
# The mobile app's own view reflects the update before the borrower leaves the session

The behavioral evidence must show the mobile app's local view updating on
submit and reconciling against the server's actual response within the
same session — not merely that the server-side state changed (that is
ac-1's claim), but that the client the borrower is looking at shows the
change without a manual refresh or app restart. A test that asserts only
the server-side state, without also asserting the client's rendered view,
does not satisfy ac-2: the whole point of this AC is that the borrower
never sees their own edit "disappear" only to reappear later.

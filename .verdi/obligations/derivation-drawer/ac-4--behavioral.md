---
id: obligation/derivation-drawer--ac-4--behavioral
kind: obligation
title: "Drawer markup cites revisions and carries no wall-clock time"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/derivation-drawer" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Drawer markup cites revisions and carries no wall-clock time

The behavioral evidence must render fixture drawers and assert (a) every
cited input line carries a digest/sha revision, and (b) the drawer markup
contains no rendered date or time — proven by rendering the same fixture
at two different (faked or real) times and asserting identical drawer
bytes, so any smuggled clock read fails the test rather than an assertion
over string patterns alone.

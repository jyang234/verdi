---
id: obligation/board-editor--ac-4--behavioral
kind: obligation
title: "e2e peeks and resets a derived fixture proposal, and sees the disclosed failure on a corrupted digest"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/board-editor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# e2e peeks and resets a derived fixture proposal, and sees the disclosed failure on a corrupted digest

The behavioral evidence must show Playwright e2e coverage over a fixture
store holding a derived proposal (derived_from.ref pinned at a fixturegit
commit, derived_from.digest matching that base): (1) before-peek renders the
pinned base's diagram read-only beside the working preview without modifying
the code pane or the artifact; (2) reset replaces the pane source with the
base source byte-for-byte (asserted against the fixture's known base bytes)
and the reloaded artifact body equals it; (3) on a second fixture whose
derived_from.digest is deliberately corrupted, both affordances show a
visible, disclosed digest-mismatch failure and the artifact on disk is
byte-identical before and after the attempts. A from-scratch proposal
(no derived_from) must not offer the affordances at all. Evidence lacking
the corrupted-digest fixture or the no-write assertion does not satisfy
this obligation.

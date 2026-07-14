---
id: obligation/derivation-drawer--ac-3--behavioral
kind: obligation
title: "A stale or partial sweep looks stale on the wall"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/derivation-drawer" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# A stale or partial sweep looks stale on the wall

The behavioral evidence must drive three fixtures: (a) a fresh, complete
report — the drawer shows covers, adr_corpus_digest, and decisions_scanned
with no mismatch line; (b) a stale report (covers differs from the current
spec revision) — the drawer visibly discloses the contrast; (c) a partial
sweep (a currently-declared decision id absent from decisions_scanned) —
the drawer visibly names the missing id. Each fixture must also show
finding disposition states rendered: dispositioned findings with their
note, undispositioned findings disclosed as undispositioned.

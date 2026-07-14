---
id: obligation/derivation-drawer--ac-2--behavioral
kind: obligation
title: "Same record, same drawer bytes"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/derivation-drawer" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Same record, same drawer bytes

The behavioral evidence must render the same fixture wall twice and assert
each badge's drawer body is byte-identical across renders, and must prove
drawer content corresponds field-for-field to the badge's embedded
derivation record (the record's inputs/revisions/records appear in the
drawer, and nothing appears in the drawer that is not in the record) — the
pure-function claim, falsifiable by any drawer line with no record source.

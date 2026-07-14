---
id: obligation/derivation-drawer--ac-4--static
kind: obligation
title: "No clock in any drawer render path"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/derivation-drawer" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# No clock in any drawer render path

The static evidence must show no wall-clock read (time.Now or equivalent)
in the drawer renderer, the judged-findings surface, or any code path that
builds drawer content — every revision cited traces to a derivation-record
field or a decoded report field (covers, adr_corpus_digest, content
digests), and the drawer markup contains no timestamp-formatting call.

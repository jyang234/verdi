---
id: obligation/badge-computes--ac-4--static
kind: obligation
title: "One canonical derivation record schema, revisions never wall-clock"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/badge-computes" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# One canonical derivation record schema, revisions never wall-clock

The static evidence must show dc-2's derivation record declared ONCE as a
shared type (source, label, target, inputs with per-input revision,
records, disclosures) and every badge constructor filling it — no badge
path that attaches without a record. It must show revision fields
populated from content digests or already-pinned fields (covers,
adr_corpus_digest, MR ids) and NO time.Now (or other wall-clock read) in
any badge compute or record constructor (dc-5, co-1).

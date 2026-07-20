---
id: obligation/finding-identity--ac-1--behavioral
kind: obligation
title: "a same-slug regenerated judged finding pre-fills as a candidate (old ruling + old text beside new text), AllDispositioned false until confirmed"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/finding-identity" }
frozen: { at: 2026-07-20, commit: fb6fb43180469f29545ca99f4d649930222b91a0 }
---
# a same-slug regenerated judged finding pre-fills as a candidate (old ruling + old text beside new text), AllDispositioned false until confirmed

The behavioral evidence must show a table-driven report-regeneration test
over the canned-judge fake (`internal/align/identity_test.go`'s
carry-forward matrix, extended): a first report is dispositioned on a
judged finding, then a second report is regenerated with the canned
judge emitting a finding at the identical rule/boundary-derived slug but
different (reworded) text. The test must assert the regenerated finding
renders as a candidate carrying both the old ruling and the old text
alongside the new text — not merely that some data is retained, but that
both texts and the old ruling are independently inspectable on the
candidate — and that `AllDispositioned` reports false for the report as
a whole until a human-equivalent confirmation step (a working-tree edit
at the covering head, mirroring X-16's existing fresh-finding discipline)
is applied to that specific candidate. A negative case must show a
freshly-dispositioned candidate does NOT silently flip
`AllDispositioned` true on its own. Green in CI's test step, as part of
`make verify`.

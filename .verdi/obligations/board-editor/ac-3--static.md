---
id: obligation/board-editor--ac-3--static
kind: obligation
title: "Unit tests prove every editor write path preserves untouched bytes over adversarial fixtures"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/board-editor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Unit tests prove every editor write path preserves untouched bytes over adversarial fixtures

The static evidence must show unit tests proving byte preservation on every
write path this story adds. For the save handler: the bytes written to the
artifact's body are the request's pane bytes verbatim (no trimming, no
newline normalization, no reindentation), proven by a round-trip assertion on
fixtures that include trailing whitespace, mixed indentation, comments
(%%), and blank lines. For the structural ops: over adversarial
renderer-legal fixtures, applying each op and diffing against the input shows
changes ONLY on the lines dc-2's grammar names for that op — every other
byte, including oddly formatted ones, is bit-identical (a property-style or
exhaustive-diff assertion, not a substring check). The tests must also
demonstrate the absence of any graph round-trip on the write path: no write
path may pass the source through a parse-then-serialize step (e.g. proven by
a fixture the renderer accepts but a graph round-trip would reorder or
reformat, surviving bit-for-bit). Fabricable evidence — e.g. asserting only
that the changed line appears — does not satisfy this obligation.

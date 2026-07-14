---
id: obligation/verification-extractor--ac-4--behavioral
kind: obligation
title: "A test proves a matching digest discloses no staleness and a mismatched one discloses stale-base"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/verification-extractor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# A test proves a matching digest discloses no staleness and a mismatched one discloses stale-base

The behavioral evidence must show a test that: (1) supplies a
`derived_from.digest` computed from the same canned truth graph the test
regenerates, asserting the stale-base check reports no staleness; (2)
supplies a deliberately different digest (a fixed wrong sha256 string),
asserting the check reports `stale-base`; and (3) demonstrates the two
tests above can each be run with AC-3's three-way comparison producing
either an empty or a non-empty residual, showing the two checks vary
independently (stale-base does not depend on, and is not conflated with,
the three-way comparison's own result).

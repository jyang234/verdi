---
id: obligation/disposition-verb--ac-4--static
kind: obligation
title: "the closure-ritual doc names verdi disposition as the only sanctioned way to record one"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/disposition-verb" }
frozen: { at: 2026-07-16, commit: 2911a957eda4f96d5ccfda5fc7ed1dfa388231d6 }
---
# the closure-ritual doc names verdi disposition as the only sanctioned way to record one

The static evidence must show a Go test — proposed home
`internal/specalign/docsync_test.go`, reusing `computeVerdiRoot()`
(`internal/specalign/helpers_test.go`) to locate the module root — that
reads `verdi/docs/architecture-and-journeys.md` and asserts:

1. The document's closure-ritual narrative (the "D — The build loop"
   step covering `verdi align`/dispositioning, and/or the "E — The
   closure ritual" section) contains the literal string
   `verdi disposition`.
2. The document contains no sentence describing or instructing a
   hand-edit of a deviation report's disposition fields as a sanctioned
   step (a substring/pattern check for language like "hand-edit" or
   "edit ... deviation-report.md ... by hand" used normatively, not
   merely in historical/retrospective mentions of round 6).

A test that only checks for the substring "disposition" (already present
today in unrelated sentences) without pinning it to the `verdi
disposition` verb form does not satisfy this obligation.

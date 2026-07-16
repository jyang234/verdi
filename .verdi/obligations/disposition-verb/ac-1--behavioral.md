---
id: obligation/disposition-verb--ac-1--behavioral
kind: obligation
title: "verdi disposition records a finding's decision and rationale in place while leaving the report's digest and integrity independently reverifiable"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/disposition-verb" }
frozen: { at: 2026-07-16, commit: 2911a957eda4f96d5ccfda5fc7ed1dfa388231d6 }
---
# verdi disposition records a finding's decision and rationale in place while leaving the report's digest and integrity independently reverifiable

The behavioral evidence must show a Go end-to-end test under
`cmd/verdi/disposition_test.go` that builds the real verdi binary
(reusing the `buildVerdiBinary(t)` helper already established in
`cmd/verdi/serve_integration_test.go`) and execs it against a fixture
store containing a living (non-frozen) `deviation-report.md` with at
least one undispositioned finding. Driving the built binary — never the
in-process functions directly — it must prove:

1. `verdi disposition spec/<name> <finding-id> fixed --rationale "<text>"`
   (and, separately, the `accepted-deviation` decision) exits 0, and the
   named finding's frontmatter entry decodes
   (`internal/artifact.DecodeDeviation`) to the given disposition and
   note.
2. That finding's rendered bullet line in the report's markdown body also
   shows the new decision and rationale.
3. Every OTHER finding's Disposition/Note is byte-identical to its
   pre-write value.
4. The report's `digest:`, `integrity:`, and `judge_integrity:` fields
   are byte-identical to their pre-write value, AND an independent
   recomputation of `align.ComputeDigest` over the same fixture tree at
   the same commit (mirroring `cmd/verdi/align_test.go`'s
   `buildAlignRepo` harness) still equals the stored digest.

A test that only checks the frontmatter's Disposition field — without
also checking the body's rendered line and without independently
recomputing the digest — does not satisfy this obligation.

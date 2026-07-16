---
id: obligation/disposition-verb--ac-3--behavioral
kind: obligation
title: "a disposition recorded by the verb survives verdi align --freeze byte-for-byte, including under a drifting judge"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/disposition-verb" }
frozen: { at: 2026-07-16, commit: 2911a957eda4f96d5ccfda5fc7ed1dfa388231d6 }
---
# a disposition recorded by the verb survives verdi align --freeze byte-for-byte, including under a drifting judge

The behavioral evidence must show a Go end-to-end test under
`cmd/verdi/disposition_test.go` (or `cmd/verdi/align_test.go`, alongside
`TestRunAlign_FreezePreservesDispositions`, if that proves the more
natural home once written) that:

1. Generates a living `deviation-report.md` via a real (non-freeze)
   `verdi align` run against a fixturegit repo with a real judge fake
   (mirroring `buildAlignRepo`/`alignFakeJudgeOK`).
2. Dispositions every finding using `verdi disposition` (the built
   binary) — never by direct struct or file mutation.
3. Runs `verdi align --freeze` a second time with a judge fake that would
   produce DIFFERENT content on a hypothetical regeneration (mirroring
   `alignFakeJudgeDrift`, the D6-24/T.1 drifting-judge harness).
4. Asserts `align.FreezeInPlace` fired: the frozen report's findings,
   dispositions, and notes are byte-identical to what the disposition
   verb wrote in step 2, and the drifting judge's content is nowhere in
   the frozen output.

A test that dispositions findings by direct struct/file mutation instead
of the built verb, or that freezes without first proving the judge
actually drifted, does not satisfy this obligation.

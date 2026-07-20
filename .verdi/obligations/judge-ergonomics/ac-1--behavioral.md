---
id: obligation/judge-ergonomics--ac-1--behavioral
kind: obligation
title: "align prints its report path as stdout line 1 before the judge runs, and the report is never observable mid-write"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/judge-ergonomics" }
frozen: { at: 2026-07-20, commit: 08be7d012f0d438fd428a10a8c59ca76f1fda346 }
---
# align prints its report path as stdout line 1 before the judge runs, and the report is never observable mid-write

The behavioral evidence must show a built-binary test (mirroring
`cmd/verdi/align_test.go`'s existing style) that runs `verdi align`
against a canned judge and asserts the report path is the very first line
printed to stdout, and that this line appears before the judge subprocess
has produced any output of its own — a caller must never have to wait for
the judge exchange to learn where the report will land. It must also
show, via a concurrent reader polling the printed path while the judge
runs (a slow canned judge fixture gives the window), that the path is
never observed holding partial or truncated content: every read either
finds no file yet or finds the complete, final report, proving the
existing `internal/atomicfile` write seam is what `align`'s report write
now goes through. The assertion must hold against `align.go`'s own
`loadExistingReport` (`align.go:329`) as the reader — the same function
any resuming caller depends on — not a bespoke test-only reader that
could diverge from it. Green in CI's test step, as part of `make verify`.

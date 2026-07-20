---
id: obligation/judge-ergonomics--ac-3--behavioral
kind: obligation
title: "close's freeze-align inherits the first-line-path, atomic-write, and --wait contract from the shared align engine"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/judge-ergonomics" }
frozen: { at: 2026-07-20, commit: 08be7d012f0d438fd428a10a8c59ca76f1fda346 }
---
# close's freeze-align inherits the first-line-path, atomic-write, and --wait contract from the shared align engine

The behavioral evidence must show a test that drives `verdi close`'s
internal freeze-align path directly — against a canned judge, not the
real LLM — and proves the identical three guarantees ac-1/ac-2 pin for
`align` itself: the report path prints first, the report is written
atomically (never observable mid-write), and a bounded `--wait[=seconds]`
on the close-triggered freeze-align blocks then exits 0 on completion or
exits 2 with the path on expiry. The proof must be constructed so that it
can only pass if freeze-align calls through the *same* shared align-engine
hook `align` itself calls — for example, by asserting on a shared
engine-level fixture or instrumentation point common to both call sites,
not by independently re-implementing the same three assertions against
freeze-align's surface in a way that would still pass even if freeze-align
carried its own divergent, second implementation. Green in CI's test
step, as part of `make verify`.

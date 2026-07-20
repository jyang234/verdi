---
id: obligation/judge-ergonomics--ac-2--behavioral
kind: obligation
title: "align's --wait blocks bounded then exits 0 on completion, or exits 2 with the report path on expiry"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/judge-ergonomics" }
frozen: { at: 2026-07-20, commit: 08be7d012f0d438fd428a10a8c59ca76f1fda346 }
---
# align's --wait blocks bounded then exits 0 on completion, or exits 2 with the report path on expiry

The behavioral evidence must show built-binary tests (`cmd/verdi/
align_test.go`) and canned-judge unit tests (`internal/align/
judged_test.go`) proving both halves of the `--wait[=seconds]` contract.
First: against a canned judge that completes quickly, `verdi align
--wait=<bound>` blocks internally until the report is ready, then exits
0, with the finished report already on disk at the printed path — no
caller-side polling loop is needed to observe completion. Second: against
a canned judge fixture engineered to hang past the bound (mirroring the
existing hung-fake test pattern), `verdi align --wait=1` exits 2 — never
0, never 1, since this is an operational timeout rather than a verdict —
and the report path is still the first line already printed, so the exit
is never a silent hang and the caller always has a location to resume
watching. A default bound must also be exercised when `--wait` carries no
explicit value, proving the "sane default" the story's outcome promises
is a real, tested number, not merely documented. Green in CI's test step,
as part of `make verify`.

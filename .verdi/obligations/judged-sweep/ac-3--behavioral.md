---
id: obligation/judged-sweep--ac-3--behavioral
kind: obligation
title: "A round-trip test recomputes integrity from the persisted judge_integrity fields and confirms it matches"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/judged-sweep" }
frozen: { at: 2026-07-14, commit: 1d78c2776983c7c08ae4a065727a828b2fd28825 }
---
# A round-trip test recomputes integrity from the persisted judge_integrity fields and confirms it matches

The behavioral evidence must show a test that runs a sweep against a fake
judge, decodes the resulting sweep-report.md, base64-decodes its
`judge_integrity.stdin_b64` and reads its `raw_result`, recomputes
`computeIntegrity(stdin, rawResult)` independently, and asserts the result
equals the report's own persisted `integrity` field byte-for-byte — the
same self-verification round trip this codebase's decision-conflict tests
already perform for `DecisionJudgedResult`.

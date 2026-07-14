---
id: obligation/fail-loud--ac-2--behavioral
kind: obligation
title: "Honest failure paths witnessed at the verb level"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/fail-loud" }
frozen: { at: 2026-07-13, commit: 7f3c08d367dd4f76b45e982dc03813875e0e7a7c }
---
# Honest failure paths witnessed at the verb level

The behavioral evidence must show each repair firing: an unreadable active
spec directory surfaces as an operational exit 2 from the build-start verb
(not a clean no-supersession pass), skipped honestly under root; a
%w-wrapped ErrBoardNotFound still takes the 404 path; `--verdict fail`
emission exits 0 with verdict: fail present in the written runtime.json
record — the pin, not a behavior change.

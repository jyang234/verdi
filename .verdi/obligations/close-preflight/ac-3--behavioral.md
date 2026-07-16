---
id: obligation/close-preflight--ac-3--behavioral
kind: obligation
title: "A Go test proves each defect-class fixture agrees with itself: --preflight and a real close on the same store give the same reason"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/close-preflight" }
frozen: { at: 2026-07-16, commit: 20b0525430727bbeb168bb1a0cb5d0593f40a70d }
---
# A Go test proves each defect-class fixture agrees with itself: --preflight and a real close on the same store give the same reason

The behavioral evidence must show, for each defect class named in ac-1
(unmet AC per evidence kind, spec-stale, pending-supersession, unreconciled
stub, an open implementing story), ONE Go test that runs BOTH halves
against the byte-identical fixture in the same test body: first
`--preflight` (asserting its disclosure), then a real, unmodified
`verdi close`/`runClose` invocation on that same, unmutated fixture
(asserting its refusal reason matches the disclosure, not merely that it
also fails) — never two separately-asserted expectations that could
silently diverge.

A further test builds a fixture with every condition satisfied, asserts
`--preflight` reports ready (exit 0, no unmet conditions printed), then
runs a real, unmodified `verdi close` on that same fixture and asserts it
succeeds (exit 0) and the quartet is actually archived
(`specs/archive/<name>/` populated, `specs/active/<name>/` gone).

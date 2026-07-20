---
id: obligation/finding-identity--ac-4--behavioral
kind: obligation
title: "the feature-close budget unions implementing stories' archived reports with the feature's own; a same-report slug collision discloses, never dedupes"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/finding-identity" }
frozen: { at: 2026-07-20, commit: fb6fb43180469f29545ca99f4d649930222b91a0 }
---
# the feature-close budget unions implementing stories' archived reports with the feature's own; a same-report slug collision discloses, never dedupes

The behavioral evidence must show a feature-close budget test
constructed against the true X-18 shape: an accepted deviation recorded
in an implementing story's *archived* report, with the feature's own
fresh report NOT independently reproducing that same finding. The test
must assert the union counts that accepted deviation exactly once — a
negative case proves it is not silently dropped (zero), and a second
negative case (the story's report AND the feature's own report both
carrying it) proves it is not double-counted (two). This must be
constructed so it would fail against the Task-0 design wave's refuted
"unique identities within one report" framing — i.e. the test's fixture
data must span two separate reports, not rely on a single report's own
internal deduplication, or it proves nothing about the cross-report fix.
Separately, a same-report collision case: the canned judge is
constructed to emit two distinct findings sharing one slug within a
single report, and the test asserts this surfaces as its own disclosed
judge-contract-violation finding — never a silent dedupe that would hide
one of the two from a human disposing the report. Green in CI's test
step, as part of `make verify`.

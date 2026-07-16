---
id: obligation/disposition-verb--ac-2--behavioral
kind: obligation
title: "verdi disposition refuses each unsafe request as a named verdict, and only those, distinctly from operational failure"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/disposition-verb" }
frozen: { at: 2026-07-16, commit: 2911a957eda4f96d5ccfda5fc7ed1dfa388231d6 }
---
# verdi disposition refuses each unsafe request as a named verdict, and only those, distinctly from operational failure

The behavioral evidence must show table-driven Go end-to-end tests under
`cmd/verdi/disposition_test.go`, driving the built binary
(`buildVerdiBinary(t)`) against fixture stores, proving:

1. A `<finding-id>` not present in the deviation-report.md's `findings:`
   exits 1 and names the unknown id in stderr, writing nothing.
2. Re-running the verb against a finding that already carries a
   disposition, without `--amend`, exits 1 and names the
   already-dispositioned finding, writing nothing.
3. The same re-run WITH `--amend` exits 0 and replaces the finding's
   prior decision and rationale.
4. `--amend` against a finding with no existing disposition exits 1
   (nothing to amend), writing nothing.
5. Any invocation against a spec-ref whose deviation-report.md already
   carries a `frozen:` stamp exits 1 and names the report frozen,
   writing nothing — including with `--amend`.
6. A spec-ref with no deviation-report.md at all, and one that fails
   strict decode, each exit 2.

Every refusal case (1, 2, 4, 5, 6) must also assert the target file's
bytes are wholly unchanged — no partial write — and case 3 (the deliberate
amend) must assert only the targeted finding's entry and rendered body
line changed. A test that only checks the exit code without asserting
whether a write occurred does not satisfy this obligation.

---
id: obligation/attest-helper--ac-2--static
kind: obligation
title: "Table-driven unit tests cover both refusal predicates: the (story, AC) pair-existence check and the already-exists check"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-16, commit: a42d24e2ed017bcf7fa839417755b98b90bb0f34 }
---
# Table-driven unit tests cover both refusal predicates: the (story, AC) pair-existence check and the already-exists check

The static evidence must show table-driven unit tests (in
`cmd/verdi/attest_test.go`'s package, alongside the behavioral cases) over
the two refusal predicates AC-2 names, exercised directly: the
pair-existence check across its three failure shapes — a `<story-ref>` that
does not resolve via the shared two-form contract
(`internal/storyresolve.Resolve`, I-30), a ref that resolves to a spec whose
`class` is not `story` (dc-5's scope boundary), and a resolved story that
does not declare `<ac-id>` — and the already-exists check at the exact fold
path. Each row must show the predicate classifying the case as the AC-2
verdict outcome (the caller's exit 1), never as an operational error, so the
0/1/2 split (dc-5) is proven at the predicate level, not only end to end.

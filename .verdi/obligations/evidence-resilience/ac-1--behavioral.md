---
id: obligation/evidence-resilience--ac-1--behavioral
kind: obligation
title: "sync quarantines a record referencing a commit unreachable from HEAD, keeping it with the reason annotated, and exits 0"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/evidence-resilience" }
frozen: { at: 2026-07-20, commit: 8cd7e52f3d9c69014d6a020a55bb4284e67eef55 }
---
# sync quarantines a record referencing a commit unreachable from HEAD, keeping it with the reason annotated, and exits 0

The behavioral evidence must show a `fixturegit` test (beside
`cmd/verdi/sync_ancestor_test.go`'s existing home) constructing a
repository whose evidence bundle references a commit that lived only on
a branch since deleted — no ref or reachable history retains it. The
test must assert three things after running `sync`: the record is still
present in the synced output (not silently dropped), the record carries
a machine-checkable quarantine annotation naming the reason (commit
unreachable from `HEAD`), and `sync` itself exits 0 — one
unreachable-commit record must not, by itself, cause `sync` to report an
operational failure. A companion case with an ordinary, reachable-commit
record must show that record synced normally, unquarantined, proving the
quarantine path is additive and does not touch the common case. Green in
CI's test step, as part of `make verify`.

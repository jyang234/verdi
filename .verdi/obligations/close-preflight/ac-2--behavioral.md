---
id: obligation/close-preflight--ac-2--behavioral
kind: obligation
title: "A Go test proves the exit-code matrix and non-mutation of --preflight in every mode"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/close-preflight" }
frozen: { at: 2026-07-16, commit: 20b0525430727bbeb168bb1a0cb5d0593f40a70d }
---
# A Go test proves the exit-code matrix and non-mutation of --preflight in every mode

The behavioral evidence must show a Go test (in
`cmd/verdi/closepreflight_test.go`) that drives `--preflight` over three
fixtures — one ready, one with an unmet condition, one that forces a
genuine operational error (e.g. an unreadable/malformed derived record, or
the configured-but-erroring forge case, dc-5) — and asserts exit 0 / 1 / 2
respectively. For every one of the three, the test snapshots the working
tree (`git status --porcelain`, or an equivalent file/ref-listing diff)
before and after the run and asserts it is byte-identical: no branch
created, no file written, no commit made, no ref moved.

A further test proves `--preflight` succeeds (reaches its own verdict,
exit 0 or 1) with no CI environment variables set and without
`--force-local`, and that its stdout/stderr never contains close's
`--force-local` escape-hatch warning text — proving `--preflight` is
dispatched before that guard (dc-1), not merely exempted from failing it.

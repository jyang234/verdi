---
id: obligation/sync-local-flow--ac-2--static
kind: obligation
title: "Table-driven tests prove the candidate-ancestor helper starts at the commit itself, ordered via gitx's own Log"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/sync-local-flow" }
frozen: { at: 2026-07-16, commit: 8e97d547d237e007d584e977f4eafdb73d69d59a }
---
# Table-driven tests prove the candidate-ancestor helper starts at the commit itself, ordered via gitx's own Log

The static evidence must show table-driven unit tests over the new
candidate-ancestor enumeration/ordering helper, proving two properties in
isolation from any forge call: the commit under evaluation is always the
first candidate in the returned order (since a commit is its own ancestor
under `gitx.IsAncestor`'s documented self-inclusive semantics, dc-1), and
the remaining candidates are otherwise deterministically ordered.

Per dc-1, the tests must show the helper is built directly over
`internal/gitx/log.go`'s existing `Log` primitive (already most-recent-
first over a single rev with no path filter) rather than a hand-rolled
parent walk — a test that only checks the final candidate set, without
also checking the order matches what `gitx.Log` itself produces for the
same rev, does not rule out a second, possibly-disagreeing notion of
"ancestor" and so does not satisfy this obligation. No depth bound may
appear anywhere in the helper or its tests: a test asserting the walk
stops short of the ref's root, or capping the candidate list at some fixed
length, is itself a defect against dc-1's "no depth limit," not a passing
case.

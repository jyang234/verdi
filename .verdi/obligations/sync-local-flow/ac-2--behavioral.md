---
id: obligation/sync-local-flow--ac-2--behavioral
kind: obligation
title: "Integration tests over four ancestor topologies each prove which commit's bundle was accepted and how far the walk went"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/sync-local-flow" }
frozen: { at: 2026-07-16, commit: 8e97d547d237e007d584e977f4eafdb73d69d59a }
---
# Integration tests over four ancestor topologies each prove which commit's bundle was accepted and how far the walk went

The behavioral evidence must show a new `cmd/verdi/sync_ancestor_test.go`
(hermetic — fixturegit topologies plus `internal/forge/fake` seeded per
candidate commit) covering four fixture classes: a linear history with the
bundle present only at a named ancestor several commits back from HEAD; a
branched history; a bundle present at HEAD itself, asserting it wins with
no walk performed; and no bundle present anywhere on the walked path,
asserting the existing refusal, re-worded to name the ref and the commit
range actually walked.

For every one of the four, the test must assert BOTH which commit's
bundle sync accepted (or, for the refusal case, the range it exhausted)
AND the disclosed distance walked — a test that only asserts sync
succeeded or failed, without also asserting the specific commit identity
and distance sync discloses, does not satisfy this obligation, since
D6-32's asymmetry is closed only if the accepted bundle's provenance is
legible, not merely present.

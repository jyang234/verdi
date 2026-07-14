---
id: obligation/badge-computes--ac-4--behavioral
kind: obligation
title: "Derivation records are complete and deterministic"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/badge-computes" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Derivation records are complete and deterministic

The behavioral evidence must render a fixture wall with firing badges and
assert each badge's serialized derivation record names (a) its namespaced
source rule id, (b) at least the pinned inputs that compute actually read,
each with a non-empty revision, and (c) one entry per firing record — and
must render the SAME fixture twice and assert the two badge payloads are
byte-identical (no wall-clock, no map-order nondeterminism).

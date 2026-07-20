---
id: obligation/evidence-resilience--ac-2--behavioral
kind: obligation
title: "closure reads a quarantined record as a per-record disclosed-unproven, never operational exit 2 — the X-15 regression pinned negative"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/evidence-resilience" }
frozen: { at: 2026-07-20, commit: 8cd7e52f3d9c69014d6a020a55bb4284e67eef55 }
---
# closure reads a quarantined record as a per-record disclosed-unproven, never operational exit 2 — the X-15 regression pinned negative

The behavioral evidence must show a `fixturegit` closure test
reproducing X-15's exact shape: a story being closed whose synced
evidence bundle includes a record quarantined per ac-1 (referencing a
commit an unrelated, since-deleted branch produced). The test must
assert the closure run does NOT exit operationally (never exit 2, never
the literal "Not a valid commit name" text) on account of that record
alone, and that the acceptance criterion the quarantined record would
have evidenced is reported as a per-record disclosed-unproven rather than
silently marked proven. A companion case must show the gate's overall
verdict stays honest when a DIFFERENT, non-quarantined record
legitimately proves the same AC — the quarantine must degrade only the
one affected record, never mask a real proof sitting alongside it. This
is pinned as a negative regression test: it must fail against the
pre-fix behavior (bricked closure) to prove it actually exercises X-15's
shape. Green in CI's test step, as part of `make verify`.

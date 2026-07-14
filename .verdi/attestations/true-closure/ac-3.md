---
id: attestation/true-closure--ac-3
kind: attestation
title: "AC-3 attested: every evidence kind a spec can declare — runtime included — has a producing mechanism queryable by (story, AC) at close time"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/true-closure }
frozen: { at: 2026-07-13, commit: 6185f58a6d34ca38059c317576b1da4c5c87e3fe }
---
# AC-3 outcome attestation

Operator attests (round 6, 2026-07-13): every evidence kind a spec can declare
now has a real producing mechanism, queryable by (story, AC) at close time.
**Static** and **behavioral** — the `make verify`-derived `verdi-verify-static`
/ `verdi-verify-behavioral` producers (selfevidence.go), bound by (story, AC)
through `verdi.bindings.yaml`, stamped `source: ci` only in genuine CI (D6-10);
every closed story folded on these. **Attestation** — operator outcome
attestations under `.verdi/attestations/<slug>/<ac>.md`, resolved by (story, AC)
via `AttestationExists` (this record is itself one). **Runtime** — the story
`spec/runtime-evidence` closed the last gap (OQ-2): `internal/runtime`'s `Emit`
builds a `kind: runtime` record and `Query` returns it by (story, AC) via a
deterministic `CheckID`; `verdi sync --produce-runtime` is the producer
entrypoint, and `verdi close` folds a runtime record exactly as it folds
static/behavioral (proven end to end, runtime as the sole evidence kind, in
`close_runtime_test.go`). No declarable kind is left a decoder-only placeholder.

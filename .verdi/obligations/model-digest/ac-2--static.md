---
id: obligation/model-digest--ac-2--static
kind: obligation
title: "Source-witness evidence proves all four production Provenance mints set Model only via StampProvenance, with no bypass and no undiscovered fifth site"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/model-digest" }
frozen: { at: 2026-07-17, commit: b7a0cf9801ea852fcc1f4801da11ee115f6ffc41 }
---
# Source-witness evidence proves all four production Provenance mints set Model only via StampProvenance, with no bypass and no undiscovered fifth site

The static evidence must show, mirroring `spec/shared-homes` ac-1's own
"one seam, no surviving copies" convention
(`.verdi/obligations/shared-homes/ac-1--static.md`), that all four
production `artifact.Provenance{...}` construction sites —
`internal/commitdesign/commitdesign.go:254`,
`internal/align/report.go:117`, `internal/align/decision_report.go:151`,
`internal/align/diagram_report.go:137` — set their `Model` field only
through a call to `artifact.StampProvenance`, never inline in the struct
literal the way `Digest:`/`Integrity:` are set today. It must show a
source check (grep-based or AST-based, committed as a test) proving no
file outside `internal/artifact/stamp.go` itself assigns to the `.Model`
field of a `Provenance` value, so a future fifth mint site that bypasses
the seam is caught by the same check rather than requiring hand
rediscovery of the enumeration. It must also record, in the check itself
or its accompanying comment, why `cmd/verdi/attest.go` is correctly
excluded from the four-site enumeration: it mints only a `Frozen` stamp
for an `AttestationScaffold`, never a `Provenance` — so the count the
check enforces stays four, not five. Verified by `make verify`'s
build/vet/lint steps plus this dedicated source-witness test, never by
manual inspection alone.

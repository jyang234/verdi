---
schema: verdi.deviation/v1
covers: 81ec5364d0ffd9489ecf94b8c754960048fc556c
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer reviewed manually; no decision conflicts with another decision or an ADR. ac-1: internal/evidence.Obligations loads an AC's obligations by (spec-name, ac-id), missing=absent / broken=surfaced-error, and verdi matrix gains an additive OBLIGATION column (fold untouched, oq-1 honored). ac-2: the board story-AC card renders an obligations footer via the SAME loader (one reader, dc-1) — authored kind shows the obligation title (prose in the tooltip, satisfying co-3), un-obligated kind shows a disclosed 'no obligation' badge (dc-2, wall-receipts posture). Two disclosed design choices, both accepted: obligations attached post-buildProjection in loadBoard (keeps the pure projector a function of its inputs, mirroring proj.Notices); the demand TITLE is the visible payload with prose in the tooltip+DOM (reversible CSS if full prose must always show). Process note: the backend agent left its work uncommitted; the reviewer committed it (28628a6). make verify green (130 e2e). Judged coverage accepted as absent." }
digest: sha256:1549251fa5b188cd06cf4ecc790d5809a6250aeeedea413c4e28c0b18a49639c
provenance: { generator: verdi-align, version: v0, inputs: [spec/obligation-wall@81ec5364d0ffd9489ecf94b8c754960048fc556c, spec/obligation-wall@54b01d9bedf2cc4389b46d8b09cbc5077b19c53b], digest: sha256:1549251fa5b188cd06cf4ecc790d5809a6250aeeedea413c4e28c0b18a49639c }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [UNDISPOSITIONED]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json")

# testdata/corpus-invalid

Decode-failure twins (PLAN.md phase 2 deliverable 4): each file is a small
variant of a real `examples/showcase/` fixture with exactly one injected
defect, proven in `internal/corpus/invalid_test.go` to fail loudly with an
error naming the offense.

| File | Base fixture | Defect | Expected failure |
|---|---|---|---|
| `spec-unknown-field.md` | `.verdi/specs/active/stale-decline/spec.md` | unknown top-level field `bogus_extra_field` | KnownFields(true) rejection naming the field |
| `adr-unknown-field.md` | `.verdi/adr/0002-outbox-events.md` | unknown top-level field `severity` | KnownFields(true) rejection naming the field |
| `board-unknown-field.json` | archived `board.json` | unknown field `extra_untracked_field` | `encoding/json` `DisallowUnknownFields` rejection |
| `evidence-unknown-field.json` | canned evidence record | unknown field `confidence` | `encoding/json` `DisallowUnknownFields` rejection |
| `spec-anchor.md` | `stale-decline/spec.md` | YAML anchor (`&team`) on `owners:` | dialect rejection naming "anchor" |
| `spec-alias.md` | `stale-decline/spec.md` | YAML anchor + alias (`&owner` / `*owner`) | dialect rejection naming "anchor" (checkDialect trips on the anchor before reaching the alias node) |
| `spec-custom-tag.md` | `stale-decline/spec.md` | custom YAML tag (`!urgent`) on `story:` | dialect rejection naming "custom tag" |

These are separate from `testdata/violations/`, which holds lint-rule
(VL-001..014) overlays for the phase-4 lint engine — some of those
overlays also happen to fail phase-2 decode (defense in depth), but their
purpose is to trip one specific lint rule, not to exercise the decode
seam's failure modes directly.
